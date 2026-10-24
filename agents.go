package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// agentArgv builds the headless command line for one coding CLI.
// Permission "full" lets the agent run tools without asking (it cannot ask: there is no TTY);
// "edits" limits it to file edits inside its working directory where the CLI supports that.
func agentArgv(cfg Config, agent, model, prompt, cwd string) ([]string, error) {
	a, ok := cfg.Agents[agent]
	if !ok || !contains(agentNames, agent) {
		return nil, fmt.Errorf("unknown agent %q", agent)
	}
	if model == "" {
		model = a.Model
	}
	full := cfg.PermissionMode == "full"
	switch agent {
	case "claude":
		argv := []string{a.Bin, "-p", prompt, "--output-format", "stream-json", "--verbose"}
		if full {
			argv = append(argv, "--permission-mode", "bypassPermissions")
		} else {
			argv = append(argv, "--permission-mode", "acceptEdits")
		}
		if model != "" {
			argv = append(argv, "--model", model)
		}
		return argv, nil
	case "codex":
		argv := []string{a.Bin, "exec", "--json", "--skip-git-repo-check", "-C", cwd}
		if full {
			argv = append(argv, "--dangerously-bypass-approvals-and-sandbox")
		} else {
			argv = append(argv, "--sandbox", "workspace-write")
		}
		if model != "" {
			argv = append(argv, "-m", model)
		}
		return append(argv, prompt), nil
	case "omp":
		if !full {
			return nil, errors.New(tr("omp không có chế độ chỉ sửa file; chọn claude/codex hoặc bật Toàn quyền trong Cài đặt", "omp has no edit-only mode; pick claude/codex or turn on Full access in Settings"))
		}
		argv := []string{a.Bin, "-p", "--mode", "json", "--cwd", cwd}
		if model != "" {
			argv = append(argv, "--model", model)
		}
		return append(argv, prompt), nil
	}
	return nil, fmt.Errorf("unknown agent %q", agent)
}

// AgentInfo reports whether a CLI is installed and its version.
type AgentInfo struct {
	Name      string `json:"name"`
	Bin       string `json:"bin"`
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
	Model     string `json:"model,omitempty"`
}

var (
	agentInfoMu    sync.Mutex
	agentInfoCache = map[string]AgentInfo{}
	agentInfoAt    time.Time
)

func agentInfos(cfg Config) []AgentInfo {
	agentInfoMu.Lock()
	defer agentInfoMu.Unlock()
	fresh := time.Since(agentInfoAt) < 5*time.Minute
	out := []AgentInfo{}
	for _, n := range agentNames {
		a := cfg.Agents[n]
		if c, ok := agentInfoCache[n]; ok && fresh && c.Bin == a.Bin {
			c.Model = a.Model
			out = append(out, c)
			continue
		}
		info := AgentInfo{Name: n, Bin: a.Bin, Model: a.Model}
		if path, err := exec.LookPath(a.Bin); err == nil {
			info.Available = true
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			b, _ := exec.CommandContext(ctx, path, "--version").Output()
			cancel()
			info.Version = strings.TrimSpace(strings.SplitN(string(b), "\n", 2)[0])
		}
		agentInfoCache[n] = info
		out = append(out, info)
	}
	agentInfoAt = time.Now()
	return out
}
