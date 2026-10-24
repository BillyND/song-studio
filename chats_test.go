package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeClaude prints a stream-json session like claude -p does and records each call's
// arguments in calls.log, one record per call next to it. While a file named "slow" exists
// beside it, a call waits until that file is gone.
const fakeClaude = `#!/bin/sh
dir=$(dirname "$0")
for a in "$@"; do printf '%s\037' "$a"; done >> "$dir/calls.log"
printf '\036' >> "$dir/calls.log"
while [ -e "$dir/slow" ]; do sleep 0.05; done
echo '{"type":"system","subtype":"init","session_id":"sess-1","model":"m","cwd":"x"}'
echo '{"type":"result","subtype":"success","result":"ok","session_id":"sess-1"}'
`

func chatFixture(t *testing.T) (*ChatStore, *Manager, string) {
	t.Helper()
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	songs := filepath.Join(home, "songs")
	for _, d := range []string{bin, songs} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(fakeClaude), 0o755); err != nil {
		t.Fatal(err)
	}
	cs, _ := loadConfig(filepath.Join(home, "c.json"), home)
	c := cs.Get()
	c.ProjectsDir = songs
	c.Agents["claude"] = AgentConfig{Bin: filepath.Join(bin, "claude")}
	cs.cfg = c
	data := filepath.Join(home, "data")
	m, err := newManager(data, func() (int, int) { return 1, 2 })
	if err != nil {
		t.Fatal(err)
	}
	s, err := newChatStore(data, cs, m, Projects{cfg: cs})
	if err != nil {
		t.Fatal(err)
	}
	m.OnFinish = s.runFinished
	m.Start()
	t.Cleanup(func() { m.Shutdown(time.Second) })
	return s, m, bin
}

// calls returns the arguments of every fake CLI call so far.
func calls(t *testing.T, bin string) [][]string {
	t.Helper()
	data, _ := os.ReadFile(filepath.Join(bin, "calls.log"))
	out := [][]string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\036"), "\036") {
		if line != "" {
			out = append(out, strings.Split(strings.TrimSuffix(line, "\037"), "\037"))
		}
	}
	return out
}

func argAfter(argv []string, flag string) string {
	for i, a := range argv {
		if a == flag && i+1 < len(argv) {
			return argv[i+1]
		}
	}
	return ""
}

func lastTurn(t *testing.T, s *ChatStore, m *Manager, id string) RunMeta {
	t.Helper()
	v, err := s.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	return waitStatus(t, m, v.Turns[len(v.Turns)-1].Run, "succeeded", "failed", "stopped")
}

func waitTurns(t *testing.T, s *ChatStore, id string, n int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if v, _ := s.Get(id); len(v.Turns) >= n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("chat %s did not reach %d turns", id, n)
}

func TestChatResumesSession(t *testing.T) {
	s, m, bin := chatFixture(t)
	v, err := s.Create(ChatRequest{Agent: "claude", Target: "songs", Text: "hello there"})
	if err != nil {
		t.Fatal(err)
	}
	if got := lastTurn(t, s, m, v.ID); got.Status != "succeeded" || got.Chat != v.ID {
		t.Fatalf("first turn: %+v", got)
	}
	// runFinished records the session just after the run reports success
	deadline := time.Now().Add(5 * time.Second)
	for v, _ = s.Get(v.ID); v.Session == "" && time.Now().Before(deadline); v, _ = s.Get(v.ID) {
		time.Sleep(10 * time.Millisecond)
	}
	if v.Session != "sess-1" || v.Title != "hello there" {
		t.Fatalf("session not recorded: %+v", v)
	}
	if _, err := s.Send(v.ID, "and again", false, Where{}); err != nil {
		t.Fatal(err)
	}
	lastTurn(t, s, m, v.ID)
	c := calls(t, bin)
	if len(c) != 2 {
		t.Fatalf("want 2 calls, got %d", len(c))
	}
	first, second := c[0][len(c[0])-1], c[1][len(c[1])-1]
	if argAfter(c[0], "--resume") != "" || !strings.Contains(first, "khung chat") || !strings.HasSuffix(first, "hello there") {
		t.Fatalf("first turn must brief the agent and start a new session: %q", c[0])
	}
	if argAfter(c[1], "--resume") != "sess-1" || second != "and again" {
		t.Fatalf("follow-up must resume the session with just the message: %q", c[1])
	}
}

func TestChatQueuesWhileRunning(t *testing.T) {
	s, m, bin := chatFixture(t)
	slow := filepath.Join(bin, "slow")
	os.WriteFile(slow, nil, 0o644)
	v, _ := s.Create(ChatRequest{Agent: "claude", Target: "songs", Text: "one"})
	first := v.Turns[0].Run
	waitStatus(t, m, first, "running")
	s.Send(v.ID, "two", false, Where{})
	v, _ = s.Send(v.ID, "three", false, Where{})
	if len(v.Queue) != 2 || len(v.Turns) != 1 {
		t.Fatalf("messages sent during a turn must wait: %+v", v)
	}
	if v, _ = s.RemoveQueued(v.ID, 0); len(v.Queue) != 1 || v.Queue[0] != "three" {
		t.Fatalf("remove queued: %+v", v.Queue)
	}
	s.Send(v.ID, "four", false, Where{})
	os.Remove(slow)
	waitStatus(t, m, first, "succeeded")
	waitTurns(t, s, v.ID, 2) // runFinished sends the queue just after the first turn reports success
	got := lastTurn(t, s, m, v.ID)
	v, _ = s.Get(v.ID)
	if len(v.Turns) != 2 || v.Turns[1].Text != "three\n\nfour" || len(v.Queue) != 0 || got.Resume != "sess-1" {
		t.Fatalf("queued messages must go out together, resuming the session: %+v", v)
	}
}

func TestChatStopAndSend(t *testing.T) {
	s, m, bin := chatFixture(t)
	slow := filepath.Join(bin, "slow")
	os.WriteFile(slow, nil, 0o644)
	v, _ := s.Create(ChatRequest{Agent: "claude", Target: "songs", Text: "long job"})
	first := v.Turns[0].Run
	waitStatus(t, m, first, "running")
	os.Remove(slow) // the next turn should finish
	if _, err := s.Send(v.ID, "change of plan", true, Where{}); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, m, first, "stopped", "succeeded")
	waitTurns(t, s, v.ID, 2)
	if v, _ = s.Get(v.ID); len(v.Turns) != 2 || v.Turns[1].Text != "change of plan" {
		t.Fatalf("stop & send must start the new message: %+v", v)
	}
	lastTurn(t, s, m, v.ID)
}

func TestChatContinuesStepRun(t *testing.T) {
	s, m, bin := chatFixture(t)
	cfg := s.cfg.Get()
	songs := cfg.ProjectsDir
	argv, _ := agentArgv(cfg, "claude", "", "review the lyrics", songs)
	step, _ := m.Submit(RunSpec{Kind: "agent", Agent: "claude", Title: "Soát lời", Cwd: songs, Prompt: "review the lyrics", Argv: argv})
	waitStatus(t, m, step.ID, "succeeded")
	v, err := s.Create(ChatRequest{FromRun: step.ID})
	if err != nil {
		t.Fatal(err)
	}
	if v.Session != "sess-1" || v.Title != "Soát lời" || len(v.Turns) != 1 || v.Turns[0].Run != step.ID {
		t.Fatalf("chat from a run: %+v", v)
	}
	if again, _ := s.Create(ChatRequest{FromRun: step.ID}); again.ID != v.ID {
		t.Fatal("replying to the same run twice must reopen its chat")
	}
	s.Send(v.ID, "shorter chorus", false, Where{})
	lastTurn(t, s, m, v.ID)
	c := calls(t, bin)
	last := c[len(c)-1]
	prompt := last[len(last)-1]
	if argAfter(last, "--resume") != "sess-1" || last[len(last)-2] != "--" || !strings.Contains(prompt, "[Song Studio]") || !strings.HasSuffix(prompt, "shorter chorus") {
		t.Fatalf("the first reply must tell the agent it may ask back: %q", last)
	}
	if !chatRunIDs(filepath.Dir(m.dir))[step.ID] {
		t.Fatal("pruning must keep runs a chat points at")
	}
}

func TestChatSurvivesRestart(t *testing.T) {
	s, m, _ := chatFixture(t)
	v, _ := s.Create(ChatRequest{Agent: "claude", Target: "songs", Text: "hi"})
	lastTurn(t, s, m, v.ID)
	s.Send(v.ID, "more", false, Where{}) // records the first turn's session as it starts the second
	lastTurn(t, s, m, v.ID)
	s2, err := newChatStore(filepath.Dir(m.dir), s.cfg, m, s.projects)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s2.Get(v.ID)
	if err != nil || got.Session != "sess-1" || len(got.Turns) != 2 || got.Status != "succeeded" {
		t.Fatalf("chat not reloaded: %+v %v", got, err)
	}
	if err := s2.Delete(v.ID); err != nil {
		t.Fatal(err)
	}
	if len(s2.List()) != 0 {
		t.Fatal("deleted chat still listed")
	}
	if _, err := s.Create(ChatRequest{Agent: "claude", Target: "nowhere", Text: "x"}); err == nil {
		t.Fatal("unknown target accepted")
	}
}

func TestAgentResumeArgv(t *testing.T) {
	cfg := defaultConfig("/home/x")
	cfg.PermissionMode = "edits"
	argv, _ := agentResumeArgv(cfg, "codex", "gpt-x", "msg", "/w", "th-1")
	if strings.Join(argv[1:], " ") != `exec resume --json --skip-git-repo-check -c sandbox_mode="workspace-write" -m gpt-x -- th-1 msg` {
		t.Fatalf("codex resume argv: %v", argv)
	}
	cfg.PermissionMode = "full"
	argv, _ = agentResumeArgv(cfg, "omp", "", "msg", "/w", "o-1")
	if argAfter(argv, "--resume") != "o-1" || argAfter(argv, "--cwd") != "/w" || argv[len(argv)-1] != "msg" {
		t.Fatalf("omp resume argv: %v", argv)
	}
}

func TestChatMessageStartingWithDash(t *testing.T) {
	s, m, bin := chatFixture(t)
	v, _ := s.Create(ChatRequest{Agent: "claude", Target: "songs", Text: "hi"})
	lastTurn(t, s, m, v.ID)
	s.Send(v.ID, "- shorter chorus\n- slower", false, Where{})
	if got := lastTurn(t, s, m, v.ID); got.Status != "succeeded" {
		t.Fatalf("a message that starts with a dash must reach the agent as the prompt: %+v", got)
	}
	c := calls(t, bin)
	last := c[len(c)-1]
	if last[len(last)-2] != "--" || last[len(last)-1] != "- shorter chorus\n- slower" {
		t.Fatalf("argv: %q", last)
	}
}

func TestChatRefusesAgentThatCannotRun(t *testing.T) {
	s, _, _ := chatFixture(t)
	c := s.cfg.Get()
	c.PermissionMode = "edits"
	s.cfg.cfg = c
	if _, err := s.Create(ChatRequest{Agent: "omp", Target: "songs", Text: "hi"}); err == nil {
		t.Fatal("omp without Full access must be refused")
	}
	if len(s.List()) != 0 {
		t.Fatal("a refused chat must not be saved")
	}
}

func TestChatSaveFailureKeepsStoreUsable(t *testing.T) {
	s, m, _ := chatFixture(t)
	v, _ := s.Create(ChatRequest{Agent: "claude", Target: "songs", Text: "hi"})
	lastTurn(t, s, m, v.ID)
	os.Chmod(s.dir, 0o500) // saving now fails, after the turn is submitted
	defer os.Chmod(s.dir, 0o755)
	if _, err := s.Send(v.ID, "again", false, Where{}); err == nil {
		t.Fatal("a failed save must be reported")
	}
	done := make(chan struct{})
	go func() { s.List(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the chat store stayed locked after a failed save")
	}
	if got, _ := s.Get(v.ID); len(got.Turns) != 2 {
		t.Fatalf("the submitted turn must stay recorded: %+v", got.Turns)
	}
	lastTurn(t, s, m, v.ID)
}

func TestChatTellsTheAgentWhereTheUserIs(t *testing.T) {
	s, m, bin := chatFixture(t)
	songs := s.cfg.Get().ProjectsDir
	os.MkdirAll(filepath.Join(songs, "hue"), 0o755)
	os.WriteFile(filepath.Join(songs, "hue", "project.json"), []byte(`{"title":"Huế","subtitle":"Mộng Mơ"}`), 0o644)
	at := Where{Page: "project", Project: "hue", Step: "suno"}
	v, err := s.Create(ChatRequest{Agent: "claude", Target: "songs", Text: "shorten this chorus", Where: at})
	if err != nil {
		t.Fatal(err)
	}
	lastTurn(t, s, m, v.ID)
	s.Send(v.ID, "and the verse", false, at) // same screen: no note again
	lastTurn(t, s, m, v.ID)
	s.Send(v.ID, "now this one", false, Where{Page: "skill", File: "references/style-profiles.md"})
	lastTurn(t, s, m, v.ID)
	c := calls(t, bin)
	first, second, third := c[0][len(c[0])-1], c[1][len(c[1])-1], c[2][len(c[2])-1]
	if !strings.Contains(first, `"Huế Mộng Mơ"`) || !strings.Contains(first, filepath.Join(songs, "hue")) || !strings.Contains(first, "Prompt Suno") {
		t.Fatalf("the first turn must name the song, its folder and step: %q", first)
	}
	if second != "and the verse" {
		t.Fatalf("an unchanged screen must not be repeated: %q", second)
	}
	if !strings.Contains(third, "references/style-profiles.md") || !strings.HasSuffix(third, "now this one") {
		t.Fatalf("a new screen must be named: %q", third)
	}
	if got, _ := s.Get(v.ID); got.Turns[2].Text != "now this one" {
		t.Fatalf("the chat shows what the user wrote, not the note: %q", got.Turns[2].Text)
	}
}
