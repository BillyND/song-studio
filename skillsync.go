package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The app carries a snapshot of the skill. On start it installs that snapshot into the skill
// folder when the snapshot is new, so friends get skill fixes with each app update. Local edits
// to the skill are kept until a newer snapshot arrives, and then backed up before replacing.

// skipInSkill are build and cache files that never belong to the skill's content.
func skipInSkill(rel string, d fs.DirEntry) bool {
	name := d.Name()
	return name == "__pycache__" || name == ".DS_Store" || name == ".git" || strings.HasSuffix(name, ".pyc") ||
		(d.IsDir() && rel == "plans")
}

// skillHash fingerprints a skill folder by relative path, mode and content.
func skillHash(dir string) (string, error) {
	h := sha256.New()
	var files []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if rel != "." && skipInSkill(rel, d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() {
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	for _, rel := range files {
		info, err := os.Stat(filepath.Join(dir, rel))
		if err != nil {
			return "", err
		}
		f, err := os.Open(filepath.Join(dir, rel))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%o\x00", filepath.ToSlash(rel), info.Mode().Perm()&0o111)
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return "", err
		}
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type skillState struct {
	Installed string `json:"installed"` // hash of the snapshot last installed by the app
}

// syncBundledSkill installs bundle into target when needed and reports what it did.
func syncBundledSkill(bundle, target, stateFile, backups string) (string, error) {
	want, err := skillHash(bundle)
	if err != nil {
		return "", fmt.Errorf("skill trong app hỏng: %w", err)
	}
	var st skillState
	if b, err := os.ReadFile(stateFile); err == nil {
		_ = json.Unmarshal(b, &st)
	}
	record := func() error {
		if err := os.MkdirAll(filepath.Dir(stateFile), 0o755); err != nil {
			return err
		}
		return writeJSONAtomic(stateFile, skillState{Installed: want}, 0o644)
	}
	if !exists(target) {
		if err := replaceDir(bundle, target); err != nil {
			return "", err
		}
		return "đã cài skill", record()
	}
	have, err := skillHash(target)
	if err != nil {
		return "", err
	}
	switch {
	case have == want:
		return "skill đã mới nhất", record()
	case st.Installed == want:
		return "giữ các sửa đổi skill trên máy", nil // this snapshot was installed before; the user edited since
	}
	note := "đã cập nhật skill"
	if have != st.Installed { // edited locally (or installed by hand): keep a copy
		dst := filepath.Join(backups, time.Now().Format("060102-150405"))
		if err := copyDir(target, dst); err != nil {
			return "", fmt.Errorf("không sao lưu được skill: %w", err)
		}
		note += ", bản cũ sao lưu ở " + dst
	}
	if err := replaceDir(bundle, target); err != nil {
		return "", err
	}
	return note, record()
}

// replaceDir puts a copy of src at dst, swapping it in so dst is never half written.
func replaceDir(src, dst string) error {
	tmp := dst + ".new"
	_ = os.RemoveAll(tmp)
	if err := copyDir(src, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	old := dst + ".old"
	_ = os.RemoveAll(old)
	if exists(dst) {
		if err := os.Rename(dst, old); err != nil {
			return err
		}
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Rename(old, dst)
		return err
	}
	return os.RemoveAll(old)
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if rel != "." && skipInSkill(rel, d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		out := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			return os.MkdirAll(out, 0o755)
		case d.Type().IsRegular():
			in, err := os.Open(p)
			if err != nil {
				return err
			}
			defer in.Close()
			f, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, in); err != nil {
				f.Close()
				return err
			}
			return f.Close()
		}
		return nil // symlinks and devices are not part of a skill
	})
}
