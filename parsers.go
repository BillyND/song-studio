package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Event is the normalized unit the UI renders, whatever CLI produced it.
type Event struct {
	Seq   int    `json:"seq"`
	TS    int64  `json:"ts"`
	Kind  string `json:"kind"` // system, text, thinking, tool_call, tool_result, log, stderr, result, error, status
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
	Tool  string `json:"tool,omitempty"`
	Ref   string `json:"ref,omitempty"` // links a tool_result to its tool_call
	Error bool   `json:"error,omitempty"`
}

// Usage is what a parser learns about cost while streaming.
type Usage struct {
	CostUSD      float64 `json:"costUsd,omitempty"`
	InputTokens  int     `json:"inputTokens,omitempty"`
	OutputTokens int     `json:"outputTokens,omitempty"`
	Session      string  `json:"session,omitempty"`
	Model        string  `json:"model,omitempty"`
}

// Parser turns one stdout line into zero or more events.
type Parser interface {
	Line(line string, emit func(Event), usage *Usage)
}

func newParser(kind, agent string) Parser {
	if kind != "agent" {
		return scriptParser{}
	}
	switch agent {
	case "claude":
		return &claudeParser{}
	case "codex":
		return &codexParser{}
	case "omp":
		return &ompParser{}
	}
	return scriptParser{}
}

const maxBody = 6000

func clip(s string) string {
	s = strings.TrimRight(s, "\n")
	if len(s) <= maxBody {
		return s
	}
	cut := maxBody
	for cut > 0 && (s[cut]&0xC0) == 0x80 { // do not split a UTF-8 sequence
		cut--
	}
	return s[:cut] + fmt.Sprintf("\n… (%d bytes more)", len(s)-cut)
}

// summarizeInput picks the field a person wants to see for a tool call.
func summarizeInput(raw json.RawMessage) string {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || len(m) == 0 {
		return clip(string(raw))
	}
	for _, k := range []string{"command", "cmd", "file_path", "path", "pattern", "url", "query", "description", "prompt", "skill"} {
		if v, ok := m[k]; ok {
			s := fmt.Sprint(v)
			if k == "file_path" || k == "path" {
				if old, ok := m["old_string"]; ok && old != "" {
					return s + tr("  (sửa nội dung)", "  (edit)")
				}
			}
			return clip(s)
		}
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := []string{}
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %v", k, m[k]))
	}
	return clip(strings.Join(parts, "\n"))
}

func contentText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) == nil {
		out := []string{}
		for _, p := range parts {
			if p.Text != "" {
				out = append(out, p.Text)
			} else if p.Type != "" && p.Type != "text" {
				out = append(out, "["+p.Type+"]")
			}
		}
		return strings.Join(out, "\n")
	}
	return string(raw)
}

// ---------- scripts ----------

type scriptParser struct{}

func (scriptParser) Line(line string, emit func(Event), _ *Usage) {
	if strings.TrimSpace(line) == "" {
		return
	}
	emit(Event{Kind: "log", Body: clip(line)})
}

// ---------- Claude Code: --output-format stream-json --verbose ----------

type claudeParser struct{}

func (p *claudeParser) Line(line string, emit func(Event), u *Usage) {
	var ev struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
		Model   string `json:"model"`
		Cwd     string `json:"cwd"`
		Session string `json:"session_id"`
		Message struct {
			Model   string `json:"model"`
			Content []struct {
				Type      string          `json:"type"`
				Text      string          `json:"text"`
				Thinking  string          `json:"thinking"`
				ID        string          `json:"id"`
				Name      string          `json:"name"`
				Input     json.RawMessage `json:"input"`
				ToolUseID string          `json:"tool_use_id"`
				Content   json.RawMessage `json:"content"`
				IsError   bool            `json:"is_error"`
			} `json:"content"`
		} `json:"message"`
		Result   string  `json:"result"`
		IsError  bool    `json:"is_error"`
		CostUSD  float64 `json:"total_cost_usd"`
		NumTurns int     `json:"num_turns"`
		Duration int64   `json:"duration_ms"`
		ParentID *string `json:"parent_tool_use_id"`
		Usage    struct {
			In  int `json:"input_tokens"`
			Out int `json:"output_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal([]byte(line), &ev) != nil {
		if strings.TrimSpace(line) != "" {
			emit(Event{Kind: "log", Body: clip(line)})
		}
		return
	}
	sub := ev.ParentID != nil && *ev.ParentID != ""
	switch ev.Type {
	case "system":
		if ev.Subtype == "init" {
			u.Session, u.Model = ev.Session, ev.Model
			emit(Event{Kind: "system", Title: tr("Phiên Claude bắt đầu", "Claude session started"), Body: strings.TrimSpace("model " + ev.Model + "\ncwd " + ev.Cwd)})
		}
	case "assistant":
		if ev.Message.Model != "" {
			u.Model = ev.Message.Model
		}
		for _, c := range ev.Message.Content {
			switch c.Type {
			case "text":
				if strings.TrimSpace(c.Text) != "" {
					emit(Event{Kind: "text", Body: clip(c.Text), Title: subTitle(sub)})
				}
			case "thinking":
				if strings.TrimSpace(c.Thinking) != "" {
					emit(Event{Kind: "thinking", Body: clip(c.Thinking)})
				}
			case "tool_use":
				emit(Event{Kind: "tool_call", Tool: c.Name, Ref: c.ID, Body: summarizeInput(c.Input), Title: subTitle(sub)})
			}
		}
	case "user":
		for _, c := range ev.Message.Content {
			if c.Type == "tool_result" {
				emit(Event{Kind: "tool_result", Ref: c.ToolUseID, Body: clip(contentText(c.Content)), Error: c.IsError})
			}
		}
	case "result":
		u.CostUSD = ev.CostUSD
		u.InputTokens, u.OutputTokens = ev.Usage.In, ev.Usage.Out
		title := fmt.Sprintf(tr("Xong sau %d lượt · %.1fs", "Done after %d turns · %.1fs"), ev.NumTurns, float64(ev.Duration)/1000)
		emit(Event{Kind: "result", Title: title, Body: clip(ev.Result), Error: ev.IsError || ev.Subtype != "success"})
	}
}

func subTitle(sub bool) string {
	if sub {
		return tr("agent phụ", "subagent")
	}
	return ""
}

// ---------- Codex: codex exec --json ----------

type codexParser struct{}

func (p *codexParser) Line(line string, emit func(Event), u *Usage) {
	var ev struct {
		Type     string `json:"type"`
		ThreadID string `json:"thread_id"`
		Message  string `json:"message"`
		Error    struct {
			Message string `json:"message"`
		} `json:"error"`
		Item struct {
			ID       string `json:"id"`
			Type     string `json:"type"`
			Text     string `json:"text"`
			Command  string `json:"command"`
			Output   string `json:"aggregated_output"`
			ExitCode *int   `json:"exit_code"`
			Status   string `json:"status"`
			Server   string `json:"server"`
			Tool     string `json:"tool"`
			Query    string `json:"query"`
			Changes  []struct {
				Path string `json:"path"`
				Kind string `json:"kind"`
			} `json:"changes"`
			Items []struct {
				Text      string `json:"text"`
				Completed bool   `json:"completed"`
			} `json:"items"`
		} `json:"item"`
		Usage struct {
			In  int `json:"input_tokens"`
			Out int `json:"output_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal([]byte(line), &ev) != nil {
		if strings.TrimSpace(line) != "" {
			emit(Event{Kind: "log", Body: clip(line)})
		}
		return
	}
	it := ev.Item
	switch ev.Type {
	case "thread.started":
		u.Session = ev.ThreadID
		emit(Event{Kind: "system", Title: tr("Phiên Codex bắt đầu", "Codex session started"), Body: "thread " + ev.ThreadID})
	case "item.started":
		if it.Type == "command_execution" {
			emit(Event{Kind: "tool_call", Tool: "shell", Ref: it.ID, Body: clip(it.Command)})
		}
	case "item.completed":
		switch it.Type {
		case "agent_message":
			emit(Event{Kind: "text", Body: clip(it.Text)})
		case "reasoning":
			emit(Event{Kind: "thinking", Body: clip(it.Text)})
		case "command_execution":
			failed := it.ExitCode != nil && *it.ExitCode != 0
			emit(Event{Kind: "tool_result", Ref: it.ID, Body: clip(it.Output), Error: failed || it.Status == "failed"})
		case "file_change":
			paths := []string{}
			for _, c := range it.Changes {
				paths = append(paths, c.Kind+" "+c.Path)
			}
			emit(Event{Kind: "tool_call", Tool: "edit", Ref: it.ID, Body: strings.Join(paths, "\n")})
		case "mcp_tool_call":
			emit(Event{Kind: "tool_call", Tool: it.Server + "." + it.Tool, Ref: it.ID})
		case "web_search":
			emit(Event{Kind: "tool_call", Tool: "web_search", Ref: it.ID, Body: it.Query})
		case "todo_list":
			lines := []string{}
			for _, t := range it.Items {
				mark := "☐"
				if t.Completed {
					mark = "☑"
				}
				lines = append(lines, mark+" "+t.Text)
			}
			emit(Event{Kind: "system", Title: tr("Kế hoạch", "Plan"), Body: strings.Join(lines, "\n")})
		case "error":
			emit(Event{Kind: "error", Body: clip(it.Text)})
		}
	case "turn.completed":
		u.InputTokens += ev.Usage.In
		u.OutputTokens += ev.Usage.Out
		emit(Event{Kind: "result", Title: fmt.Sprintf(tr("Xong lượt · %d token vào, %d token ra", "Turn done · %d tokens in, %d tokens out"), ev.Usage.In, ev.Usage.Out)})
	case "turn.failed", "error":
		msg := ev.Error.Message
		if msg == "" {
			msg = ev.Message
		}
		emit(Event{Kind: "error", Body: clip(msg), Error: true})
	}
}

// ---------- omp: omp -p --mode json ----------

// ompToolCall makes omp tool calls readable. omp sends a plain-language intent in "i", and MCP
// tools arrive as a write to "xd://mcp__<server>_<tool>" whose content holds the real arguments.
func ompToolCall(name string, raw json.RawMessage) (tool, title, body string) {
	tool, body = name, summarizeInput(raw)
	var args map[string]any
	if json.Unmarshal(raw, &args) != nil {
		return
	}
	title, _ = args["i"].(string)
	path, _ := args["path"].(string)
	if !strings.HasPrefix(path, "xd://") {
		return
	}
	tool = strings.TrimPrefix(strings.TrimPrefix(path, "xd://"), "mcp__")
	if strings.HasPrefix(tool, "cua_repl") {
		tool = "CUA"
	}
	body = ""
	content, _ := args["content"].(string)
	var inner map[string]any
	if content != "" && json.Unmarshal([]byte(content), &inner) == nil {
		if t, _ := inner["title"].(string); t != "" {
			body = t
		}
		if code, _ := inner["code"].(string); code != "" {
			body = strings.TrimSpace(body + "\n" + code)
		}
	} else if content != "" {
		body = content
	}
	return tool, title, clip(body)
}

type ompParser struct{ lastText string }

func (p *ompParser) Line(line string, emit func(Event), u *Usage) {
	var ev struct {
		Type    string `json:"type"`
		ID      string `json:"id"`
		Cwd     string `json:"cwd"`
		Error   string `json:"error"`
		Message struct {
			Role    string `json:"role"`
			Model   string `json:"model"`
			Content []struct {
				Type      string          `json:"type"`
				Text      string          `json:"text"`
				Thinking  string          `json:"thinking"`
				ID        string          `json:"id"`
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			} `json:"content"`
			Usage struct {
				In   int `json:"input"`
				Out  int `json:"output"`
				Cost struct {
					Total float64 `json:"total"`
				} `json:"cost"`
			} `json:"usage"`
			StopReason string `json:"stopReason"`
			ErrMsg     string `json:"errorMessage"`
		} `json:"message"`
		ToolCallID string `json:"toolCallId"`
		ToolName   string `json:"toolName"`
		IsError    bool   `json:"isError"`
		Result     struct {
			Content json.RawMessage `json:"content"`
		} `json:"result"`
	}
	if json.Unmarshal([]byte(line), &ev) != nil {
		if strings.TrimSpace(line) != "" {
			emit(Event{Kind: "log", Body: clip(line)})
		}
		return
	}
	switch ev.Type {
	case "session":
		u.Session = ev.ID
		emit(Event{Kind: "system", Title: tr("Phiên omp bắt đầu", "omp session started"), Body: "cwd " + ev.Cwd})
	case "message_end":
		m := ev.Message
		if m.Role != "assistant" {
			return
		}
		if m.Model != "" {
			u.Model = m.Model
		}
		u.InputTokens += m.Usage.In
		u.OutputTokens += m.Usage.Out
		u.CostUSD += m.Usage.Cost.Total
		for _, c := range m.Content {
			switch c.Type {
			case "text":
				if strings.TrimSpace(c.Text) != "" {
					p.lastText = c.Text
					emit(Event{Kind: "text", Body: clip(c.Text)})
				}
			case "thinking":
				if strings.TrimSpace(c.Thinking) != "" {
					emit(Event{Kind: "thinking", Body: clip(c.Thinking)})
				}
			case "toolCall":
				tool, title, body := ompToolCall(c.Name, c.Arguments)
				emit(Event{Kind: "tool_call", Tool: tool, Ref: c.ID, Title: title, Body: body})
			}
		}
		if m.StopReason == "error" && m.ErrMsg != "" {
			emit(Event{Kind: "error", Body: clip(m.ErrMsg), Error: true})
		}
	case "tool_execution_end":
		emit(Event{Kind: "tool_result", Ref: ev.ToolCallID, Tool: ev.ToolName, Body: clip(contentText(ev.Result.Content)), Error: ev.IsError})
	case "agent_end":
		emit(Event{Kind: "result", Title: fmt.Sprintf(tr("Xong · %d token vào, %d token ra", "Done · %d tokens in, %d tokens out"), u.InputTokens, u.OutputTokens), Body: clip(p.lastText)})
	case "error":
		emit(Event{Kind: "error", Body: clip(ev.Error), Error: true})
	}
}
