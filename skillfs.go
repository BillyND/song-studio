package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type SkillEntry struct {
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	Editable bool      `json:"editable"`
}

type SkillFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	SHA     string `json:"sha"`
}

var errConflict = trErr("file đã đổi trên đĩa từ lúc mở", "file changed on disk since it was opened")

var editableExt = map[string]bool{".md": true, ".py": true, ".sh": true, ".txt": true, ".json": true, ".yaml": true, ".yml": true, ".js": true, ".toml": true}

const maxEditable = 1 << 20 // 1 MiB keeps the browser editor responsive

func skillTree(root string) ([]SkillEntry, error) {
	out := []SkillEntry{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p != root && (strings.HasPrefix(name, ".") || name == "__pycache__" || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		out = append(out, SkillEntry{Path: filepath.ToSlash(rel), Size: info.Size(), Modified: info.ModTime(),
			Editable: editableExt[strings.ToLower(filepath.Ext(name))] && info.Size() <= maxEditable})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return sortKey(out[i].Path) < sortKey(out[j].Path) })
	return out, err
}

// sortKey lists SKILL.md first, then folders in a stable order.
func sortKey(p string) string {
	if p == "SKILL.md" {
		return "0"
	}
	return "1" + p
}

func hashOf(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func readSkillFile(root, rel string) (SkillFile, error) {
	full, err := safeJoin(root, rel)
	if err != nil {
		return SkillFile{}, err
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return SkillFile{}, err
	}
	if len(b) > maxEditable || !utf8.Valid(b) {
		return SkillFile{}, errors.New(tr("file nhị phân hoặc quá lớn, không sửa ở đây được", "binary or very large file is not editable here"))
	}
	return SkillFile{Path: rel, Content: string(b), SHA: hashOf(b)}, nil
}

// writeSkillFile saves content when the caller's sha still matches the file on disk.
// An empty sha creates a new file and fails if one already exists.
func writeSkillFile(root, rel, content, sha string) (SkillFile, error) {
	if !editableExt[strings.ToLower(filepath.Ext(rel))] {
		return SkillFile{}, errors.New(tr("không sửa được loại file này", "this file type is not editable"))
	}
	if len(content) > maxEditable {
		return SkillFile{}, errors.New(tr("file quá lớn", "file too large"))
	}
	full, err := safeJoin(root, rel)
	if err != nil {
		return SkillFile{}, err
	}
	cur, err := os.ReadFile(full)
	switch {
	case err == nil && sha != hashOf(cur):
		return SkillFile{}, errConflict
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return SkillFile{}, err
	case errors.Is(err, os.ErrNotExist) && sha != "":
		return SkillFile{}, errConflict
	}
	mode := os.FileMode(0o644)
	if st, err := os.Stat(full); err == nil {
		mode = st.Mode().Perm()
	}
	if err := writeFileAtomic(full, []byte(content), mode); err != nil {
		return SkillFile{}, err
	}
	return SkillFile{Path: rel, Content: content, SHA: hashOf([]byte(content))}, nil
}
