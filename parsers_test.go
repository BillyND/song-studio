package main

import (
	"bufio"
	"os"
	"testing"
)

func parseFixture(t *testing.T, agent, file string, extra ...string) ([]Event, Usage) {
	t.Helper()
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	p := newParser("agent", agent)
	var events []Event
	var u Usage
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	lines := []string{}
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	for _, l := range append(lines, extra...) {
		p.Line(l, func(e Event) { events = append(events, e) }, &u)
	}
	return events, u
}

func kinds(events []Event) map[string]int {
	m := map[string]int{}
	for _, e := range events {
		m[e.Kind]++
	}
	return m
}

func TestClaudeParser(t *testing.T) {
	events, u := parseFixture(t, "claude", "testdata/claude.jsonl")
	k := kinds(events)
	if k["system"] != 1 || k["tool_call"] != 1 || k["tool_result"] != 1 || k["text"] != 1 || k["result"] != 1 {
		t.Fatalf("unexpected kinds %v", k)
	}
	var call, res Event
	for _, e := range events {
		switch e.Kind {
		case "tool_call":
			call = e
		case "tool_result":
			res = e
		}
	}
	if call.Tool != "Bash" || call.Body != "echo hi-from-tool" || call.Ref == "" || res.Ref != call.Ref || res.Body != "hi-from-tool" {
		t.Fatalf("tool pairing wrong: %+v / %+v", call, res)
	}
	if u.CostUSD <= 0 || u.Session == "" {
		t.Fatalf("usage not captured: %+v", u)
	}
}

func TestCodexParser(t *testing.T) {
	events, u := parseFixture(t, "codex", "testdata/codex.jsonl")
	k := kinds(events)
	if k["tool_call"] != 2 || k["tool_result"] != 2 || k["text"] != 2 || k["result"] != 1 {
		t.Fatalf("unexpected kinds %v", k)
	}
	last := events[len(events)-2]
	if last.Kind != "text" || last.Body != "DONE" {
		t.Fatalf("final message wrong: %+v", last)
	}
	if u.InputTokens == 0 || u.Session == "" {
		t.Fatalf("usage not captured: %+v", u)
	}
}

func TestOmpParser(t *testing.T) {
	events, u := parseFixture(t, "omp", "testdata/omp.jsonl", `{"type":"agent_end","messages":[]}`)
	k := kinds(events)
	if k["tool_call"] != 1 || k["tool_result"] != 1 || k["text"] != 1 || k["result"] != 1 || k["system"] != 1 {
		t.Fatalf("unexpected kinds %v", k)
	}
	for _, e := range events {
		if e.Kind == "tool_call" && e.Body != "echo hi-from-tool" {
			t.Fatalf("tool summary wrong: %q", e.Body)
		}
		if e.Kind == "result" && e.Body != "DONE" {
			t.Fatalf("result should carry last text, got %q", e.Body)
		}
	}
	if u.InputTokens == 0 || u.Model == "" {
		t.Fatalf("usage not captured: %+v", u)
	}
}

func TestNonJSONLinesBecomeLogs(t *testing.T) {
	var got []Event
	for _, agent := range []string{"claude", "codex", "omp"} {
		newParser("agent", agent).Line("plain warning", func(e Event) { got = append(got, e) }, &Usage{})
	}
	if len(got) != 3 || got[0].Kind != "log" {
		t.Fatalf("got %+v", got)
	}
}

func TestClipKeepsUTF8(t *testing.T) {
	s := ""
	for len(s) < maxBody+50 {
		s += "ước "
	}
	out := clip(s)
	for i := 0; i < len(out); {
		r, size := []rune(out[i:])[0], len(string([]rune(out[i:])[0]))
		if r == '�' {
			t.Fatal("clip split a rune")
		}
		i += size
	}
}

func TestSummarizeInput(t *testing.T) {
	cases := map[string]string{
		`{"command":"ls -la","description":"list"}`:                 "ls -la",
		`{"file_path":"/a/b.md"}`:                                   "/a/b.md",
		`{"file_path":"/a/b.md","old_string":"x","new_string":"y"}`: "/a/b.md  (sửa nội dung)",
		`{"b":1,"a":2}`: "a: 2\nb: 1",
	}
	for in, want := range cases {
		if got := summarizeInput([]byte(in)); got != want {
			t.Errorf("%s: got %q want %q", in, got, want)
		}
	}
}

func TestOmpToolCallReadable(t *testing.T) {
	tool, title, body := ompToolCall("write", []byte(`{"i":"Opening Suno creation page","path":"xd://mcp__cua_repl_js","content":"{\"code\":\"await app.pressKey('super+t');\",\"title\":\"Mở tab Suno\"}"}`))
	if tool != "CUA" || title != "Opening Suno creation page" || body != "Mở tab Suno\nawait app.pressKey('super+t');" {
		t.Fatalf("got %q %q %q", tool, title, body)
	}
	tool, title, body = ompToolCall("bash", []byte(`{"i":"Running echo","command":"echo hi"}`))
	if tool != "bash" || title != "Running echo" || body != "echo hi" {
		t.Fatalf("got %q %q %q", tool, title, body)
	}
}
