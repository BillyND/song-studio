package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestImageConfigMigration(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "c.json")
	os.WriteFile(path, []byte(`{"imageModel":"gpt-image-old","image":{"baseUrl":"http://evil.example/v1","size":"huge"}}`), 0o600)
	cs, err := loadConfig(path, home)
	if err != nil {
		t.Fatal(err)
	}
	im := cs.Get().Image
	if im.Provider != "hub" || im.Models["hub"] != "gpt-image-old" || im.Models["openai"] == "" || im.Size != "2048x1152" {
		t.Fatalf("migrated image config %+v", im)
	}
	// an invalid hub address is kept as typed (never silently swapped for another host) and gets no key
	if im.BaseURL != "http://evil.example/v1" || validBaseURL(im.BaseURL) {
		t.Fatalf("invalid base URL handling: %s", im.BaseURL)
	}
	store := &memStore{}
	store.Set("image.hub", "hub-key-123456789")
	if env := strings.Join(imageEnv(cs.Get(), store), " "); strings.Contains(env, "hub-key") {
		t.Fatalf("no key may go to an invalid hub address: %s", env)
	}
	if _, err := cs.Update(cs.Get(), home); err == nil {
		t.Fatal("saving settings with an invalid hub address must fail")
	}
	if cs.Get().ImageModel != "" {
		t.Fatal("legacy imageModel should be cleared after migration")
	}
}

func TestSecretsReachOnlyTheirRun(t *testing.T) {
	data := t.TempDir()
	store := &memStore{}
	store.Set("image.hub", "secret-value-1234")
	cs, _ := loadConfig(filepath.Join(t.TempDir(), "c.json"), t.TempDir())
	m, _ := newManager(data, func() (int, int) { return 1, 2 })
	defer m.Shutdown(time.Second)
	m.SecretEnv = func(names []string) []string { return imageEnv(cs.Get(), store) }
	t.Setenv("AIHUB_API_KEY", "leaked-from-shell")
	with, _ := m.Submit(RunSpec{Kind: "script", Cwd: data, Secrets: []string{"image"},
		Argv: []string{"/bin/sh", "-c", `test "${QH_IMAGE_API_KEY%1234}" = secret-value- && test "$QH_IMAGE_PROVIDER" = hub && test -z "$AIHUB_API_KEY"`}})
	without, _ := m.Submit(RunSpec{Kind: "agent", Cwd: data,
		Argv: []string{"/bin/sh", "-c", `test -z "$QH_IMAGE_API_KEY" && test -z "$AIHUB_API_KEY"`}})
	if got := waitStatus(t, m, with.ID, "succeeded", "failed"); got.Status != "succeeded" {
		t.Fatalf("image run should see its key: %+v", got)
	}
	if got := waitStatus(t, m, without.ID, "succeeded", "failed"); got.Status != "succeeded" {
		t.Fatalf("other runs must not see image keys: %+v", got)
	}
	for _, id := range []string{with.ID, without.ID} {
		for _, f := range []string{"meta.json", "events.jsonl", "stdout.log"} {
			b, _ := os.ReadFile(filepath.Join(data, "runs", id, f))
			if strings.Contains(string(b), "secret-value-1234") {
				t.Fatalf("%s of %s contains the key", f, id)
			}
		}
	}
}

func TestSecretEndpoints(t *testing.T) {
	home := t.TempDir()
	cs, _ := loadConfig(filepath.Join(home, "c.json"), home)
	m, _ := newManager(t.TempDir(), func() (int, int) { return 1, 1 })
	defer m.Shutdown(time.Second)
	app := &App{cfg: cs, runs: m, projects: Projects{cfg: cs}, secrets: &memStore{}, token: "tok", home: home, version: "t", build: "b", started: time.Now()}
	h := app.routes()
	call := func(method, path, body string) (int, string) {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Host = "127.0.0.1:4317"
		r.Header.Set("X-Studio-Token", "tok")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code, w.Body.String()
	}
	if code, _ := call("PUT", "/api/secrets/image.hub", `{"value":"short"}`); code != 400 {
		t.Fatalf("a too-short key must be refused, got %d", code)
	}
	if code, _ := call("PUT", "/api/secrets/other", `{"value":"abcdefgh12345"}`); code != 400 {
		t.Fatalf("unknown secret names must be refused, got %d", code)
	}
	code, body := call("PUT", "/api/secrets/image.openai", `{"value":"sk-test-abcdefgh9876"}`)
	if code != 200 || strings.Contains(body, "abcdefgh") || !strings.Contains(body, "…9876") {
		t.Fatalf("save should answer with a masked hint only: %d %s", code, body)
	}
	if _, body := call("GET", "/api/secrets", ""); strings.Contains(body, "sk-test") {
		t.Fatalf("listing leaked the key: %s", body)
	}
	os.MkdirAll(filepath.Dir(legacyHubKey(home)), 0o700)
	os.WriteFile(legacyHubKey(home), []byte("legacy-hub-key-5555\n"), 0o600)
	if code, body := call("POST", "/api/secrets/import-legacy", ""); code != 200 || !strings.Contains(body, "…5555") {
		t.Fatalf("import legacy: %d %s", code, body)
	}
	if code, _ := call("DELETE", "/api/secrets/image.openai", ""); code != 200 {
		t.Fatal("delete")
	}
	if _, err := app.secrets.Get("image.openai"); err == nil {
		t.Fatal("key should be gone")
	}
}

func TestCheckImageKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer good-key-123456" {
			w.WriteHeader(401)
			return
		}
		w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()
	cfg := Config{Image: ImageConfig{BaseURL: srv.URL + "/v1"}}
	if err := checkImageKey(cfg, "hub", "good-key-123456", srv.Client()); err != nil {
		t.Fatalf("good key: %v", err)
	}
	if err := checkImageKey(cfg, "hub", "bad-key-1234567", srv.Client()); err == nil || !strings.Contains(err.Error(), "từ chối") {
		t.Fatalf("bad key should be refused: %v", err)
	}
}

func TestRunTimeout(t *testing.T) {
	data := t.TempDir()
	m, _ := newManager(data, func() (int, int) { return 1, 1 })
	defer m.Shutdown(time.Second)
	meta, _ := m.Submit(RunSpec{Kind: "script", Cwd: data, TimeoutSec: 1, Argv: []string{"/bin/sh", "-c", "sleep 30"}})
	got := waitStatus(t, m, meta.ID, "failed", "succeeded", "stopped")
	if got.Status != "failed" || !strings.Contains(got.Error, "quá giới hạn") {
		t.Fatalf("a run past its limit should fail with a clear reason: %+v", got)
	}
}

func TestQueueSurvivesRestart(t *testing.T) {
	data := t.TempDir()
	m, _ := newManager(data, func() (int, int) { return 1, 1 })
	m.Start()
	metas, err := m.SubmitChain([]RunSpec{
		{Kind: "script", Cwd: data, Heavy: true, Argv: []string{"/bin/sh", "-c", "sleep 2; echo first"}},
		{Kind: "script", Cwd: data, Heavy: true, Argv: []string{"/bin/sh", "-c", "echo second"}},
	}, [][]int{nil, {0}})
	if err != nil {
		t.Fatal(err)
	}
	waitStatus(t, m, metas[0].ID, "running")
	m.Shutdown(5 * time.Second)

	read := func(id string) RunMeta {
		var meta RunMeta
		b, _ := os.ReadFile(filepath.Join(data, "runs", id, "meta.json"))
		json.Unmarshal(b, &meta)
		return meta
	}
	if a, b := read(metas[0].ID), read(metas[1].ID); a.Status != "queued" || b.Status != "queued" {
		t.Fatalf("after shutdown the running script and its follower should be queued: %s %s", a.Status, b.Status)
	}
	m2, _ := newManager(data, func() (int, int) { return 1, 1 })
	defer m2.Shutdown(time.Second)
	m2.Start()
	for _, mm := range metas {
		if got := waitStatus(t, m2, mm.ID, "succeeded", "failed"); got.Status != "succeeded" {
			t.Fatalf("resumed run %s: %+v", mm.ID, got)
		}
	}
}

func TestAgentInterruptedOnShutdown(t *testing.T) {
	data := t.TempDir()
	m, _ := newManager(data, func() (int, int) { return 1, 1 })
	m.Start()
	meta, _ := m.Submit(RunSpec{Kind: "agent", Cwd: data, Argv: []string{"/bin/sh", "-c", "sleep 20"}})
	waitStatus(t, m, meta.ID, "running")
	m.Shutdown(5 * time.Second)
	m2, _ := newManager(data, func() (int, int) { return 1, 1 })
	defer m2.Shutdown(time.Second)
	r, _ := m2.Get(meta.ID)
	if got := r.snapshot(); got.Status != "interrupted" {
		t.Fatalf("an agent cut off by shutdown is not re-run: %s", got.Status)
	}
}

func TestDoctorAndDiagnostics(t *testing.T) {
	home := t.TempDir()
	cs, _ := loadConfig(filepath.Join(home, "c.json"), home)
	data := t.TempDir()
	m, _ := newManager(data, func() (int, int) { return 1, 1 })
	defer m.Shutdown(time.Second)
	store := &memStore{}
	store.Set("image.hub", "hub-secret-key-777777")
	app := &App{cfg: cs, runs: m, projects: Projects{cfg: cs}, secrets: store, token: "tok", home: home, version: "t", build: "b", started: time.Now()}
	checks := app.doctor()
	ids := map[string]Check{}
	for _, c := range checks {
		ids[c.ID] = c
	}
	for _, id := range []string{"ffmpeg", "uv", "skill", "venv", "disk", "imagekey"} {
		if _, ok := ids[id]; !ok {
			t.Fatalf("doctor misses %s: %+v", id, checks)
		}
	}
	if ids["imagekey"].Status != "ok" || ids["skill"].Status != "bad" {
		t.Fatalf("key set and skill missing should show: %+v %+v", ids["imagekey"], ids["skill"])
	}
	// a failed run whose log echoed a key must come out redacted
	meta, _ := m.Submit(RunSpec{Kind: "script", Cwd: data, Argv: []string{"/bin/sh", "-c", "echo token hub-secret-key-777777 sk-abcdefghijklmnop1234 >&2; exit 3"}})
	waitStatus(t, m, meta.ID, "failed")
	os.WriteFile(filepath.Join(data, "studio.log"), []byte("Authorization: Bearer abcdefghijklmnopqrstu\n"), 0o644)
	r := httptest.NewRequest("GET", "/api/diagnostics", nil)
	r.Host = "127.0.0.1:4317"
	w := httptest.NewRecorder()
	app.routes().ServeHTTP(w, r)
	body := w.Body.String()
	if w.Code != 200 || !strings.HasPrefix(body, "PK") {
		t.Fatalf("diagnostics should be a zip: %d", w.Code)
	}
	zr, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, f := range zr.File {
		names = append(names, f.Name)
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		for _, leak := range []string{"hub-secret-key-777777", "sk-abcdefghijklmnop1234", "abcdefghijklmnopqrstu"} {
			if strings.Contains(string(b), leak) {
				t.Fatalf("%s leaks %s", f.Name, leak)
			}
		}
	}
	if !strings.Contains(strings.Join(names, ","), "runs/"+meta.ID+"/events.jsonl") {
		t.Fatalf("failed runs should carry their events: %v", names)
	}
}

func TestGenreLanguageAndTheme(t *testing.T) {
	home := t.TempDir()
	cs, _ := loadConfig(filepath.Join(home, "c.json"), home)
	c := cs.Get()
	c.ProjectsDir = filepath.Join(home, "songs")
	cs.cfg = c
	p := Projects{cfg: cs}
	if _, err := p.Create("X", "", "a", nil, "jazz-fusion", ""); err == nil {
		t.Fatal("unknown genre must be refused")
	}
	slug, err := p.Create("Đêm Phố", "", "a", nil, "pop", "en")
	if err != nil {
		t.Fatal(err)
	}
	read := func() map[string]any {
		var m map[string]any
		b, _ := os.ReadFile(filepath.Join(c.ProjectsDir, slug, "project.json"))
		json.Unmarshal(b, &m)
		return m
	}
	if m := read(); m["video_theme"] != "hien-dai" || m["karaoke_color"] != "blue" || m["language"] != "en" {
		t.Fatalf("pop song defaults: %v", m)
	}
	str := func(s string) *string { return &s }
	if err := p.SetMeta(slug, SongMeta{Genre: str("bolero")}); err != nil {
		t.Fatal(err)
	}
	if m := read(); m["video_theme"] != "co-dien" || m["karaoke_color"] != "gold" || m["title"] != "Đêm Phố" {
		t.Fatalf("genre change should move the theme and keep other keys: %v", m)
	}
	if err := p.SetMeta(slug, SongMeta{Theme: str("hien-dai")}); err != nil || read()["genre"] != "bolero" || read()["video_theme"] != "hien-dai" {
		t.Fatalf("theme alone: %v %v", err, read())
	}
	for _, bad := range []SongMeta{{Language: str("xx")}, {Theme: str("neon")}, {Genre: str("x")}} {
		if err := p.SetMeta(slug, bad); err == nil {
			t.Fatalf("invalid meta accepted: %+v", bad)
		}
	}
	d, _ := p.Detail(slug, nil)
	prompt, _ := buildActionPrompt("lyrics.agent", "/skill", d.Dir, ActionRequest{}, d)
	if !strings.Contains(prompt, "Thể loại: Bolero, trữ tình. Ngôn ngữ hát: Tiếng Anh.") {
		t.Fatalf("agents should be told the genre and language:\n%s", prompt)
	}
	if body := sceneBody(d.Dir, d.Config); !strings.Contains(body, "image_style") || !strings.Contains(body, "Nostalgic") {
		t.Fatalf("scene planning should ask for a genre image style:\n%s", body)
	}
}

func TestValidBaseURL(t *testing.T) {
	ok := []string{"https://aihub.tigrendev.com/v1", "https://api.openai.com/v1", "http://127.0.0.1:8080/v1", "http://localhost:4000/v1", "http://[::1]:9000/v1"}
	bad := []string{"http://127.0.0.1@evil.example/v1", "http://localhost.evil.example/v1", "http://192.168.1.5:8080/v1",
		"htps://x/v1", "https://user:pw@host/v1", "ftp://host", "", "https://host/v1?x=1"}
	for _, u := range ok {
		if !validBaseURL(u) {
			t.Errorf("should accept %s", u)
		}
	}
	for _, u := range bad {
		if validBaseURL(u) {
			t.Errorf("should refuse %s", u)
		}
	}
}

func TestSetMetaSafety(t *testing.T) {
	home := t.TempDir()
	cs, _ := loadConfig(filepath.Join(home, "c.json"), home)
	c := cs.Get()
	c.ProjectsDir = filepath.Join(home, "songs")
	cs.cfg = c
	p := Projects{cfg: cs}
	slug, _ := p.Create("Bài", "", "a", nil, "pop", "vi")
	pj := filepath.Join(c.ProjectsDir, slug, "project.json")
	str := func(s string) *string { return &s }
	// a theme picked by hand survives a genre change
	p.SetMeta(slug, SongMeta{Theme: str("co-dien")})
	p.SetMeta(slug, SongMeta{Genre: str("rock")})
	var m map[string]any
	b, _ := os.ReadFile(pj)
	json.Unmarshal(b, &m)
	if m["video_theme"] != "co-dien" || m["genre"] != "rock" {
		t.Fatalf("chosen theme must be kept: %v", m)
	}
	// an empty file (another writer mid-write) is never replaced by a stub
	os.WriteFile(pj, nil, 0o644)
	if err := p.SetMeta(slug, SongMeta{Genre: str("pop")}); err == nil {
		t.Fatal("SetMeta must refuse an empty project.json")
	}
	if b, _ := os.ReadFile(pj); len(b) != 0 {
		t.Fatalf("project.json was rewritten: %s", b)
	}
}

func TestStopAfterRequeueDoesNotPanic(t *testing.T) {
	data := t.TempDir()
	m, _ := newManager(data, func() (int, int) { return 1, 1 })
	m.Start()
	meta, _ := m.Submit(RunSpec{Kind: "script", Cwd: data, Heavy: true, Argv: []string{"/bin/sh", "-c", "sleep 20"}})
	waitStatus(t, m, meta.ID, "running")
	m.Shutdown(5 * time.Second)
	if err := m.Stop(meta.ID); err != nil { // the HTTP server can still take a stop while shutting down
		t.Fatal(err)
	}
}

func TestCleanExitAtLimitIsSuccess(t *testing.T) {
	data := t.TempDir()
	m, _ := newManager(data, func() (int, int) { return 1, 1 })
	defer m.Shutdown(time.Second)
	m.Start()
	meta, _ := m.Submit(RunSpec{Kind: "script", Cwd: data, TimeoutSec: 1, Argv: []string{"/bin/sh", "-c", "trap 'exit 0' TERM; sleep 5 & wait"}})
	got := waitStatus(t, m, meta.ID, "succeeded", "failed", "stopped")
	if got.Status != "succeeded" {
		t.Fatalf("a run that exits 0 when the limit hits is a success: %+v", got)
	}
}

func TestResumedAgentRebuildsArgv(t *testing.T) {
	data := t.TempDir()
	m, _ := newManager(data, func() (int, int) { return 0, 0 }) // nothing starts in the first process
	meta, _ := m.Submit(RunSpec{Kind: "agent", Cwd: data, Prompt: "p", Argv: []string{"/bin/sh", "-c", "exit 7"}})
	m.Shutdown(time.Second)
	m2, _ := newManager(data, func() (int, int) { return 1, 1 })
	defer m2.Shutdown(time.Second)
	m2.RebuildArgv = func(spec RunSpec) ([]string, error) { return []string{"/bin/sh", "-c", "exit 0"}, nil }
	m2.Start()
	if got := waitStatus(t, m2, meta.ID, "succeeded", "failed"); got.Status != "succeeded" {
		t.Fatalf("a resumed agent must run with the current settings: %+v", got)
	}
}
