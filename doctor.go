package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"
)

// Check is one line of the "Kiểm tra máy" page.
type Check struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Status string `json:"status"` // ok, warn, bad
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
}

const minFreeBytes = 5 << 30 // a song with stems, stills and a 1080p render needs a few GB

func (a *App) doctor() []Check {
	cfg := a.cfg.Get()
	out := []Check{}
	add := func(c Check) { out = append(out, c) }

	// versionArg is the flag that prints a version; "" only checks the path (MuseScore may open its window)
	tool := func(id, label, bin, versionArg, fix string, required bool) {
		path, err := exec.LookPath(bin)
		if err != nil && bin == "mscore" {
			if p := "/Applications/MuseScore 4.app/Contents/MacOS/mscore"; exists(p) {
				path, err = p, nil
			}
		}
		if err != nil {
			status := "warn"
			if required {
				status = "bad"
			}
			add(Check{ID: id, Label: label, Status: status, Detail: "không thấy", Fix: fix})
			return
		}
		add(Check{ID: id, Label: label, Status: "ok", Detail: toolVersion(path, versionArg)})
	}
	tool("ffmpeg", "ffmpeg (xử lý âm thanh, xuất video)", "ffmpeg", "-version", "brew install ffmpeg", true)
	tool("ffprobe", "ffprobe", "ffprobe", "-version", "brew install ffmpeg", true)
	tool("uv", "uv (cài môi trường Python)", "uv", "--version", "brew install uv", true)
	tool("mscore", "MuseScore 4 (PDF bản nhạc)", "mscore", "", "brew install --cask musescore", false)

	if exists(filepath.Join(cfg.SkillDir, "SKILL.md")) {
		add(Check{ID: "skill", Label: "Skill suno-que-huong", Status: "ok", Detail: cfg.SkillDir})
	} else {
		add(Check{ID: "skill", Label: "Skill suno-que-huong", Status: "bad", Detail: "không thấy SKILL.md ở " + cfg.SkillDir, Fix: "Sửa thư mục Skill ở Cài đặt"})
	}

	venv := filepath.Join(a.home, ".local", "share", "suno-que-huong", "venv")
	stamp, _ := os.ReadFile(filepath.Join(venv, ".requirements.sha256"))
	switch want := fileSHA(filepath.Join(cfg.SkillDir, "scripts", "requirements-video.lock")); {
	case !exists(filepath.Join(venv, "bin", "python")):
		add(Check{ID: "venv", Label: "Môi trường Python", Status: "warn", Detail: "chưa cài", Fix: "Tự cài ở lần canh lời đầu tiên (tải PyTorch, vài phút)"})
	case want != "" && strings.TrimSpace(string(stamp)) != want:
		add(Check{ID: "venv", Label: "Môi trường Python", Status: "warn", Detail: "khác bản khoá trong skill", Fix: "Tự cập nhật ở lần chạy script tiếp theo"})
	default:
		add(Check{ID: "venv", Label: "Môi trường Python", Status: "ok", Detail: venv})
	}

	cache := filepath.Join(a.home, ".cache")
	models := []string{}
	if exists(filepath.Join(cache, "whisper", "tiny.pt")) {
		models = append(models, "Whisper tiny")
	}
	// demucs 4.1 keeps its weights in the Hugging Face cache; older releases used torch hub
	hf, _ := filepath.Glob(filepath.Join(cache, "huggingface", "hub", "models--adefossez--HTDemucs", "snapshots", "*"))
	th, _ := filepath.Glob(filepath.Join(cache, "torch", "hub", "checkpoints", "*.th"))
	if len(hf)+len(th) > 0 {
		models = append(models, "Demucs")
	}
	if len(models) == 2 {
		add(Check{ID: "models", Label: "Model tách giọng và canh lời", Status: "ok", Detail: strings.Join(models, ", ")})
	} else {
		add(Check{ID: "models", Label: "Model tách giọng và canh lời", Status: "warn", Detail: "chưa tải đủ", Fix: "Tự tải ở lần canh lời đầu tiên (khoảng 200 MB)"})
	}

	avail := 0
	for _, ag := range agentInfos(cfg) {
		if ag.Available {
			avail++
		}
	}
	if avail > 0 {
		add(Check{ID: "agents", Label: "Agent (claude, omp, codex)", Status: "ok", Detail: fmt.Sprintf("%d agent dùng được", avail)})
	} else {
		add(Check{ID: "agents", Label: "Agent (claude, omp, codex)", Status: "bad", Detail: "không thấy agent nào", Fix: "Cài Claude Code hoặc Codex, rồi sửa đường dẫn ở Cài đặt"})
	}

	var st syscall.Statfs_t
	if err := syscall.Statfs(existingParent(cfg.ProjectsDir), &st); err == nil {
		free := int64(st.Bavail) * int64(st.Bsize)
		c := Check{ID: "disk", Label: "Ổ đĩa còn trống", Status: "ok", Detail: fmt.Sprintf("%.1f GB", float64(free)/(1<<30))}
		if free < minFreeBytes {
			c.Status, c.Fix = "warn", "Dọn bớt ổ đĩa hoặc Thùng rác; mỗi bài cần vài GB khi đang làm"
		}
		add(c)
	}

	provider := cfg.Image.Provider
	key, _ := a.secrets.Get("image." + provider)
	switch {
	case key != "":
		add(Check{ID: "imagekey", Label: "Khoá tạo ảnh", Status: "ok", Detail: provider + " (Keychain)"})
	case provider == "hub" && readLegacyHubKey(a.home) != "":
		add(Check{ID: "imagekey", Label: "Khoá tạo ảnh", Status: "ok", Detail: "hub (file cũ aihub.key)", Fix: "Nên chuyển vào Keychain ở Cài đặt → Tạo ảnh"})
	default:
		add(Check{ID: "imagekey", Label: "Khoá tạo ảnh", Status: "bad", Detail: "chưa có khoá cho " + provider, Fix: "Dán khoá ở Cài đặt → Tạo ảnh"})
	}
	return out
}

// toolVersion is the first line of `<bin> <arg>`, or the path when there is no arg or no output.
func toolVersion(bin, arg string) string {
	if arg == "" {
		return bin
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	b, _ := exec.CommandContext(ctx, bin, arg).Output()
	if line := firstLine(string(b), 90); line != "" {
		return line
	}
	return bin
}

func fileSHA(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func existingParent(p string) string {
	for p != "/" && !exists(p) {
		p = filepath.Dir(p)
	}
	return p
}

func (a *App) doctorHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, a.doctor())
}

// keyPatterns catch API keys that slipped into logs, whatever their source.
var keyPatterns = regexp.MustCompile(`(sk-[A-Za-z0-9_\-]{12,}|AIza[0-9A-Za-z_\-]{20,}|(?i:bearer)\s+[A-Za-z0-9._\-]{12,})`)

func (a *App) redact(s string) string {
	for _, n := range secretNames {
		if v, err := a.secrets.Get(n); err == nil && len(v) >= 8 {
			s = strings.ReplaceAll(s, v, "[đã che]")
		}
	}
	if v := readLegacyHubKey(a.home); len(v) >= 8 {
		s = strings.ReplaceAll(s, v, "[đã che]")
	}
	return keyPatterns.ReplaceAllString(s, "[đã che]")
}

// diagnostics streams a zip to send along with a bug report: checks, settings, the
// studio log tail and recent runs, with keys redacted.
func (a *App) diagnostics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="que-huong-studio-chan-doan-%s.zip"`, time.Now().Format("060102-1504")))
	z := zip.NewWriter(w)
	defer z.Close()
	put := func(name, body string) {
		if f, err := z.Create(name); err == nil {
			_, _ = io.WriteString(f, a.redact(body))
		}
	}
	checks, _ := json.MarshalIndent(a.doctor(), "", " ")
	put("kiem-tra-may.json", string(checks))
	cfg, _ := json.MarshalIndent(a.cfg.Get(), "", " ")
	put("cau-hinh.json", string(cfg))
	put("phien-ban.txt", fmt.Sprintf("version %s\nbuild %s\nstarted %s\n", a.version, a.build, a.started.Format(time.RFC3339)))
	put("studio.log", tailFile(filepath.Join(filepath.Dir(a.runs.dir), "studio.log"), 1<<20))
	metas := a.runs.List(30)
	sort.Slice(metas, func(i, j int) bool { return metas[i].Created.After(metas[j].Created) })
	for _, m := range metas {
		m.Prompt = "" // prompts carry lyrics; the meta is enough to diagnose
		b, _ := json.MarshalIndent(m, "", " ")
		put("runs/"+m.ID+"/meta.json", string(b))
		if m.Status == "failed" || m.Status == "interrupted" {
			put("runs/"+m.ID+"/events.jsonl", tailFile(filepath.Join(a.runs.dir, m.ID, "events.jsonl"), 256<<10))
		}
	}
}

func tailFile(path string, max int64) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	if st, err := f.Stat(); err == nil && st.Size() > max {
		_, _ = f.Seek(st.Size()-max, io.SeekStart)
	}
	b, _ := io.ReadAll(f)
	return string(b)
}
