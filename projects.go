package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// A project is a folder under ProjectsDir holding project.json and the song's files,
// laid out as the suno-que-huong skill expects (see its references/karaoke-video.md).

type Step struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"` // todo, done, stale, skip, running
	Detail string `json:"detail"`
	RunID  string `json:"runId,omitempty"`
}

type ProjectSummary struct {
	Slug     string    `json:"slug"`
	Title    string    `json:"title"`
	Subtitle string    `json:"subtitle"`
	Cover    string    `json:"cover,omitempty"`
	Updated  time.Time `json:"updated"`
	Done     int       `json:"done"`
	Total    int       `json:"total"`
	Video    string    `json:"video,omitempty"`
}

// RefFile is a reference song attached for style analysis (never uploaded anywhere).
type RefFile struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Analysis string `json:"analysis,omitempty"`
}

var refExts = []string{".m4a", ".mp3", ".wav", ".flac", ".ogg", ".opus", ".aac", ".mp4", ".webm", ".mov", ".mkv"}

type TimingLine struct {
	Text   string  `json:"text"`
	Start  float64 `json:"start"`
	End    float64 `json:"end"`
	Drift  float64 `json:"drift"`
	Source string  `json:"source"`
}

type ProjectDetail struct {
	ProjectSummary
	Dir        string          `json:"dir"`
	Config     json.RawMessage `json:"config"`
	Lyrics     string          `json:"lyrics"`
	SunoLyrics string          `json:"sunoLyrics"`
	SunoStyle  string          `json:"sunoStyle"`
	Notes      string          `json:"notes"`
	Audio      string          `json:"audio,omitempty"`
	Duration   float64         `json:"duration,omitempty"`
	Timings    []TimingLine    `json:"timings"`
	Images     []string        `json:"images"`
	Stills     []string        `json:"stills"`
	Videos     []string        `json:"videos"`
	ConfigSHA  string          `json:"configSha"`
	Refs       []RefFile       `json:"refs"`
	Brief      string          `json:"brief"`
	Profile    string          `json:"profile"` // suno/profile.md written from the references and brief
	Steps      []Step          `json:"steps"`
}

type projectConfig struct {
	Audio    string     `json:"audio"`
	Title    string     `json:"title"`
	Subtitle string     `json:"subtitle"`
	IntroCut []float64  `json:"intro_cut"`
	Scenes   []sceneCfg `json:"scenes"`
	Profile  string     `json:"profile"`
	Vocal    string     `json:"vocal"`
}

type sceneCfg struct {
	ID     string  `json:"id"`
	Start  float64 `json:"start"`
	Reuse  bool    `json:"reuse"`
	Prompt string  `json:"prompt"`
}

type Projects struct{ cfg *ConfigStore }

func (p Projects) root() string { return p.cfg.Get().ProjectsDir }

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// slugify turns a Vietnamese title into an ASCII folder name.
func slugify(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "đ", "d"), "Đ", "D")
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	out := strings.Trim(slugRe.ReplaceAllString(b.String(), "-"), "-")
	if out == "" {
		out = "bai-hat"
	}
	return out
}

func (p Projects) dir(slug string) (string, error) {
	if slug == "" || slug != filepath.Base(slug) || strings.HasPrefix(slug, ".") {
		return "", errOutside
	}
	d, err := safeJoin(p.root(), slug)
	if err != nil {
		return "", err
	}
	if st, err := os.Stat(d); err != nil || !st.IsDir() {
		return "", os.ErrNotExist
	}
	return d, nil
}

func (p Projects) List() []ProjectSummary {
	entries, _ := os.ReadDir(p.root())
	out := []ProjectSummary{}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		d := filepath.Join(p.root(), e.Name())
		if !exists(filepath.Join(d, "project.json")) && !exists(filepath.Join(d, "lyrics.txt")) {
			continue
		}
		det, err := p.Detail(e.Name(), nil)
		if err == nil {
			out = append(out, det.ProjectSummary)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Updated.After(out[j].Updated) })
	return out
}

func readConfig(dir string) (projectConfig, json.RawMessage) {
	raw, err := os.ReadFile(filepath.Join(dir, "project.json"))
	var pc projectConfig
	if err != nil {
		return pc, json.RawMessage("{}")
	}
	_ = json.Unmarshal(raw, &pc)
	return pc, raw
}

func (p Projects) Detail(slug string, active map[string]string) (ProjectDetail, error) {
	dir, err := p.dir(slug)
	if err != nil {
		return ProjectDetail{}, err
	}
	pc, raw := readConfig(dir)
	d := ProjectDetail{Dir: dir, Config: raw, ConfigSHA: hashOf(raw)}
	d.Slug = slug
	d.Title, d.Subtitle = pc.Title, pc.Subtitle
	if d.Title == "" {
		d.Title = slug
	}
	d.Lyrics = readText(filepath.Join(dir, "lyrics.txt"))
	d.SunoLyrics = readText(filepath.Join(dir, "suno", "lyrics.txt"))
	d.SunoStyle = readText(filepath.Join(dir, "suno", "style.txt"))
	d.Notes = readText(filepath.Join(dir, "suno", "notes.md"))
	d.Brief = readText(filepath.Join(dir, "brief.md"))
	d.Profile = readText(filepath.Join(dir, "suno", "profile.md"))
	d.Refs = listRefs(dir)
	if pc.Audio != "" && exists(pc.Audio) {
		if rel, err := filepath.Rel(dir, pc.Audio); err == nil && !strings.HasPrefix(rel, "..") {
			d.Audio = filepath.ToSlash(rel)
		}
	}
	d.Images = listRel(dir, "img", ".png", ".jpg", ".jpeg", ".webp")
	d.Stills = listRel(dir, "work", ".jpg")
	d.Stills = filterPrefix(d.Stills, "work/still-")
	d.Videos = karaokeVideos(dir, pc.Audio)
	d.Timings = readTimings(filepath.Join(dir, "work", "timings.json"))
	d.Updated = newestMtime(dir)
	for _, img := range d.Images {
		if strings.HasPrefix(img, "img/01") {
			d.Cover = img
			break
		}
	}
	if d.Cover == "" && len(d.Images) > 0 {
		d.Cover = d.Images[0]
	}
	if len(d.Videos) > 0 {
		d.Video = d.Videos[len(d.Videos)-1]
	}
	d.Steps = deriveSteps(dir, pc, d, active)
	for _, s := range d.Steps {
		if s.Status == "done" || s.Status == "skip" {
			d.Done++
		}
	}
	d.Total = len(d.Steps)
	return d, nil
}

func deriveSteps(dir string, pc projectConfig, d ProjectDetail, active map[string]string) []Step {
	mt := func(rel string) time.Time { return mtime(filepath.Join(dir, rel)) }
	steps := []Step{
		{ID: "lyrics", Title: "Lời, bài mẫu & yêu cầu"},
		{ID: "suno", Title: "Prompt Suno"},
		{ID: "audio", Title: "Nhạc từ Suno"},
		{ID: "align", Title: "Canh lời từng chữ"},
		{ID: "trim", Title: "Rút ngắn intro"},
		{ID: "scenes", Title: "Cảnh & ảnh AI"},
		{ID: "stills", Title: "Xem trước khung hình"},
		{ID: "render", Title: "Xuất video karaoke"},
	}
	set := func(id, status, detail string) {
		for i := range steps {
			if steps[i].ID == id {
				steps[i].Status, steps[i].Detail = status, detail
			}
		}
	}
	lines := strings.Count(strings.TrimSpace(d.Lyrics), "\n") + 1
	if strings.TrimSpace(d.Lyrics) == "" {
		set("lyrics", "todo", "Chưa có lời")
	} else {
		parts := []string{fmt.Sprintf("%d dòng", lines)}
		if len(d.Refs) > 0 {
			parts = append(parts, fmt.Sprintf("%d bài mẫu", len(d.Refs)))
		}
		if strings.TrimSpace(d.Brief) != "" {
			parts = append(parts, "có yêu cầu")
		}
		set("lyrics", "done", strings.Join(parts, " · "))
	}
	// the Suno prompt is built from the lyrics, the brief and the reference analyses
	inputs := []string{"lyrics.txt", "brief.md"}
	for _, r := range d.Refs {
		inputs = append(inputs, r.Path+".analysis.txt")
	}
	switch {
	case d.SunoLyrics == "" || d.SunoStyle == "":
		set("suno", "todo", "Chưa tạo prompt")
	case newestOf(dir, inputs).After(mt("suno/style.txt")):
		set("suno", "stale", "Lời, yêu cầu hoặc bài mẫu đã đổi sau khi tạo prompt")
	default:
		set("suno", "done", fmt.Sprintf("Styles %d/1000 ký tự", len([]rune(strings.TrimSpace(d.SunoStyle)))))
	}
	if d.Audio == "" {
		set("audio", "todo", "Tải bản M4A từ Suno rồi thêm vào đây")
	} else {
		set("audio", "done", d.Audio)
	}
	switch {
	case len(d.Timings) == 0:
		set("align", "todo", "Chưa canh lời")
	case pc.Audio != "" && mtime(pc.Audio).After(mt("work/timings.json")) && len(pc.IntroCut) == 0:
		set("align", "stale", "Nhạc mới hơn timing")
	default:
		set("align", "done", fmt.Sprintf("%d câu", len(d.Timings)))
	}
	first := 0.0
	if len(d.Timings) > 0 {
		first = d.Timings[0].Start
	}
	switch {
	case len(pc.IntroCut) == 2:
		set("trim", "done", fmt.Sprintf("Đã cắt %.1fs → %.1fs, hát ở %.1fs", pc.IntroCut[0], pc.IntroCut[1], first))
	case len(d.Timings) == 0:
		set("trim", "todo", "Cần canh lời trước")
	case first <= 15:
		set("trim", "skip", fmt.Sprintf("Giọng vào ở %.1fs, không cần cắt", first))
	default:
		set("trim", "todo", fmt.Sprintf("Giọng vào ở %.1fs, nên cắt về ~11s", first))
	}
	need, have := 0, 0
	for _, s := range pc.Scenes {
		if s.Reuse {
			continue
		}
		need++
		if exists(filepath.Join(dir, "img", s.ID+".png")) {
			have++
		}
	}
	switch {
	case need == 0:
		set("scenes", "todo", "Chưa lên danh sách cảnh")
	case have < need:
		set("scenes", "todo", fmt.Sprintf("%d/%d ảnh", have, need))
	default:
		set("scenes", "done", fmt.Sprintf("%d cảnh, đủ ảnh", len(pc.Scenes)))
	}
	stamps := readStamps(dir)
	current := videoInputsHash(dir)
	outdated := func(step string, outputs []string) bool {
		if h, ok := stamps[step]; ok {
			return h != current
		}
		// made before the studio recorded fingerprints: only a newer timing invalidates it
		return newestOf(dir, outputs).Before(mt("work/timings.json"))
	}
	switch {
	case len(d.Stills) == 0:
		set("stills", "todo", "Chưa xem trước")
	case outdated("stills", d.Stills):
		set("stills", "stale", fmt.Sprintf("%d khung hình, cấu hình video đã đổi", len(d.Stills)))
	default:
		set("stills", "done", fmt.Sprintf("%d khung hình", len(d.Stills)))
	}
	switch {
	case len(d.Videos) == 0:
		set("render", "todo", "Chưa xuất")
	case outdated("render", d.Videos):
		set("render", "stale", "Cảnh, lời hoặc nhạc đã đổi sau lần xuất trước")
	default:
		set("render", "done", d.Videos[len(d.Videos)-1])
	}
	for i := range steps {
		if id, ok := active[steps[i].ID]; ok {
			steps[i].Status, steps[i].RunID = "running", id
		}
	}
	return steps
}

// Create makes a new project folder with lyrics.txt and a starter project.json.
func (p Projects) Create(title, subtitle, lyrics string, credits [][2]string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", errors.New("cần tên bài")
	}
	if err := os.MkdirAll(p.root(), 0o755); err != nil {
		return "", err
	}
	base := slugify(strings.TrimSpace(title + " " + subtitle))
	slug := base
	for i := 2; exists(filepath.Join(p.root(), slug)); i++ {
		slug = fmt.Sprintf("%s-%d", base, i)
	}
	dir := filepath.Join(p.root(), slug)
	for _, sub := range []string{"", "suno", "img", "work"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return "", err
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "lyrics.txt"), []byte(strings.TrimSpace(lyrics)+"\n"), 0o644); err != nil {
		return "", err
	}
	cr := [][]string{}
	for _, c := range credits {
		if strings.TrimSpace(c[1]) != "" {
			cr = append(cr, []string{c[0], strings.TrimSpace(c[1])})
		}
	}
	cfg := map[string]any{
		"title": title, "subtitle": strings.TrimSpace(subtitle), "credits": cr,
		"karaoke_color": "pink", "badge": false, "image_model": p.cfg.Get().ImageModel, "scenes": []any{},
	}
	return slug, writeJSONAtomic(filepath.Join(dir, "project.json"), cfg, 0o644)
}

// editable files the UI may overwrite, relative to the project dir
var editableProjectFiles = map[string]bool{"lyrics.txt": true, "suno/lyrics.txt": true, "suno/style.txt": true, "suno/notes.md": true, "project.json": true, "brief.md": true, "suno/profile.md": true}

// WriteFileIf saves like WriteFile but refuses when sha no longer matches the file on disk
// (an agent may have changed it since the page loaded). An empty sha skips the check.
func (p Projects) WriteFileIf(slug, rel, content, sha string) error {
	if sha != "" {
		dir, err := p.dir(slug)
		if err != nil {
			return err
		}
		cur, _ := os.ReadFile(filepath.Join(dir, rel))
		if hashOf(cur) != sha {
			return errConflict
		}
	}
	return p.WriteFile(slug, rel, content)
}

func (p Projects) WriteFile(slug, rel, content string) error {
	if !editableProjectFiles[rel] {
		return errors.New("file is not editable")
	}
	dir, err := p.dir(slug)
	if err != nil {
		return err
	}
	if rel == "project.json" {
		var v map[string]any
		if err := json.Unmarshal([]byte(content), &v); err != nil {
			return fmt.Errorf("project.json không hợp lệ: %w", err)
		}
		return writeJSONAtomic(filepath.Join(dir, rel), v, 0o644)
	}
	return writeFileAtomic(filepath.Join(dir, rel), []byte(content), 0o644)
}

// ImportAudio copies a local audio file into the project and points project.json at it.
func (p Projects) ImportAudio(slug, name string, src io.Reader) (string, error) {
	dir, err := p.dir(slug)
	if err != nil {
		return "", err
	}
	ext := strings.ToLower(filepath.Ext(name))
	if !contains([]string{".m4a", ".mp4", ".mp3", ".wav", ".aac", ".flac", ".ogg"}, ext) {
		return "", errors.New("chỉ nhận file âm thanh (nên dùng M4A từ Suno)")
	}
	pc, raw := readConfig(dir)
	dst := filepath.Join(dir, safeFileName(pc.Title+" "+pc.Subtitle, slug)+ext)
	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		return "", err
	}
	tmp.Close()
	if err := os.Rename(tmp.Name(), dst); err != nil {
		return "", err
	}
	_, raw = readConfig(dir)
	var cfg map[string]any
	_ = json.Unmarshal(raw, &cfg)
	if cfg == nil {
		cfg = map[string]any{}
	}
	cfg["audio"] = dst
	delete(cfg, "intro_cut") // a new download starts from the full intro again
	return dst, writeJSONAtomic(filepath.Join(dir, "project.json"), cfg, 0o644)
}

// videoInputsHash fingerprints everything the renderer reads: the video-relevant
// project.json keys, the timings, and the scene images (by name, size and mtime).
func videoInputsHash(dir string) string {
	h := sha256.New()
	var cfg map[string]json.RawMessage
	raw, _ := os.ReadFile(filepath.Join(dir, "project.json"))
	_ = json.Unmarshal(raw, &cfg)
	for _, k := range []string{"audio", "title", "subtitle", "credits", "karaoke_color", "badge", "scenes", "intro_cut"} {
		h.Write([]byte(k))
		h.Write(cfg[k])
	}
	t, _ := os.ReadFile(filepath.Join(dir, "work", "timings.json"))
	h.Write(t)
	entries, _ := os.ReadDir(filepath.Join(dir, "img"))
	for _, e := range entries {
		if info, err := e.Info(); err == nil {
			fmt.Fprintf(h, "%s:%d:%d;", e.Name(), info.Size(), info.ModTime().UnixNano())
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

const stampFile = "work/studio-stamps.json"

func readStamps(dir string) map[string]string {
	m := map[string]string{}
	if b, err := os.ReadFile(filepath.Join(dir, stampFile)); err == nil {
		_ = json.Unmarshal(b, &m)
	}
	return m
}

// RecordStamp remembers which inputs a successful stills/render run used.
func (p Projects) RecordStamp(slug, step, hash string) error {
	dir, err := p.dir(slug)
	if err != nil {
		return err
	}
	m := readStamps(dir)
	m[step] = hash
	return writeJSONAtomic(filepath.Join(dir, stampFile), m, 0o644)
}

func listRefs(dir string) []RefFile {
	entries, _ := os.ReadDir(filepath.Join(dir, "ref"))
	out := []RefFile{}
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") || !contains(refExts, strings.ToLower(filepath.Ext(e.Name()))) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		rel := "ref/" + e.Name()
		out = append(out, RefFile{Path: rel, Name: e.Name(), Size: info.Size(), Analysis: readText(filepath.Join(dir, rel+".analysis.txt"))})
	}
	return out
}

// AddRef copies a reference song into ref/. The file is only analysed locally.
func (p Projects) AddRef(slug, name string, src io.Reader) (string, error) {
	dir, err := p.dir(slug)
	if err != nil {
		return "", err
	}
	ext := strings.ToLower(filepath.Ext(name))
	if !contains(refExts, ext) {
		return "", errors.New("chỉ nhận file nhạc hoặc video (mp3, m4a, wav, mp4, webm…)")
	}
	if err := os.MkdirAll(filepath.Join(dir, "ref"), 0o755); err != nil {
		return "", err
	}
	base := safeFileName(strings.TrimSuffix(filepath.Base(name), filepath.Ext(name)), "bai-mau")
	dst := filepath.Join(dir, "ref", base+ext)
	for i := 2; exists(dst); i++ {
		dst = filepath.Join(dir, "ref", fmt.Sprintf("%s-%d%s", base, i, ext))
	}
	tmp, err := os.CreateTemp(filepath.Join(dir, "ref"), ".upload-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		return "", err
	}
	tmp.Close()
	return dst, os.Rename(tmp.Name(), dst)
}

func (p Projects) RemoveRef(slug, name string) error {
	dir, err := p.dir(slug)
	if err != nil {
		return err
	}
	if name == "" || name != filepath.Base(name) {
		return errOutside
	}
	full := filepath.Join(dir, "ref", name)
	if !exists(full) {
		return os.ErrNotExist
	}
	_ = os.Remove(full + ".analysis.txt")
	return os.Remove(full)
}

// safeFileName turns a song title into a file name that stays inside its folder.
func safeFileName(title, fallback string) string {
	s := strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', 0:
			return '-'
		}
		return r
	}, strings.TrimSpace(title))
	s = strings.TrimLeft(s, ". ")
	if s == "" {
		return fallback
	}
	return s
}

// karaokeVideos lists rendered karaoke MP4s, oldest first, ignoring an imported .mp4 audio file.
func karaokeVideos(dir, audio string) []string {
	out := []string{}
	for _, rel := range listRel(dir, "", ".mp4") {
		if strings.HasSuffix(rel, "Karaoke.mp4") && filepath.Join(dir, rel) != audio {
			out = append(out, rel)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return mtime(filepath.Join(dir, out[i])).Before(mtime(filepath.Join(dir, out[j])))
	})
	return out
}

// readTimings never returns nil: the UI treats every list in ProjectDetail as an array.
func readTimings(path string) []TimingLine {
	raw, err := os.ReadFile(path)
	if err != nil {
		return []TimingLine{}
	}
	var lines []struct {
		Text      string      `json:"text"`
		Times     [][]float64 `json:"times"`
		SunoStart float64     `json:"suno_start"`
		Source    string      `json:"source"`
	}
	if json.Unmarshal(raw, &lines) != nil {
		return []TimingLine{}
	}
	out := []TimingLine{}
	for _, l := range lines {
		if len(l.Times) == 0 {
			continue
		}
		out = append(out, TimingLine{Text: l.Text, Start: l.Times[0][0], End: l.Times[len(l.Times)-1][1], Drift: l.Times[0][0] - l.SunoStart, Source: l.Source})
	}
	return out
}

func readText(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

func mtime(path string) time.Time {
	st, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return st.ModTime()
}

func listRel(dir, sub string, exts ...string) []string {
	entries, _ := os.ReadDir(filepath.Join(dir, sub))
	out := []string{}
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if contains(exts, strings.ToLower(filepath.Ext(e.Name()))) {
			out = append(out, filepath.ToSlash(filepath.Join(sub, e.Name())))
		}
	}
	sort.Strings(out)
	return out
}

func filterPrefix(list []string, prefix string) []string {
	out := []string{}
	for _, s := range list {
		if strings.HasPrefix(s, prefix) {
			out = append(out, s)
		}
	}
	return out
}

func newestOf(dir string, rels []string) time.Time {
	var t time.Time
	for _, r := range rels {
		if m := mtime(filepath.Join(dir, r)); m.After(t) {
			t = m
		}
	}
	return t
}

func newestMtime(dir string) time.Time {
	t := mtime(filepath.Join(dir, "project.json"))
	for _, rel := range []string{"lyrics.txt", "suno/lyrics.txt", "work/timings.json"} {
		if m := mtime(filepath.Join(dir, rel)); m.After(t) {
			t = m
		}
	}
	return t
}
