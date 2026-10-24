package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(dir, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		mode := os.FileMode(0o644)
		if strings.HasSuffix(rel, ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(p, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSyncBundledSkill(t *testing.T) {
	root := t.TempDir()
	bundle, target := filepath.Join(root, "bundle"), filepath.Join(root, "skills", "suno-que-huong")
	state, backups := filepath.Join(root, "state.json"), filepath.Join(root, "backups")
	cache := "__" + "pycache__"
	writeSkill(t, bundle, map[string]string{"SKILL.md": "v1", "scripts/run.sh": "echo", "scripts/" + cache + "/x.pyc": "junk"})
	sync := func() string {
		t.Helper()
		note, err := syncBundledSkill(bundle, target, state, backups)
		if err != nil {
			t.Fatal(err)
		}
		return note
	}
	if n := sync(); n != "đã cài skill" {
		t.Fatalf("fresh machine: %s", n)
	}
	if st, _ := os.Stat(filepath.Join(target, "scripts", "run.sh")); st.Mode().Perm()&0o100 == 0 {
		t.Fatal("exec bit lost")
	}
	if exists(filepath.Join(target, "scripts", cache)) {
		t.Fatal("caches must not be installed")
	}
	if n := sync(); n != "skill đã mới nhất" {
		t.Fatalf("second start: %s", n)
	}
	// the user edits the skill: the same snapshot must not undo it
	os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("my edit"), 0o644)
	if n := sync(); n != "giữ các sửa đổi skill trên máy" {
		t.Fatalf("local edit: %s", n)
	}
	if b, _ := os.ReadFile(filepath.Join(target, "SKILL.md")); string(b) != "my edit" {
		t.Fatal("local edit was overwritten")
	}
	// a new app version brings a new snapshot: local edits are backed up, then replaced
	writeSkill(t, bundle, map[string]string{"SKILL.md": "v2"})
	n := sync()
	if !strings.Contains(n, "đã cập nhật skill") || !strings.Contains(n, "sao lưu") {
		t.Fatalf("update over local edits: %s", n)
	}
	if b, _ := os.ReadFile(filepath.Join(target, "SKILL.md")); string(b) != "v2" {
		t.Fatal("skill not updated")
	}
	saved, _ := filepath.Glob(filepath.Join(backups, "*", "SKILL.md"))
	if len(saved) != 1 {
		t.Fatalf("backup missing: %v", saved)
	}
	if b, _ := os.ReadFile(saved[0]); string(b) != "my edit" {
		t.Fatal("backup holds the wrong content")
	}
	// the next snapshot over an untouched install needs no backup
	writeSkill(t, bundle, map[string]string{"SKILL.md": "v3"})
	if n := sync(); n != "đã cập nhật skill" {
		t.Fatalf("clean update: %s", n)
	}
	if matches, _ := filepath.Glob(filepath.Join(backups, "*")); len(matches) != 1 {
		t.Fatalf("no new backup expected: %v", matches)
	}
}
