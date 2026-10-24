package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ActionRequest is posted by a workflow step button.
type ActionRequest struct {
	Agent   string    `json:"agent"`
	Model   string    `json:"model"`
	Note    string    `json:"note"`    // extra instruction typed by the user
	FixAll  bool      `json:"fixAll"`  // lyrics: apply structural proposals too
	Profile string    `json:"profile"` // "" = choose from references and brief, else a skill profile id
	Times   []float64 `json:"times"`   // stills
	Meter   string    `json:"meter"`   // sheet: 2/4, 3/4 or 4/4; "" = last run's
	Key     string    `json:"key"`     // sheet: tonic to transpose to, "orig" = as sung; "" = last run's
	// chained is set for links of a chain: inputs made by earlier links do not exist yet,
	// so per-step preconditions are skipped and the scripts check at run time.
	chained bool
}

// Action describes one runnable step action for the UI.
type Action struct {
	ID    string `json:"id"`
	Step  string `json:"step"`
	Label string `json:"label"`
	Kind  string `json:"kind"` // script or agent
	Heavy bool   `json:"heavy,omitempty"`
	Hint  string `json:"hint"`
	// Secrets the script needs in its environment (see Manager.SecretEnv).
	Secrets []string `json:"-"`
}

// workflowActions lists the step actions with labels and hints in the studio language.
func workflowActions() []Action {
	return []Action{
		{ID: "ref.analyze", Step: "lyrics", Label: tr("Phân tích bài mẫu", "Analyze references"), Kind: "script", Heavy: true, Hint: tr("Đo tempo, giọng, hợp âm, bộ gõ, bass, cấu trúc", "Measures tempo, key, chords, percussion, bass, structure")},
		{ID: "lyrics.agent", Step: "suno", Label: tr("Soát lời & chia đoạn", "Review lyrics & sections"), Kind: "agent", Hint: tr("Sửa tên, dấu, dấu câu; chia Verse/Chorus", "Fixes names, diacritics, punctuation; splits Verse/Chorus")},
		{ID: "style.agent", Step: "suno", Label: tr("Phối khí & viết Styles", "Arrange & write Styles"), Kind: "agent", Hint: tr("Từ bài mẫu, yêu cầu và kiểu phối, viết Styles và thẻ đoạn", "Writes Styles and section tags from the references, brief and arrangement")},
		{ID: "suno.agent", Step: "suno", Label: tr("Agent điền form Suno", "Agent fills the Suno form"), Kind: "agent", Hint: tr("Dùng CUA mở suno.com/create, không bấm Create", "Opens suno.com/create with CUA, never clicks Create")},
		{ID: "align", Step: "align", Label: tr("Canh lời", "Align lyrics"), Kind: "script", Heavy: true, Hint: tr("Tách giọng + Whisper, khoảng 3 phút", "Vocal separation + Whisper, about 3 minutes")},
		{ID: "scenes.agent", Step: "scenes", Label: tr("Agent lên danh sách cảnh", "Agent plans the scenes"), Kind: "agent", Hint: tr("Viết scenes vào project.json theo lời", "Writes scenes into project.json from the lyrics")},
		{ID: "images", Step: "scenes", Label: tr("Tạo ảnh còn thiếu", "Generate missing images"), Kind: "script", Hint: tr("4 ảnh song song, nguồn chọn ở Cài đặt", "4 images at once, provider set in Settings"), Secrets: []string{"image"}},
		{ID: "stills", Step: "stills", Label: tr("Render khung hình xem trước", "Render preview frames"), Kind: "script", Heavy: true, Hint: tr("Tựa đề, đếm ngược, giữa câu, đoạn kết", "Title, countdown, mid-line, ending")},
		{ID: "render", Step: "render", Label: tr("Xuất video", "Render video"), Kind: "script", Heavy: true, Hint: tr("MP4 1080p, khoảng 8–9 phút", "1080p MP4, about 8–9 minutes")},
		{ID: "sheet", Step: "sheet", Label: tr("Chép bản nhạc", "Transcribe sheet music"), Kind: "script", Heavy: true, Hint: tr("Giai điệu và lời từ giọng hát, ra PDF; khoảng 30 giây", "Melody and lyrics from the vocal, as a PDF; about 30 seconds")},
	}
}

func findAction(id string) *Action {
	acts := workflowActions()
	for i := range acts {
		if acts[i].ID == id {
			return &acts[i]
		}
	}
	return nil
}

// Profile is one arrangement profile defined in the skill's references/style-profiles.md.
type Profile struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

var profileHeading = regexp.MustCompile(`(?m)^## ([A-Z][A-Z0-9]*)\. (.+)$`)

// skillProfiles reads the profile list from the skill, so new profiles appear without code changes.
func skillProfiles(skill string) []Profile {
	b, err := os.ReadFile(filepath.Join(skill, "references", "style-profiles.md"))
	if err != nil {
		return nil
	}
	out := []Profile{}
	for _, m := range profileHeading.FindAllStringSubmatch(string(b), -1) {
		out = append(out, Profile{ID: m[1], Title: strings.TrimSpace(m[2])})
	}
	return out
}

func profileInstruction(req ActionRequest, skill string) string {
	if req.Profile != "" {
		for _, p := range skillProfiles(skill) {
			if p.ID == req.Profile {
				return fmt.Sprintf("Người dùng chọn profile %s (%s) trong references/style-profiles.md: lấy làm nền, rồi chỉnh theo yêu cầu mở rộng.", p.ID, p.Title)
			}
		}
	}
	return "Phối theo bài mẫu: lấy tempo, nhịp, giọng, nhạc cụ, bộ gõ, bass và cấu trúc từ ref/*.analysis.txt, rồi áp yêu cầu mở rộng. references/style-profiles.md chỉ để học cách viết Styles và thẻ đoạn, không ép bài theo một profile có sẵn. Khi không có bài mẫu thì mới dùng profile mặc định của skill."
}

const promptHeader = `Bạn đang chạy trong Song Studio, không có người trả lời giữa chừng.
Dùng skill suno-que-huong ở %[1]s: đọc %[1]s/SKILL.md và đúng reference của bước này trước khi làm.
Thư mục dự án: %[2]s
Không hỏi lại người dùng. Khi cần quyết định, chọn mặc định của skill và ghi câu hỏi còn mở vào %[2]s/suno/notes.md.
Không in hay ghi khoá API ra bất kỳ đâu. Trả lời cuối bằng tiếng Việt, ngắn gọn: đã làm gì, file nào đổi, còn gì cần người dùng quyết.
`

// projectPromptHeader starts every agent prompt run in a song folder. When the studio is in
// English, the agent writes what the user reads in English; the instructions stay Vietnamese.
func projectPromptHeader(skill, dir string) string {
	return fmt.Sprintf(promptHeader, skill, dir) + tr("", "The studio is in English: write every note meant for the user (notes.md, your final reply) in English, even where this prompt says Vietnamese. Song lyrics stay in their sung language.\n")
}

func buildActionPrompt(action, skill, dir string, req ActionRequest, d ProjectDetail) (string, error) {
	head := projectPromptHeader(skill, dir)
	var body string
	switch action {
	case "lyrics.agent":
		structural := "Chỉ sửa lỗi rõ ràng; các đề xuất cấu trúc (thêm câu, sửa vần) ghi vào notes.md, chưa áp dụng."
		if req.FixAll {
			structural = "Người dùng đã đồng ý sửa hết: áp dụng cả các đề xuất cấu trúc và liệt kê chúng."
		}
		body = fmt.Sprintf(`Việc (một phần của chuỗi, chạy song song với việc phân tích bài mẫu): soát lời và chia đoạn cho %[1]s/lyrics.txt.
- Soát lời theo references/lyric-review.md; chỉ chạy scripts/check-luc-bat.py khi lời là tiếng Việt có dạng lục bát. %[2]s
- Chia đoạn theo mục "Mapping lyrics to sections" và ghi %[1]s/suno/lyrics.txt: lời đã gắn thẻ đoạn trơn như [Intro], [Verse 1], [Chorus], [Interlude], [Break], [Final Chorus], [Outro], [End], CHƯA ghi mô tả nhạc cụ trong thẻ (bước phối khí sẽ thêm).
- Nếu chưa có "title"/"subtitle" trong %[1]s/project.json thì đặt theo mục Title (giữ nguyên các khoá khác).
- Ghi %[1]s/suno/notes.md: danh sách sửa (cũ → mới), đề xuất chưa áp dụng, câu hỏi mở.
- Không sửa %[1]s/lyrics.txt, không viết Styles, không mở trình duyệt.`, dir, structural)
	case "style.agent":
		body = fmt.Sprintf(`Việc (bước cuối của chuỗi tạo prompt Suno): phối khí và viết prompt cho bài.
Đầu vào, theo thứ tự ưu tiên:
1. Yêu cầu mở rộng của người dùng: %[1]s/brief.md (nếu có; làm đúng từng ý).
2. Số đo bài mẫu: %[1]s/ref/*.analysis.txt (nếu có).
3. %[2]s
4. Lời đã chia đoạn: %[1]s/suno/lyrics.txt.
Quy tắc:
- Chỉ học phong cách từ bài mẫu (tempo, chất vùng miền, dàn nhạc, độ dày bộ gõ và bass, độ dài intro). Không chép giai điệu, lời, tên bài hay câu nhạc đặc trưng. Không đưa file bài mẫu lên Suno hay dịch vụ nào.
- Styles hợp với thể loại ở trên (nếu chưa chọn thì suy ra từ bài mẫu và yêu cầu); ngôn ngữ hát ghi rõ trong Styles khi không phải tiếng Việt.
- Styles tiếng Anh, dưới 1000 ký tự, mô tả theo hướng có gì vì Suno hiểu sai câu phủ định (ví dụ "không bộ gõ" thành "no percussion section; the pulse is carried by plucked strings and the vocal phrasing").
Ghi ra:
- %[1]s/suno/style.txt: nội dung ô Styles.
- %[1]s/suno/lyrics.txt: thêm mô tả ngắn bằng tiếng Anh vào từng thẻ đoạn (ví dụ [Verse 2: đàn tranh and soft strings join]), đúng yêu cầu (không bộ gõ thì không nhắc drums/percussion). Không đổi dòng lời nào.
- %[1]s/project.json: cập nhật "vocal" ("Male"/"Female") và "profile" (id profile nền hoặc "custom"), giữ nguyên các khoá khác.`, dir, profileInstruction(req, skill))
	case "suno.agent":
		if d.SunoLyrics == "" {
			return "", errors.New(tr("chưa có prompt: bấm Tạo prompt Suno trước", "no prompt yet: click Create Suno prompt first"))
		}
		body = fmt.Sprintf(`Việc: bước 4 của skill. Điền suno.com/create qua CUA theo references/suno-cua-fill.md.
- Lyrics: nội dung %[1]s/suno/lyrics.txt. Styles: %[1]s/suno/style.txt.
- Vocal Gender và tên bài lấy từ %[1]s/project.json ("vocal", "title" + "subtitle").
- Kiểm tra từng ô sau khi dán, kiểm tra thanh địa chỉ không bị dán nhầm.
- TUYỆT ĐỐI không bấm Create.`, dir)
	case "scenes.agent":
		if len(d.Timings) == 0 {
			return "", errors.New(tr("chưa có timing: chạy bước Canh lời trước", "no timing yet: run the Align lyrics step first"))
		}
		body = sceneBody(dir, d.Config)
	default:
		return "", fmt.Errorf("unknown agent action %q", action)
	}
	if strings.TrimSpace(req.Note) != "" {
		body += "\nGhi chú thêm của người dùng: " + strings.TrimSpace(req.Note)
	}
	if action == "scenes.agent" { // sceneBody already names the genre
		return head + "\n" + body, nil
	}
	var pc projectConfig
	_ = json.Unmarshal(d.Config, &pc)
	return head + "\n" + genreLine(pc) + "\n" + body, nil
}

func sceneBody(dir string, config json.RawMessage) string {
	var pc projectConfig
	_ = json.Unmarshal(config, &pc)
	style := "ảnh chụp điện ảnh chân thực, hợp nội dung lời"
	if g := findGenre(pc.Genre); g != nil {
		style = g.ImageStyle
	}
	return genreLine(pc) + "\n" + fmt.Sprintf(`Việc: lên danh sách cảnh cho video theo references/karaoke-video.md (mục Plan scenes và Style contract).
- Đọc %[1]s/work/timings.json, %[1]s/lyrics.txt và %[1]s/project.json.
- Ghi mảng "scenes" vào %[1]s/project.json, giữ nguyên các khoá khác. Mỗi cảnh: "id" dạng "NN-ten-ngan", "start" (giây, trùng đầu một câu), "prompt" tiếng Anh nêu đúng địa danh hoặc hình ảnh trong lời.
- Ghi "image_style" vào %[1]s/project.json: một câu tiếng Anh về phong cách ảnh chung cho cả bài, hợp thể loại và lời, kết thúc bằng "No text, no letters, no watermark, no logo." Gợi ý cho thể loại này: %[2]s
- Cảnh 1 là ảnh tựa đề (nửa trái trống), cảnh 2 bắt đầu khoảng 5 giây trước chữ hát đầu tiên, cảnh cuối dùng lại cảnh 1 với "reuse": true làm cảnh kết khoảng 15 giây trước khi hết bài.
- Không tạo ảnh và không render.`, dir, style)
}

// sceneRetimeBody keeps the existing scenes and images and only moves their start times
// onto the new timings (used after the music was replaced and re-aligned).
func sceneRetimeBody(dir string) string {
	return fmt.Sprintf(`Việc: nhạc của bài vừa được thay và canh lời lại, nên mốc thời gian các cảnh đã lệch.
- Đọc %[1]s/work/timings.json (mới) và mảng "scenes" trong %[1]s/project.json.
- Giữ nguyên "id", "prompt", "reuse" và thứ tự các cảnh; chỉ sửa "start" để mỗi cảnh lại bắt đầu đúng đầu câu lời tương ứng, theo quy tắc trong references/karaoke-video.md (cảnh 2 khoảng 5 giây trước chữ hát đầu, cảnh kết khoảng 15 giây trước khi hết bài).
- Không đổi khoá nào khác, không tạo ảnh, không render.`, dir)
}

// pipelineSpecs lays out a chain of runs: specs plus, for each, the indexes it waits on.
// "suno": [reference analysis] ‖ lyric review → arrangement & Styles.
// "video": align → scenes → images → stills → render (+ sheet), skipping steps already done.
func pipelineSpecs(name string, cfg Config, d ProjectDetail, req ActionRequest) ([]RunSpec, [][]int, error) {
	agent := req.Agent
	if agent == "" {
		agent = cfg.DefaultAgent
	}
	var specs []RunSpec
	var deps [][]int
	addScript := func(id string, after []int, override ...ActionRequest) error {
		act := findAction(id)
		r := req
		if len(override) > 0 {
			r = override[0]
		}
		r.chained = true
		argv, title, err := scriptArgv(id, cfg.SkillDir, d.Dir, r, d)
		if err != nil {
			return err
		}
		spec := RunSpec{Kind: "script", Step: act.Step, Project: d.Slug, Cwd: cfg.SkillDir, Argv: argv, Title: title, Heavy: act.Heavy, Secrets: act.Secrets}
		if id == "stills" || id == "render" {
			spec.FingerprintDir = d.Dir
		}
		specs, deps = append(specs, spec), append(deps, after)
		return nil
	}
	addAgent := func(id string, prompt string, after []int) error {
		act := findAction(id)
		argv, err := agentArgv(cfg, agent, req.Model, prompt, d.Dir)
		if err != nil {
			return err
		}
		specs = append(specs, RunSpec{Kind: "agent", Step: act.Step, Project: d.Slug, Cwd: d.Dir, Agent: agent, Model: req.Model, Prompt: prompt, Argv: argv, Title: act.Label})
		deps = append(deps, after)
		return nil
	}
	last := func() []int {
		if len(specs) == 0 {
			return nil
		}
		return []int{len(specs) - 1}
	}
	switch name {
	case "suno":
		if strings.TrimSpace(d.Lyrics) == "" {
			return nil, nil, errors.New(tr("cần có lời bài hát trước", "add the song lyrics first"))
		}
		wait := []int{}
		unanalysed := false
		for _, r := range d.Refs {
			if r.Analysis == "" {
				unanalysed = true
			}
		}
		if unanalysed {
			if err := addScript("ref.analyze", nil); err != nil {
				return nil, nil, err
			}
			wait = append(wait, len(specs)-1)
		}
		lp, err := buildActionPrompt("lyrics.agent", cfg.SkillDir, d.Dir, req, d)
		if err != nil {
			return nil, nil, err
		}
		if err := addAgent("lyrics.agent", lp, nil); err != nil {
			return nil, nil, err
		}
		wait = append(wait, len(specs)-1)
		sp, _ := buildActionPrompt("style.agent", cfg.SkillDir, d.Dir, req, d)
		if err := addAgent("style.agent", sp, wait); err != nil {
			return nil, nil, err
		}
	case "video":
		if d.Audio == "" {
			return nil, nil, errors.New(tr("cần thêm file nhạc M4A trước", "add the M4A audio file first"))
		}
		var pc projectConfig
		_ = json.Unmarshal(d.Config, &pc)
		status := map[string]string{}
		for _, s := range d.Steps {
			status[s.ID] = s.Status
		}
		// stale timings (the music was replaced) are redone, not reused
		realign := len(d.Timings) == 0 || status["align"] == "stale" || status["align"] == "todo"
		if realign {
			if err := addScript("align", nil); err != nil {
				return nil, nil, err
			}
		}
		timed := last() // the lead sheet only needs the timings, so it runs beside the scene work
		header := projectPromptHeader(cfg.SkillDir, d.Dir) + "\n"
		switch {
		case len(pc.Scenes) == 0:
			if err := addAgent("scenes.agent", header+sceneBody(d.Dir, d.Config), last()); err != nil {
				return nil, nil, err
			}
		case realign:
			if err := addAgent("scenes.agent", header+sceneRetimeBody(d.Dir), last()); err != nil {
				return nil, nil, err
			}
		}
		if err := addScript("images", last()); err != nil {
			return nil, nil, err
		}
		// stills and render read timings and scenes when they start, so default times are recomputed by the script itself
		if err := addScript("stills", last(), ActionRequest{}); err != nil {
			return nil, nil, err
		}
		if err := addScript("render", last()); err != nil {
			return nil, nil, err
		}
		if realign || status["sheet"] != "done" {
			if err := addScript("sheet", timed); err != nil {
				return nil, nil, err
			}
		}
	default:
		return nil, nil, fmt.Errorf("unknown pipeline %q", name)
	}
	return specs, deps, nil
}

var sheetKey = regexp.MustCompile(`^([A-G](b|#)?|orig)?$`)

// scriptArgv builds a deterministic script run in the skill directory.
// Paths reach bash only as positional arguments, never spliced into the script text.
func scriptArgv(action, skill, dir string, req ActionRequest, d ProjectDetail) ([]string, string, error) {
	py := `PY=$(bash scripts/setup-video-env.sh) && exec "$PY" `
	switch action {
	case "ref.analyze":
		if len(d.Refs) == 0 {
			return nil, "", errors.New(tr("chưa có bài mẫu", "no reference songs yet"))
		}
		argv := []string{"/bin/bash", "-c", `set -o pipefail; PY=$(bash scripts/setup-video-env.sh) || exit 1
for f in "$@"; do echo "== $(basename "$f")"; "$PY" scripts/analyze-reference.py "$f" 2>/dev/null | tee "$f.analysis.txt" || exit 1; done`, "analyze"}
		for _, r := range d.Refs {
			argv = append(argv, filepath.Join(dir, r.Path))
		}
		return argv, tr("Phân tích bài mẫu", "Analyze references"), nil
	case "align":
		if d.Audio == "" && !req.chained {
			return nil, "", errors.New(tr("chưa có file nhạc", "no audio file yet"))
		}
		return []string{"/bin/bash", "-c", py + `scripts/karaoke-align.py "$1"`, "align", dir}, tr("Canh lời từng chữ", "Word-by-word alignment"), nil
	case "images":
		return []string{"python3", "scripts/generate-scene-images.py", dir}, tr("Tạo ảnh cảnh", "Generate scene images"), nil
	case "stills":
		argv := []string{"/bin/bash", "-c", py + `scripts/render-karaoke-video.py "$1" still "${@:2}"`, "stills", dir}
		if len(req.Times) == 0 {
			argv = append(argv, "auto") // the renderer picks checkpoints from the timings it reads at start
		}
		for _, t := range req.Times {
			argv = append(argv, fmt.Sprintf("%.1f", t))
		}
		return argv, tr("Render khung hình xem trước", "Render preview frames"), nil
	case "render":
		name := safeFileName(d.Title+" "+d.Subtitle, filepath.Base(dir)) + " - Karaoke.mp4"
		out := filepath.Join(dir, name)
		return []string{"/bin/bash", "-c", py + `scripts/render-karaoke-video.py "$1" full "$2"`, "render", dir, out}, tr("Xuất video karaoke", "Render karaoke video"), nil
	case "sheet":
		if len(d.Timings) == 0 && !req.chained {
			return nil, "", errors.New(tr("chưa có timing: chạy Canh lời trước", "no timing yet: run Align lyrics first"))
		}
		if !contains([]string{"", "2/4", "3/4", "4/4"}, req.Meter) || !sheetKey.MatchString(req.Key) {
			return nil, "", errors.New(tr("nhịp hoặc giọng không hợp lệ", "invalid meter or key"))
		}
		out := filepath.Join(dir, safeFileName(d.Title+" "+d.Subtitle, filepath.Base(dir))+" - Bản nhạc")
		return []string{"/bin/bash", "-c", py + `scripts/transcribe-sheet.py "$1" --out "$2" ${3:+--meter "$3"} ${4:+--key "$4"}`,
			"sheet", dir, out, req.Meter, req.Key}, tr("Chép bản nhạc", "Transcribe sheet music"), nil
	}
	return nil, "", fmt.Errorf("unknown script action %q", action)
}

const skillPromptHeader = `Bạn đang sửa skill suno-que-huong ở %[1]s theo yêu cầu bên dưới, trong Song Studio (không có người trả lời giữa chừng).
- Đọc SKILL.md và các file liên quan trước khi sửa. Giữ nguyên hành vi đã được chấp nhận; chỉ đổi đúng phần được yêu cầu.
- Nếu có ~/.claude/skills/ak-skill-creator, làm theo luồng "update" của nó và chạy các bước kiểm tra (quick_validate, lint_cruft --routing); chạy test trong scripts/tests nếu có.
- Không ghi khoá API hay thông tin bí mật vào skill.
- Trả lời cuối bằng tiếng Việt: file nào đã sửa, vì sao, kết quả kiểm tra.

Yêu cầu: `
