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
	if im.BaseURL != defaultImage.BaseURL {
		t.Fatalf("plain http to another host must be refused: %s", im.BaseURL)
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
