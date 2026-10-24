package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed web
var webFS embed.FS

// webBuild fingerprints the embedded UI so an open tab can tell it is running old code.
func webBuild() string {
	h := sha256.New()
	_ = fs.WalkDir(webFS, "web", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			b, _ := webFS.ReadFile(p)
			h.Write([]byte(p))
			h.Write(b)
		}
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))[:12]
}

type App struct {
	cfg      *ConfigStore
	runs     *Manager
	projects Projects
	secrets  SecretStore
	token    string
	home     string
	version  string
	build    string
	started  time.Time
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	static, _ := fs.Sub(webFS, "web")
	files := http.FileServer(http.FS(static))
	mux.HandleFunc("GET /{$}", a.index)
	for _, dir := range []string{"/js/", "/lib/", "/css/", "/img/"} {
		mux.Handle("GET "+dir, noCache(files))
	}
	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("GET /api/bootstrap", a.bootstrap)
	mux.HandleFunc("GET /api/system", a.system)
	mux.HandleFunc("PUT /api/settings", a.putSettings)
	mux.HandleFunc("GET /api/doctor", a.doctorHandler)
	mux.HandleFunc("GET /api/diagnostics", a.diagnostics)
	mux.HandleFunc("GET /api/secrets", a.listSecrets)
	mux.HandleFunc("PUT /api/secrets/{name}", a.putSecret)
	mux.HandleFunc("DELETE /api/secrets/{name}", a.deleteSecret)
	mux.HandleFunc("POST /api/secrets/{name}/check", a.checkSecret)
	mux.HandleFunc("POST /api/secrets/import-legacy", a.importLegacySecret)
	mux.HandleFunc("POST /api/shutdown", a.shutdown)

	mux.HandleFunc("GET /api/projects", a.listProjects)
	mux.HandleFunc("POST /api/projects", a.createProject)
	mux.HandleFunc("GET /api/projects/{slug}", a.getProject)
	mux.HandleFunc("PATCH /api/projects/{slug}/meta", a.patchMeta)
	mux.HandleFunc("DELETE /api/projects/{slug}", a.deleteProject)
	mux.HandleFunc("DELETE /api/projects/{slug}/video", a.deleteVideo)
	mux.HandleFunc("PUT /api/projects/{slug}/files", a.putProjectFile)
	mux.HandleFunc("POST /api/projects/{slug}/audio", a.importAudio)
	mux.HandleFunc("POST /api/projects/{slug}/refs", a.addRef)
	mux.HandleFunc("DELETE /api/projects/{slug}/refs/{name}", a.removeRef)
	mux.HandleFunc("GET /api/projects/{slug}/lucbat", a.lucbat)
	mux.HandleFunc("POST /api/projects/{slug}/actions/{action}", a.projectAction)
	mux.HandleFunc("POST /api/projects/{slug}/pipeline/{name}", a.projectPipeline)
	mux.HandleFunc("GET /api/groups/{group}", a.runGroup)
	mux.HandleFunc("GET /api/skill/profiles", a.profiles)
	mux.HandleFunc("POST /api/projects/{slug}/open", a.openInFinder)
	mux.HandleFunc("GET /media/{slug}/{path...}", a.media)
	mux.HandleFunc("GET /api/downloads/audio", a.recentDownloads)

	mux.HandleFunc("GET /api/runs", a.listRuns)
	mux.HandleFunc("POST /api/runs", a.createRun)
	mux.HandleFunc("GET /api/runs/{id}", a.getRun)
	mux.HandleFunc("GET /api/runs/{id}/events", a.runEvents)
	mux.HandleFunc("POST /api/runs/{id}/stop", a.stopRun)

	mux.HandleFunc("GET /api/skill/tree", a.skillTree)
	mux.HandleFunc("GET /api/skill/file", a.getSkillFile)
	mux.HandleFunc("PUT /api/skill/file", a.putSkillFile)
	mux.HandleFunc("POST /api/skill/validate", a.validateSkill)
	return guard(a.cfg.Get().Addr, a.token, logRequests(mux))
}

func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		h.ServeHTTP(w, r)
	})
}

func logRequests(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				fmt.Fprintf(os.Stderr, "panic %s %s: %v\n", r.Method, r.URL.Path, v)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		h.ServeHTTP(w, r)
	})
}

// ---------- helpers ----------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func failFor(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, os.ErrNotExist):
		fail(w, http.StatusNotFound, errors.New("không tìm thấy"))
	case errors.Is(err, errOutside):
		fail(w, http.StatusForbidden, err)
	case errors.Is(err, errConflict):
		fail(w, http.StatusConflict, err)
	default:
		fail(w, http.StatusBadRequest, err)
	}
}

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 4<<20))
	return dec.Decode(v)
}

// ---------- pages & system ----------

func (a *App) index(w http.ResponseWriter, r *http.Request) {
	b, err := webFS.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "missing index", http.StatusInternalServerError)
		return
	}
	b = bytes.ReplaceAll(b, []byte("{{TOKEN}}"), []byte(a.token))
	b = bytes.ReplaceAll(b, []byte("{{VERSION}}"), []byte(a.version))
	b = bytes.ReplaceAll(b, []byte("{{BUILD}}"), []byte(a.build))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; media-src 'self' blob:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'")
	_, _ = w.Write(b)
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"app": "que-huong-studio", "version": a.version, "pid": os.Getpid()})
}

func (a *App) bootstrap(w http.ResponseWriter, r *http.Request) {
	cfg := a.cfg.Get()
	writeJSON(w, 200, map[string]any{
		"version": a.version, "config": cfg, "agents": agentInfos(cfg), "actions": workflowActions, "profiles": skillProfiles(cfg.SkillDir),
		"skillExists": exists(filepath.Join(cfg.SkillDir, "SKILL.md")), "genres": genres, "languages": languages,
	})
}

var (
	procMu    sync.Mutex
	procCache map[string]int
	procAt    time.Time
)

func agentProcessCounts() map[string]int {
	procMu.Lock()
	defer procMu.Unlock()
	if time.Since(procAt) < 3*time.Second && procCache != nil {
		return procCache
	}
	out := map[string]int{"claude": 0, "omp": 0, "codex": 0}
	if b, err := exec.Command("ps", "-axo", "comm=").Output(); err == nil {
		for _, l := range strings.Split(string(b), "\n") {
			name := filepath.Base(strings.TrimSpace(l))
			if _, ok := out[name]; ok {
				out[name]++
			}
		}
	}
	procCache, procAt = out, time.Now()
	return out
}

func loadAverage() []float64 {
	b, err := exec.Command("sysctl", "-n", "vm.loadavg").Output()
	if err != nil {
		return nil
	}
	out := []float64{}
	for _, f := range strings.Fields(strings.Trim(strings.TrimSpace(string(b)), "{}")) {
		if v, err := strconv.ParseFloat(f, 64); err == nil {
			out = append(out, v)
		}
	}
	return out
}

func (a *App) system(w http.ResponseWriter, r *http.Request) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	running, queued := 0, 0
	for _, m := range a.runs.List(200) {
		switch m.Status {
		case "running":
			running++
		case "queued":
			queued++
		}
	}
	writeJSON(w, 200, map[string]any{
		"load": loadAverage(), "cpus": runtime.NumCPU(), "running": running, "queued": queued,
		"agentProcesses": agentProcessCounts(), "serverMemMB": float64(ms.Sys) / (1 << 20),
		"uptimeSec": int(time.Since(a.started).Seconds()), "build": a.build,
	})
}

// shutdown stops the studio from the UI; running jobs are stopped first by main's signal path.
func (a *App) shutdown(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]bool{"ok": true})
	go func() {
		time.Sleep(200 * time.Millisecond)
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(os.Interrupt)
	}()
}

func (a *App) putSettings(w http.ResponseWriter, r *http.Request) {
	var next Config
	if err := decode(r, &next); err != nil {
		failFor(w, err)
		return
	}
	next.Addr = a.cfg.Get().Addr // the listen address only changes through the config file
	cfg, err := a.cfg.Update(next, a.home)
	if err != nil {
		failFor(w, err)
		return
	}
	a.runs.poke()
	writeJSON(w, 200, map[string]any{"config": cfg, "agents": agentInfos(cfg)})
}

// ---------- projects ----------

func (a *App) listProjects(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, a.projects.List())
}

func (a *App) createProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title    string      `json:"title"`
		Subtitle string      `json:"subtitle"`
		Lyrics   string      `json:"lyrics"`
		Credits  [][2]string `json:"credits"`
		Genre    string      `json:"genre"`
		Language string      `json:"language"`
	}
	if err := decode(r, &req); err != nil {
		failFor(w, err)
		return
	}
	slug, err := a.projects.Create(req.Title, req.Subtitle, req.Lyrics, req.Credits, req.Genre, req.Language)
	if err != nil {
		failFor(w, err)
		return
	}
	writeJSON(w, 201, map[string]string{"slug": slug})
}

func (a *App) detail(slug string) (ProjectDetail, error) {
	return a.projects.Detail(slug, a.runs.ActiveFor(slug))
}

func (a *App) getProject(w http.ResponseWriter, r *http.Request) {
	d, err := a.detail(r.PathValue("slug"))
	if err != nil {
		failFor(w, err)
		return
	}
	writeJSON(w, 200, d)
}

func (a *App) putProjectFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		SHA     string `json:"sha"`
	}
	if err := decode(r, &req); err != nil {
		failFor(w, err)
		return
	}
	if err := a.projects.WriteFileIf(r.PathValue("slug"), req.Path, req.Content, req.SHA); err != nil {
		failFor(w, err)
		return
	}
	a.getProject(w, r)
}

func (a *App) importAudio(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	dst, ok := a.uploadOrPath(w, r, func(name string, src io.Reader) (string, error) { return a.projects.ImportAudio(slug, name, src) })
	if !ok {
		return
	}
	d, _ := a.detail(slug)
	writeJSON(w, 200, map[string]any{"audio": dst, "project": d})
}

func expandHome(p, home string) string {
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}

// uploadOrPath reads a multipart "file" or a JSON {path} body and hands the stream to save.
func (a *App) uploadOrPath(w http.ResponseWriter, r *http.Request, save func(name string, src io.Reader) (string, error)) (string, bool) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		r.Body = http.MaxBytesReader(w, r.Body, 2<<30)
		file, hdr, err := r.FormFile("file")
		if err != nil {
			failFor(w, err)
			return "", false
		}
		defer file.Close()
		dst, err := save(hdr.Filename, file)
		if err != nil {
			failFor(w, err)
			return "", false
		}
		return dst, true
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := decode(r, &req); err != nil {
		failFor(w, err)
		return "", false
	}
	src := expandHome(strings.TrimSpace(req.Path), a.home)
	f, err := os.Open(src)
	if err != nil {
		failFor(w, err)
		return "", false
	}
	defer f.Close()
	dst, err := save(src, f)
	if err != nil {
		failFor(w, err)
		return "", false
	}
	return dst, true
}

func (a *App) addRef(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	dst, ok := a.uploadOrPath(w, r, func(name string, src io.Reader) (string, error) { return a.projects.AddRef(slug, name, src) })
	if !ok {
		return
	}
	d, _ := a.detail(slug)
	writeJSON(w, 200, map[string]any{"ref": dst, "project": d})
}

func (a *App) removeRef(w http.ResponseWriter, r *http.Request) {
	if err := a.projects.RemoveRef(r.PathValue("slug"), r.PathValue("name")); err != nil {
		failFor(w, err)
		return
	}
	d, _ := a.detail(r.PathValue("slug"))
	writeJSON(w, 200, d)
}

var errBusy = errors.New("bài đang có việc chạy, hãy dừng hoặc chờ xong rồi xoá")

// deleteProject and deleteVideo move files to the Trash rather than erasing them.
func (a *App) deleteProject(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if a.runs.Busy(slug) {
		fail(w, http.StatusConflict, errBusy)
		return
	}
	dst, err := a.projects.TrashProject(slug, a.home)
	if err != nil {
		failFor(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"trashed": dst})
}

func (a *App) deleteVideo(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if a.runs.Busy(slug) {
		fail(w, http.StatusConflict, errBusy)
		return
	}
	if _, err := a.projects.TrashVideo(slug, r.URL.Query().Get("path"), a.home); err != nil {
		failFor(w, err)
		return
	}
	d, _ := a.detail(slug)
	writeJSON(w, 200, d)
}

// recentDownloads suggests the newest audio files in ~/Downloads, where Suno saves them.
// ?kind=ref also lists video files, since reference songs often come from YouTube downloads.
func (a *App) recentDownloads(w http.ResponseWriter, r *http.Request) {
	dir := filepath.Join(a.home, "Downloads")
	entries, _ := os.ReadDir(dir)
	type file struct {
		Name     string    `json:"name"`
		Path     string    `json:"path"`
		Size     int64     `json:"size"`
		Modified time.Time `json:"modified"`
	}
	out := []file{}
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		allowed := []string{".m4a", ".mp3", ".wav"}
		if r.URL.Query().Get("kind") == "ref" {
			allowed = refExts
		}
		if e.IsDir() || !contains(allowed, ext) {
			continue
		}
		if info, err := e.Info(); err == nil {
			out = append(out, file{e.Name(), filepath.Join(dir, e.Name()), info.Size(), info.ModTime()})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Modified.After(out[j].Modified) })
	if len(out) > 8 {
		out = out[:8]
	}
	writeJSON(w, 200, out)
}

func (a *App) lucbat(w http.ResponseWriter, r *http.Request) {
	dir, err := a.projects.dir(r.PathValue("slug"))
	if err != nil {
		failFor(w, err)
		return
	}
	skill := a.cfg.Get().SkillDir
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", "-I", filepath.Join(skill, "scripts", "check-luc-bat.py"), filepath.Join(dir, "lyrics.txt"), "--json")
	out, err := cmd.Output()
	if err != nil {
		failFor(w, fmt.Errorf("check-luc-bat: %w", err))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(out)
}

func (a *App) projectAction(w http.ResponseWriter, r *http.Request) {
	slug, action := r.PathValue("slug"), r.PathValue("action")
	var req ActionRequest
	if r.ContentLength != 0 {
		if err := decode(r, &req); err != nil {
			failFor(w, err)
			return
		}
	}
	d, err := a.detail(slug)
	if err != nil {
		failFor(w, err)
		return
	}
	act := findAction(action)
	if act == nil {
		fail(w, 404, fmt.Errorf("unknown action %q", action))
		return
	}
	cfg := a.cfg.Get()
	spec := RunSpec{Kind: act.Kind, Step: act.Step, Project: slug, Cwd: cfg.SkillDir, Heavy: act.Heavy, Secrets: act.Secrets}
	if act.Kind == "agent" {
		agent := req.Agent
		if agent == "" {
			agent = cfg.DefaultAgent
		}
		prompt, err := buildActionPrompt(action, cfg.SkillDir, d.Dir, req, d)
		if err != nil {
			failFor(w, err)
			return
		}
		spec.Cwd = d.Dir
		if action == "ref.save.agent" {
			spec.Cwd = cfg.SkillDir
		}
		argv, err := agentArgv(cfg, agent, req.Model, prompt, spec.Cwd)
		if err != nil {
			failFor(w, err)
			return
		}
		spec.Agent, spec.Model, spec.Prompt, spec.Argv, spec.Title = agent, req.Model, prompt, argv, act.Label
	} else {
		argv, title, err := scriptArgv(action, cfg.SkillDir, d.Dir, req, d)
		if err != nil {
			failFor(w, err)
			return
		}
		spec.Argv, spec.Title = argv, title
		if action == "stills" || action == "render" {
			spec.FingerprintDir = d.Dir
		}
	}
	meta, err := a.runs.Submit(spec)
	if err != nil {
		failFor(w, err)
		return
	}
	writeJSON(w, 202, meta)
}

// projectPipeline queues a chain of runs; independent links start in parallel.
func (a *App) projectPipeline(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var req ActionRequest
	if r.ContentLength != 0 {
		if err := decode(r, &req); err != nil {
			failFor(w, err)
			return
		}
	}
	d, err := a.detail(slug)
	if err != nil {
		failFor(w, err)
		return
	}
	specs, deps, err := pipelineSpecs(r.PathValue("name"), a.cfg.Get(), d, req)
	if err != nil {
		failFor(w, err)
		return
	}
	active := a.runs.ActiveFor(slug)
	for _, s := range specs {
		if id, busy := active[s.Step]; busy {
			fail(w, http.StatusConflict, fmt.Errorf("bài này đang có việc chạy ở bước %q (%s); đợi xong hoặc dừng nó trước", s.Step, id))
			return
		}
	}
	metas, err := a.runs.SubmitChain(specs, deps)
	if err != nil {
		failFor(w, err)
		return
	}
	writeJSON(w, 202, metas)
}

func (a *App) runGroup(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, a.runs.Group(r.PathValue("group")))
}

func (a *App) profiles(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, skillProfiles(a.cfg.Get().SkillDir))
}

func (a *App) openInFinder(w http.ResponseWriter, r *http.Request) {
	dir, err := a.projects.dir(r.PathValue("slug"))
	if err != nil {
		failFor(w, err)
		return
	}
	var req struct {
		Path   string `json:"path"`
		Reveal bool   `json:"reveal"`
		App    string `json:"app"` // "musescore" opens a score for editing
	}
	if err := decode(r, &req); err != nil {
		failFor(w, err)
		return
	}
	target := dir
	if req.Path != "" {
		if target, err = safeJoin(dir, req.Path); err != nil {
			failFor(w, err)
			return
		}
	}
	args := []string{target}
	if req.Reveal {
		args = []string{"-R", target}
	}
	if req.App == "musescore" {
		args = []string{"-a", "MuseScore 4", target}
	}
	if err := exec.Command("open", args...).Run(); err != nil {
		failFor(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *App) media(w http.ResponseWriter, r *http.Request) {
	dir, err := a.projects.dir(r.PathValue("slug"))
	if err != nil {
		failFor(w, err)
		return
	}
	full, err := safeJoin(dir, r.PathValue("path"))
	if err != nil {
		failFor(w, err)
		return
	}
	f, err := os.Open(full)
	if err != nil {
		failFor(w, err)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		failFor(w, os.ErrNotExist)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, st.Name(), st.ModTime(), f) // handles Range for video seeking
}

// ---------- runs ----------

func (a *App) listRuns(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	project := r.URL.Query().Get("project")
	out := []RunMeta{}
	for _, m := range a.runs.List(500) {
		if project == "" || m.Project == project {
			m.Prompt = "" // list stays light; the detail endpoint has it
			out = append(out, m)
		}
		if len(out) >= limit {
			break
		}
	}
	writeJSON(w, 200, out)
}

func (a *App) createRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Agent   string `json:"agent"`
		Model   string `json:"model"`
		Prompt  string `json:"prompt"`
		Target  string `json:"target"` // "skill" or "project"
		Project string `json:"project"`
		Title   string `json:"title"`
	}
	if err := decode(r, &req); err != nil {
		failFor(w, err)
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		fail(w, 400, errors.New("cần nội dung yêu cầu"))
		return
	}
	cfg := a.cfg.Get()
	if req.Agent == "" {
		req.Agent = cfg.DefaultAgent
	}
	spec := RunSpec{Kind: "agent", Agent: req.Agent, Model: req.Model, Title: req.Title}
	switch req.Target {
	case "skill":
		spec.Cwd, spec.Step = cfg.SkillDir, "skill"
		spec.Prompt = fmt.Sprintf(skillPromptHeader, cfg.SkillDir) + strings.TrimSpace(req.Prompt)
		if spec.Title == "" {
			spec.Title = "Sửa skill: " + firstLine(req.Prompt, 60)
		}
	case "project":
		dir, err := a.projects.dir(req.Project)
		if err != nil {
			failFor(w, err)
			return
		}
		spec.Cwd, spec.Project = dir, req.Project
		spec.Prompt = fmt.Sprintf(promptHeader, cfg.SkillDir, dir) + "\nViệc: " + strings.TrimSpace(req.Prompt)
		if spec.Title == "" {
			spec.Title = firstLine(req.Prompt, 60)
		}
	default:
		fail(w, 400, errors.New("target must be skill or project"))
		return
	}
	argv, err := agentArgv(cfg, spec.Agent, spec.Model, spec.Prompt, spec.Cwd)
	if err != nil {
		failFor(w, err)
		return
	}
	spec.Argv = argv
	meta, err := a.runs.Submit(spec)
	if err != nil {
		failFor(w, err)
		return
	}
	writeJSON(w, 202, meta)
}

func firstLine(s string, n int) string {
	s = strings.TrimSpace(strings.SplitN(strings.TrimSpace(s), "\n", 2)[0])
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

func (a *App) getRun(w http.ResponseWriter, r *http.Request) {
	run, ok := a.runs.Get(r.PathValue("id"))
	if !ok {
		failFor(w, os.ErrNotExist)
		return
	}
	writeJSON(w, 200, run.snapshot())
}

func (a *App) stopRun(w http.ResponseWriter, r *http.Request) {
	if err := a.runs.Stop(r.PathValue("id")); err != nil {
		failFor(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// runEvents streams a run as Server-Sent Events: backlog first, then live events,
// then an "end" event carrying the final metadata.
func (a *App) runEvents(w http.ResponseWriter, r *http.Request) {
	run, ok := a.runs.Get(r.PathValue("id"))
	if !ok {
		failFor(w, os.ErrNotExist)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		fail(w, 500, errors.New("streaming unsupported"))
		return
	}
	after, _ := strconv.Atoi(r.URL.Query().Get("after"))
	if v := r.Header.Get("Last-Event-ID"); v != "" {
		after, _ = strconv.Atoi(v)
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	send := func(name string, id int, v any) {
		b, _ := json.Marshal(v)
		if id > 0 {
			fmt.Fprintf(w, "id: %d\n", id)
		}
		if name != "" {
			fmt.Fprintf(w, "event: %s\n", name)
		}
		fmt.Fprintf(w, "data: %s\n\n", b)
	}
	backlog, ch, cancel := a.runs.Subscribe(run, after)
	defer cancel()
	send("meta", 0, run.snapshot())
	for _, e := range backlog {
		send("", e.Seq, e)
	}
	flusher.Flush()
	if ch == nil {
		send("end", 0, run.snapshot())
		flusher.Flush()
		return
	}
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case e, ok := <-ch:
			if !ok {
				if meta := run.snapshot(); !meta.active() {
					send("end", 0, meta)
					flusher.Flush()
				} // else: dropped for falling behind; the browser reconnects with Last-Event-ID
				return
			}
			send("", e.Seq, e)
			if e.Kind == "status" {
				send("meta", 0, run.snapshot())
			}
			flusher.Flush()
		case <-tick.C:
			fmt.Fprint(w, ": ping\n\n")
			send("meta", 0, run.snapshot())
			flusher.Flush()
		}
	}
}

// ---------- skill ----------

func (a *App) skillTree(w http.ResponseWriter, r *http.Request) {
	root := a.cfg.Get().SkillDir
	tree, err := skillTree(root)
	if err != nil {
		failFor(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"root": root, "files": tree})
}

func (a *App) getSkillFile(w http.ResponseWriter, r *http.Request) {
	f, err := readSkillFile(a.cfg.Get().SkillDir, r.URL.Query().Get("path"))
	if err != nil {
		failFor(w, err)
		return
	}
	writeJSON(w, 200, f)
}

func (a *App) putSkillFile(w http.ResponseWriter, r *http.Request) {
	var req SkillFile
	if err := decode(r, &req); err != nil {
		failFor(w, err)
		return
	}
	f, err := writeSkillFile(a.cfg.Get().SkillDir, req.Path, req.Content, req.SHA)
	if err != nil {
		failFor(w, err)
		return
	}
	writeJSON(w, 200, f)
}

func (a *App) validateSkill(w http.ResponseWriter, r *http.Request) {
	cfg := a.cfg.Get()
	creator := filepath.Join(filepath.Dir(cfg.SkillDir), "ak-skill-creator")
	script := `set -o pipefail
echo "== Unit tests"; for t in scripts/tests/test-*.py; do [ -f "$t" ] && python3 -I "$t" 2>&1 | tail -3; done
if [ -d "$1" ]; then
  echo "== quick_validate"; (cd "$1" && uv run --quiet scripts/quick_validate.py "$2")
  echo "== lint_cruft --routing"; (cd "$1" && uv run --quiet --with PyYAML==6.0.3 scripts/lint_cruft.py "$2" --routing)
else
  echo "ak-skill-creator not installed: structural checks skipped"
fi`
	meta, err := a.runs.Submit(RunSpec{Kind: "script", Step: "skill", Title: "Kiểm tra skill", Cwd: cfg.SkillDir,
		Argv: []string{"/bin/bash", "-c", script, "validate", creator, cfg.SkillDir}})
	if err != nil {
		failFor(w, err)
		return
	}
	writeJSON(w, 202, meta)
}

// ---------- secrets ----------

func (a *App) secretEnv(names []string) []string {
	if slices.Contains(names, "image") {
		return imageEnv(a.cfg.Get(), a.secrets)
	}
	return nil
}

func secretName(r *http.Request) (string, error) {
	name := r.PathValue("name")
	if !slices.Contains(secretNames, name) {
		return "", errors.New("tên khoá không hợp lệ")
	}
	return name, nil
}

func (a *App) listSecrets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"secrets": secretInfos(a.secrets), "legacy": exists(legacyHubKey(a.home))})
}

func (a *App) putSecret(w http.ResponseWriter, r *http.Request) {
	name, err := secretName(r)
	if err != nil {
		failFor(w, err)
		return
	}
	var req struct {
		Value string `json:"value"`
	}
	if err := decode(r, &req); err != nil {
		failFor(w, err)
		return
	}
	v := strings.TrimSpace(req.Value)
	if len(v) < 8 || len(v) > 512 || strings.ContainsAny(v, " \t\r\n") {
		failFor(w, errors.New("khoá trông không đúng: dán đúng một dòng khoá"))
		return
	}
	if err := a.secrets.Set(name, v); err != nil {
		failFor(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"secrets": secretInfos(a.secrets)})
}

func (a *App) deleteSecret(w http.ResponseWriter, r *http.Request) {
	name, err := secretName(r)
	if err != nil {
		failFor(w, err)
		return
	}
	_ = a.secrets.Delete(name)
	writeJSON(w, 200, map[string]any{"secrets": secretInfos(a.secrets)})
}

func (a *App) checkSecret(w http.ResponseWriter, r *http.Request) {
	name, err := secretName(r)
	if err != nil {
		failFor(w, err)
		return
	}
	key, err := a.secrets.Get(name)
	if err != nil {
		failFor(w, err)
		return
	}
	if err := checkImageKey(a.cfg.Get(), strings.TrimPrefix(name, "image."), key, nil); err != nil {
		writeJSON(w, 200, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// importLegacySecret copies the old key file into the keychain; the file stays for scripts run outside the studio.
func (a *App) importLegacySecret(w http.ResponseWriter, r *http.Request) {
	v := readLegacyHubKey(a.home)
	if v == "" {
		failFor(w, os.ErrNotExist)
		return
	}
	if err := a.secrets.Set("image.hub", v); err != nil {
		failFor(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"secrets": secretInfos(a.secrets)})
}

func (a *App) patchMeta(w http.ResponseWriter, r *http.Request) {
	var m SongMeta
	if err := decode(r, &m); err != nil {
		failFor(w, err)
		return
	}
	slug := r.PathValue("slug")
	if err := a.projects.SetMeta(slug, m); err != nil {
		failFor(w, err)
		return
	}
	d, _ := a.detail(slug)
	writeJSON(w, 200, d)
}
