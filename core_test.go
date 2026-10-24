package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGuard(t *testing.T) {
	h := guard("127.0.0.1:4317", "tok", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	cases := []struct {
		method, host, origin, token string
		want                        int
	}{
		{"GET", "127.0.0.1:4317", "", "", 204},
		{"GET", "localhost:4317", "", "", 204},
		{"GET", "evil.example:4317", "", "", 403},  // DNS rebinding
		{"POST", "127.0.0.1:4317", "", "", 403},    // missing token
		{"POST", "127.0.0.1:4317", "", "bad", 403}, // wrong token
		{"POST", "127.0.0.1:4317", "http://127.0.0.1:4317", "tok", 204},
		{"POST", "127.0.0.1:4317", "https://evil.example", "tok", 403}, // cross-site
		{"PUT", "127.0.0.1:4317", "", "tok", 204},
	}
	for _, c := range cases {
		r := httptest.NewRequest(c.method, "/x", nil)
		r.Host = c.host
		if c.origin != "" {
			r.Header.Set("Origin", c.origin)
		}
		if c.token != "" {
			r.Header.Set("X-Studio-Token", c.token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != c.want {
			t.Errorf("%+v: got %d", c, w.Code)
		}
	}
}

func TestSafeJoin(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "a"), 0o755)
	outside := t.TempDir()
	os.Symlink(outside, filepath.Join(root, "link"))
	ok := []string{"a", "a/new.md", "SKILL.md", "/a/../a/x"}
	bad := []string{"../x", "a/../../x", "link/x", "", "a\x00b"}
	for _, p := range ok {
		if _, err := safeJoin(root, p); err != nil {
			t.Errorf("%q should be allowed: %v", p, err)
		}
	}
	for _, p := range bad {
		if got, err := safeJoin(root, p); err == nil && !strings.HasPrefix(got, root) || err == nil && p == "link/x" {
			t.Errorf("%q should be refused, got %q", p, got)
		}
	}
}

func TestSkillFileConflict(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("v1"), 0o644)
	f, err := readSkillFile(root, "SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writeSkillFile(root, "SKILL.md", "v2", f.SHA); err != nil {
		t.Fatal(err)
	}
	if _, err := writeSkillFile(root, "SKILL.md", "v3", f.SHA); err != errConflict {
		t.Fatalf("stale sha must conflict, got %v", err)
	}
	if _, err := writeSkillFile(root, "new.md", "x", ""); err != nil {
		t.Fatalf("create new file: %v", err)
	}
	if _, err := writeSkillFile(root, "new.md", "y", ""); err != errConflict {
		t.Fatal("creating over an existing file must conflict")
	}
	if _, err := writeSkillFile(root, "font.ttf", "x", ""); err == nil {
		t.Fatal("binary types are not editable")
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{"Ninh Bình Quê Hương": "ninh-binh-que-huong", "Đà Lạt  mộng mơ!": "da-lat-mong-mo", "   ": "bai-hat"}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestProjectCreateAndSteps(t *testing.T) {
	home := t.TempDir()
	cs, _ := loadConfig(filepath.Join(home, "c.json"), home)
	c := cs.Get()
	c.ProjectsDir = filepath.Join(home, "songs")
	cs.cfg = c
	p := Projects{cfg: cs}
	slug, err := p.Create("Huế", "Mộng Mơ", "Huế ơi mộng mơ sông Hương\nThuyền ai lờ lững vấn vương câu hò", [][2]string{{"Sáng tác: ", "Nguyễn Văn Thanh"}})
	if err != nil || slug != "hue-mong-mo" {
		t.Fatalf("create: %v %q", err, slug)
	}
	if again, _ := p.Create("Huế", "Mộng Mơ", "x", nil); again != "hue-mong-mo-2" {
		t.Fatalf("duplicate slug not suffixed: %q", again)
	}
	d, err := p.Detail(slug, map[string]string{"align": "run1"})
	if err != nil {
		t.Fatal(err)
	}
	status := map[string]string{}
	for _, s := range d.Steps {
		status[s.ID] = s.Status
	}
	if status["lyrics"] != "done" || status["suno"] != "todo" || status["audio"] != "todo" || status["align"] != "running" {
		t.Fatalf("steps: %v", status)
	}
	if err := p.WriteFile(slug, "project.json", "{bad"); err == nil {
		t.Fatal("invalid project.json must be rejected")
	}
	if err := p.WriteFile(slug, "../../etc/passwd", "x"); err == nil {
		t.Fatal("non-editable path accepted")
	}
	if _, err := p.dir("../x"); err == nil {
		t.Fatal("slug traversal accepted")
	}
	dst, err := p.ImportAudio(slug, "song.m4a", strings.NewReader("data"))
	if err != nil || !strings.HasSuffix(dst, "Huế Mộng Mơ.m4a") {
		t.Fatalf("import: %v %q", err, dst)
	}
	var cfg map[string]any
	b, _ := os.ReadFile(filepath.Join(d.Dir, "project.json"))
	json.Unmarshal(b, &cfg)
	if cfg["audio"] != dst {
		t.Fatalf("audio not recorded: %v", cfg["audio"])
	}
}

func TestAgentArgv(t *testing.T) {
	cfg := defaultConfig("/home/x")
	cfg.Agents["claude"] = AgentConfig{Bin: "claude", Model: "opus"}
	argv, _ := agentArgv(cfg, "claude", "", "hi; rm -rf /", "/w")
	joined := strings.Join(argv, "|")
	if !strings.Contains(joined, "-p|hi; rm -rf /|") || !strings.Contains(joined, "--model|opus") || !strings.Contains(joined, "bypassPermissions") {
		t.Fatalf("claude argv: %v", argv)
	}
	cfg.PermissionMode = "edits"
	argv, _ = agentArgv(cfg, "codex", "gpt-x", "p", "/w")
	if argv[len(argv)-1] != "p" || !strings.Contains(strings.Join(argv, " "), "--sandbox workspace-write") || !strings.Contains(strings.Join(argv, " "), "-m gpt-x") {
		t.Fatalf("codex argv: %v", argv)
	}
	if _, err := agentArgv(cfg, "rm", "", "p", "/w"); err == nil {
		t.Fatal("unknown agent accepted")
	}
}

func waitStatus(t *testing.T, m *Manager, id string, want ...string) RunMeta {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		r, _ := m.Get(id)
		meta := r.snapshot()
		for _, w := range want {
			if meta.Status == w {
				return meta
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	r, _ := m.Get(id)
	t.Fatalf("run %s did not reach %v, is %s", id, want, r.snapshot().Status)
	return RunMeta{}
}

func TestRunLifecycle(t *testing.T) {
	data := t.TempDir()
	m, err := newManager(data, func() int { return 1 })
	if err != nil {
		t.Fatal(err)
	}
	ok, _ := m.Submit(RunSpec{Kind: "script", Title: "echo", Cwd: data, Argv: []string{"/bin/sh", "-c", "echo one; echo two 1>&2"}})
	slow, _ := m.Submit(RunSpec{Kind: "script", Title: "sleep", Cwd: data, Argv: []string{"/bin/sh", "-c", "sleep 30"}})
	queued, _ := m.Submit(RunSpec{Kind: "script", Title: "never", Cwd: data, Argv: []string{"/bin/sh", "-c", "echo never"}})

	meta := waitStatus(t, m, ok.ID, "succeeded")
	if *meta.ExitCode != 0 {
		t.Fatal("exit code")
	}
	r, _ := m.Get(ok.ID)
	logs := []string{}
	for _, e := range m.Events(r, 0) {
		if e.Kind == "log" {
			logs = append(logs, e.Body)
		}
	}
	if strings.Join(logs, ",") != "one,two" && strings.Join(logs, ",") != "two,one" {
		t.Fatalf("logs %v", logs)
	}
	waitStatus(t, m, slow.ID, "running")
	if s, _ := m.Get(queued.ID); s.snapshot().Status != "queued" {
		t.Fatal("concurrency limit not respected")
	}
	if err := m.Stop(queued.ID); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, m, queued.ID, "stopped")
	start := time.Now()
	m.Stop(slow.ID)
	waitStatus(t, m, slow.ID, "stopped")
	if time.Since(start) > 5*time.Second {
		t.Fatal("stop took too long")
	}
	if _, err := os.Stat(filepath.Join(data, "runs", ok.ID, "events.jsonl")); err != nil {
		t.Fatal("events not persisted")
	}
	m.Shutdown(time.Second)

	// a fresh manager reloads history
	m2, _ := newManager(data, func() int { return 1 })
	if len(m2.List(10)) != 3 {
		t.Fatalf("history not reloaded: %d", len(m2.List(10)))
	}
	r2, _ := m2.Get(ok.ID)
	if len(m2.Events(r2, 0)) == 0 {
		t.Fatal("events not readable from disk")
	}
	m2.Shutdown(time.Second)
}

func TestFailedRun(t *testing.T) {
	data := t.TempDir()
	m, _ := newManager(data, func() int { return 2 })
	defer m.Shutdown(time.Second)
	r, _ := m.Submit(RunSpec{Kind: "script", Cwd: data, Argv: []string{"/bin/sh", "-c", "exit 3"}})
	meta := waitStatus(t, m, r.ID, "failed")
	if *meta.ExitCode != 3 {
		t.Fatalf("exit %d", *meta.ExitCode)
	}
	if _, err := m.Submit(RunSpec{Kind: "script", Cwd: "/nonexistent", Argv: []string{"true"}}); err == nil {
		t.Fatal("missing cwd accepted")
	}
}

func TestDefaultStillTimes(t *testing.T) {
	d := ProjectDetail{Timings: []TimingLine{{Text: "a", Start: 11.8, End: 15}, {Text: "long line here", Start: 16, End: 20}, {Text: "b", Start: 40, End: 44}}}
	got := defaultStillTimes(d)
	if len(got) < 4 || got[0] != 3 {
		t.Fatalf("times %v", got)
	}
	for i := 1; i < len(got); i++ {
		if got[i] <= got[i-1] {
			t.Fatalf("not increasing %v", got)
		}
	}
}

func TestVideoStampsDriveStaleness(t *testing.T) {
	home := t.TempDir()
	cs, _ := loadConfig(filepath.Join(home, "c.json"), home)
	c := cs.Get()
	c.ProjectsDir = filepath.Join(home, "songs")
	cs.cfg = c
	p := Projects{cfg: cs}
	slug, _ := p.Create("Đà Lạt", "", "a\nb", nil)
	d, _ := p.Detail(slug, nil)
	os.WriteFile(filepath.Join(d.Dir, "Đà Lạt - Karaoke.mp4"), []byte("v"), 0o644)
	if err := p.RecordStamp(slug, "render", videoInputsHash(d.Dir)); err != nil {
		t.Fatal(err)
	}
	status := func() string {
		d, _ := p.Detail(slug, nil)
		for _, s := range d.Steps {
			if s.ID == "render" {
				return s.Status
			}
		}
		return ""
	}
	if got := status(); got != "done" {
		t.Fatalf("fresh render should be done, got %s", got)
	}
	// a field the renderer ignores must not invalidate the video
	var cfg map[string]any
	b, _ := os.ReadFile(filepath.Join(d.Dir, "project.json"))
	json.Unmarshal(b, &cfg)
	cfg["vocal"] = "Female"
	nb, _ := json.Marshal(cfg)
	p.WriteFile(slug, "project.json", string(nb))
	if got := status(); got != "done" {
		t.Fatalf("unrelated edit made render %s", got)
	}
	cfg["title"] = "Đà Lạt Mới"
	nb, _ = json.Marshal(cfg)
	p.WriteFile(slug, "project.json", string(nb))
	if got := status(); got != "stale" {
		t.Fatalf("title change should make render stale, got %s", got)
	}
}

// Subscribing and cancelling while a run streams must never panic (send on closed channel).
func TestSubscribeCancelWhileStreaming(t *testing.T) {
	data := t.TempDir()
	m, _ := newManager(data, func() int { return 1 })
	defer m.Shutdown(time.Second)
	r, _ := m.Submit(RunSpec{Kind: "script", Cwd: data, Argv: []string{"/bin/sh", "-c", "i=0; while [ $i -lt 3000 ]; do echo line $i; echo err $i 1>&2; i=$((i+1)); done"}})
	run, _ := m.Get(r.ID)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
			}
			_, ch, cancel := m.Subscribe(run, 0)
			if ch != nil {
				select {
				case <-ch:
				case <-time.After(time.Millisecond):
				}
			}
			cancel()
		}
	}()
	meta := waitStatus(t, m, r.ID, "succeeded", "failed")
	close(done)
	if meta.Status != "succeeded" {
		t.Fatalf("status %s", meta.Status)
	}
	events := m.Events(run, 0)
	for i := 1; i < len(events); i++ {
		if events[i].Seq != events[i-1].Seq+1 {
			t.Fatalf("events out of order at %d: %d after %d", i, events[i].Seq, events[i-1].Seq)
		}
	}
}

// A background child that keeps stdout open must not hold the run (and the queue) forever.
func TestDetachedChildDoesNotHangRun(t *testing.T) {
	data := t.TempDir()
	m, _ := newManager(data, func() int { return 1 })
	defer m.Shutdown(time.Second)
	start := time.Now()
	r, _ := m.Submit(RunSpec{Kind: "script", Cwd: data, Argv: []string{"/bin/sh", "-c", "sleep 8 & echo started"}})
	waitStatus(t, m, r.ID, "succeeded", "failed")
	if time.Since(start) > 6*time.Second {
		t.Fatalf("run waited for the detached child: %s", time.Since(start))
	}
	run, _ := m.Get(r.ID)
	if pid := run.snapshot().PID; pid > 0 {
		killGroup(pid, 9)
	}
}

func TestGuardRejectsCrossSiteFetch(t *testing.T) {
	h := guard("127.0.0.1:4317", "tok", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	for site, want := range map[string]int{"cross-site": 403, "same-site": 403, "same-origin": 204, "none": 204, "": 204} {
		r := httptest.NewRequest("GET", "/media/x/y.png", nil)
		r.Host = "127.0.0.1:4317"
		if site != "" {
			r.Header.Set("Sec-Fetch-Site", site)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Errorf("Sec-Fetch-Site %q: got %d want %d", site, w.Code, want)
		}
	}
}

func TestSafeFileNameAndVideos(t *testing.T) {
	cases := map[string]string{"Phần 1/2": "Phần 1-2", "../../evil": "-..-evil", "  ": "fallback", "Huế: mộng mơ": "Huế- mộng mơ"}
	for in, want := range cases {
		if got := safeFileName(in, "fallback"); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Song.mp4"), []byte("audio"), 0o644)
	os.WriteFile(filepath.Join(dir, "Song - Karaoke.mp4"), []byte("v"), 0o644)
	got := karaokeVideos(dir, filepath.Join(dir, "Song.mp4"))
	if len(got) != 1 || got[0] != "Song - Karaoke.mp4" {
		t.Fatalf("videos %v", got)
	}
}
