package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// ActionRequest is posted by a workflow step button.
type ActionRequest struct {
	Agent   string    `json:"agent"`
	Model   string    `json:"model"`
	Note    string    `json:"note"`    // extra instruction typed by the user
	FixAll  bool      `json:"fixAll"`  // lyrics: apply structural proposals too
	Profile string    `json:"profile"` // lyrics: "A" or "B"
	VocalAt float64   `json:"vocalAt"` // trim target second
	Times   []float64 `json:"times"`   // stills
}

// Action describes one runnable step action for the UI.
type Action struct {
	ID    string `json:"id"`
	Step  string `json:"step"`
	Label string `json:"label"`
	Kind  string `json:"kind"` // script or agent
	Hint  string `json:"hint"`
}

var workflowActions = []Action{
	{ID: "ref.analyze", Step: "ref", Label: "Phân tích bài mẫu", Kind: "script", Hint: "Đo tempo, giọng, hợp âm, bộ gõ, bass, cấu trúc"},
	{ID: "ref.agent", Step: "ref", Label: "Agent tạo kiểu phối riêng", Kind: "agent", Hint: "Từ phân tích bài mẫu và yêu cầu mở rộng, viết suno/profile.md"},
	{ID: "ref.save.agent", Step: "ref", Label: "Agent lưu kiểu phối vào skill", Kind: "agent", Hint: "Thêm thành profile dùng lại trong references/style-profiles.md"},
	{ID: "lyrics.agent", Step: "lyrics", Label: "Agent soát lời & chia đoạn", Kind: "agent", Hint: "Sửa tên, dấu, chia Verse/Chorus, viết Styles"},
	{ID: "suno.agent", Step: "suno", Label: "Agent điền form Suno", Kind: "agent", Hint: "Dùng CUA mở suno.com/create, không bấm Create"},
	{ID: "align", Step: "align", Label: "Canh lời", Kind: "script", Hint: "Tách giọng + Whisper, khoảng 3 phút"},
	{ID: "trim", Step: "trim", Label: "Cắt intro", Kind: "script", Hint: "Bỏ các câu nhạc dạo cho giọng vào ~11s"},
	{ID: "scenes.agent", Step: "scenes", Label: "Agent lên danh sách cảnh", Kind: "agent", Hint: "Viết scenes vào project.json theo lời"},
	{ID: "images", Step: "scenes", Label: "Tạo ảnh còn thiếu", Kind: "script", Hint: "gpt-image-2.5, 4 ảnh song song"},
	{ID: "stills", Step: "stills", Label: "Render khung hình xem trước", Kind: "script", Hint: "Tựa đề, đếm ngược, giữa câu, đoạn kết"},
	{ID: "render", Step: "render", Label: "Xuất video", Kind: "script", Hint: "MP4 1080p, khoảng 8–9 phút"},
	{ID: "autopilot.agent", Step: "render", Label: "Agent làm nốt các bước video", Kind: "agent", Hint: "Canh lời → cảnh → ảnh → xem trước → xuất"},
}

const promptHeader = `Bạn đang chạy trong Quê Hương Studio, không có người trả lời giữa chừng.
Dùng skill suno-que-huong ở %[1]s: đọc %[1]s/SKILL.md và đúng reference của bước này trước khi làm.
Thư mục dự án: %[2]s
Không hỏi lại người dùng. Khi cần quyết định, chọn mặc định của skill và ghi câu hỏi còn mở vào %[2]s/suno/notes.md.
Không in hay ghi khoá API ra bất kỳ đâu. Trả lời cuối bằng tiếng Việt, ngắn gọn: đã làm gì, file nào đổi, còn gì cần người dùng quyết.
`

func buildActionPrompt(action, skill, dir string, req ActionRequest, d ProjectDetail) (string, error) {
	head := fmt.Sprintf(promptHeader, skill, dir)
	var body string
	switch action {
	case "lyrics.agent":
		profile := req.Profile
		if profile != "A" {
			profile = "B"
		}
		structural := "Chỉ sửa lỗi rõ ràng; các đề xuất cấu trúc (thêm câu, sửa vần) ghi vào notes.md, chưa áp dụng."
		if req.FixAll {
			structural = "Người dùng đã đồng ý sửa hết: áp dụng cả các đề xuất cấu trúc và liệt kê chúng."
		}
		if strings.TrimSpace(d.Profile) != "" {
			profile = "riêng của bài này"
		}
		if strings.TrimSpace(d.Brief) != "" {
			structural += " Tôn trọng yêu cầu mở rộng trong " + dir + "/brief.md."
		}
		body = fmt.Sprintf(`Việc: bước 1–3 của skill cho lời ở %[1]s/lyrics.txt.
- Soát lời theo references/lyric-review.md (chạy scripts/check-luc-bat.py). %[2]s
- Dùng profile %[3]s (xem %[1]s/suno/profile.md nếu có: khi đó dùng đúng Styles, giọng và thẻ đoạn trong file đó thay cho profile A/B của references/style-profiles.md).
- Ghi %[1]s/suno/lyrics.txt: lời đã gắn thẻ đoạn, sẵn để dán vào ô Lyrics của Suno.
- Ghi %[1]s/suno/style.txt: nội dung ô Styles (dưới 1000 ký tự).
- Cập nhật %[1]s/project.json (giữ nguyên các khoá khác): "title", "subtitle", "vocal" ("Male"/"Female"), "profile".
- Ghi %[1]s/suno/notes.md: danh sách sửa (cũ → mới), đề xuất chưa áp dụng, câu hỏi mở.
- Không sửa %[1]s/lyrics.txt. Không mở trình duyệt.`, dir, structural, profile)
	case "suno.agent":
		if d.SunoLyrics == "" {
			return "", errors.New("chưa có suno/lyrics.txt: chạy bước Lời bài hát trước")
		}
		body = fmt.Sprintf(`Việc: bước 4 của skill. Điền suno.com/create qua CUA theo references/suno-cua-fill.md.
- Lyrics: nội dung %[1]s/suno/lyrics.txt. Styles: %[1]s/suno/style.txt.
- Vocal Gender và tên bài lấy từ %[1]s/project.json ("vocal", "title" + "subtitle").
- Kiểm tra từng ô sau khi dán, kiểm tra thanh địa chỉ không bị dán nhầm.
- TUYỆT ĐỐI không bấm Create.`, dir)
	case "scenes.agent":
		if len(d.Timings) == 0 {
			return "", errors.New("chưa có timing: chạy bước Canh lời trước")
		}
		body = fmt.Sprintf(`Việc: lên danh sách cảnh cho video theo references/karaoke-video.md (mục Plan scenes và Style contract).
- Đọc %[1]s/work/timings.json, %[1]s/lyrics.txt và %[1]s/project.json.
- Ghi mảng "scenes" vào %[1]s/project.json, giữ nguyên các khoá khác. Mỗi cảnh: "id" dạng "NN-ten-ngan", "start" (giây, trùng đầu một câu), "prompt" tiếng Anh nêu đúng địa danh.
- Cảnh 1 là ảnh tựa đề (nửa trái trống), cảnh 2 bắt đầu khoảng 5 giây trước chữ hát đầu tiên, cảnh cuối dùng lại cảnh 1 với "reuse": true làm cảnh kết khoảng 15 giây trước khi hết bài.
- Không tạo ảnh và không render.`, dir)
	case "ref.agent":
		if len(d.Refs) == 0 && strings.TrimSpace(d.Brief) == "" {
			return "", errors.New("cần ít nhất một bài mẫu hoặc yêu cầu mở rộng")
		}
		body = fmt.Sprintf(`Việc: tạo kiểu phối riêng cho bài này, rồi ghi vào %[1]s/suno/profile.md.
Đầu vào:
- Yêu cầu mở rộng của người dùng: %[1]s/brief.md (ưu tiên cao nhất; làm đúng từng ý).
- Kết quả đo bài mẫu: %[1]s/ref/*.analysis.txt. Nếu bài mẫu chưa có file phân tích, tự chạy: PY=$(bash scripts/setup-video-env.sh) && "$PY" scripts/analyze-reference.py "<file>" trong thư mục skill.
- Các kiểu phối đã có và cách chuyển số đo thành prompt: references/style-profiles.md (mục "Adapting a profile").
Quy tắc:
- Chỉ học phong cách (tempo, chất vùng miền, dàn nhạc, độ dày bộ gõ và bass, độ dài intro). Không chép giai điệu, lời, tên bài hay câu nhạc đặc trưng của bài mẫu. Không đưa file bài mẫu lên Suno hay dịch vụ nào.
- Styles viết tiếng Anh, dưới 1000 ký tự, mô tả theo hướng có gì (Suno hiểu sai câu phủ định): ví dụ "không bộ gõ" thành "no percussion section; the pulse is carried by plucked đàn tranh and the vocal phrasing", và bỏ mọi chữ drums/percussion khỏi thẻ đoạn.
Nội dung %[1]s/suno/profile.md (tiếng Việt, trừ ô Styles):
1. Tóm tắt bài mẫu: các số đo chính và cảm nhận phong cách.
2. Các yêu cầu mở rộng đã áp dụng thế nào.
3. Styles (khối code, sẵn để dán vào Suno).
4. Vocal Gender.
5. Thẻ đoạn: [Intro …] tới [End], theo cấu trúc của skill.
Không sửa lyrics.txt, không chạy các bước khác.`, dir)
	case "ref.save.agent":
		if strings.TrimSpace(d.Profile) == "" {
			return "", errors.New("chưa có suno/profile.md: chạy 'Agent tạo kiểu phối riêng' trước")
		}
		body = fmt.Sprintf(`Việc: thêm kiểu phối trong %[1]s/suno/profile.md vào skill thành một profile dùng lại được.
- Sửa references/style-profiles.md: thêm mục mới (đặt tên theo ghi chú của người dùng, nếu không có thì đặt theo đặc điểm chính), giữ đúng định dạng các profile A/B: số đo, khối Styles, Vocal Gender, thẻ đoạn. Không đổi các profile cũ.
- Cập nhật SKILL.md và references/lyric-review.md chỉ ở chỗ liệt kê lựa chọn profile, nếu cần.
- Làm theo luồng update của ~/.claude/skills/ak-skill-creator nếu có, rồi chạy quick_validate, lint_cruft --routing và test trong scripts/tests.`, dir)
	case "autopilot.agent":
		body = fmt.Sprintf(`Việc: làm nốt các bước video còn thiếu cho dự án theo references/karaoke-video.md, theo thứ tự: canh lời (nếu chưa có work/timings.json), cắt intro (nếu giọng vào sau 15 giây), lên cảnh, tạo ảnh (xem contact sheet, tạo lại ảnh lỗi), render khung hình kiểm tra và tự xem chúng, rồi xuất video ra "%[1]s/<title> <subtitle> - Karaoke.mp4".
- Dùng python của môi trường cố định: PY=$(bash scripts/setup-video-env.sh).
- Chạy từng bước nặng một lần, không chạy song song.`, dir)
	default:
		return "", fmt.Errorf("unknown agent action %q", action)
	}
	if strings.TrimSpace(req.Note) != "" {
		body += "\nGhi chú thêm của người dùng: " + strings.TrimSpace(req.Note)
	}
	return head + "\n" + body, nil
}

// scriptArgv builds a deterministic script run in the skill directory.
// Paths reach bash only as positional arguments, never spliced into the script text.
func scriptArgv(action, skill, dir string, req ActionRequest, d ProjectDetail) ([]string, string, error) {
	py := `PY=$(bash scripts/setup-video-env.sh) && exec "$PY" `
	switch action {
	case "ref.analyze":
		if len(d.Refs) == 0 {
			return nil, "", errors.New("chưa có bài mẫu")
		}
		argv := []string{"/bin/bash", "-c", `set -o pipefail; PY=$(bash scripts/setup-video-env.sh) || exit 1
for f in "$@"; do echo "== $(basename "$f")"; "$PY" scripts/analyze-reference.py "$f" 2>/dev/null | tee "$f.analysis.txt" || exit 1; done`, "analyze"}
		for _, r := range d.Refs {
			argv = append(argv, filepath.Join(dir, r.Path))
		}
		return argv, "Phân tích bài mẫu", nil
	case "align":
		if d.Audio == "" {
			return nil, "", errors.New("chưa có file nhạc")
		}
		return []string{"/bin/bash", "-c", py + `scripts/karaoke-align.py "$1"`, "align", dir}, "Canh lời từng chữ", nil
	case "trim":
		if len(d.Timings) == 0 {
			return nil, "", errors.New("chưa có timing: chạy Canh lời trước")
		}
		at := req.VocalAt
		if at < 5 || at > 30 {
			at = 11.5
		}
		return []string{"/bin/bash", "-c", py + `scripts/trim-intro.py "$1" --vocal-at "$2"`, "trim", dir, fmt.Sprintf("%.2f", at)}, "Rút ngắn intro", nil
	case "images":
		return []string{"python3", "scripts/generate-scene-images.py", dir}, "Tạo ảnh cảnh", nil
	case "stills":
		times := req.Times
		if len(times) == 0 {
			times = defaultStillTimes(d)
		}
		argv := []string{"/bin/bash", "-c", py + `scripts/render-karaoke-video.py "$1" still "${@:2}"`, "stills", dir}
		for _, t := range times {
			argv = append(argv, fmt.Sprintf("%.1f", t))
		}
		return argv, "Render khung hình xem trước", nil
	case "render":
		name := safeFileName(d.Title+" "+d.Subtitle, filepath.Base(dir)) + " - Karaoke.mp4"
		out := filepath.Join(dir, name)
		return []string{"/bin/bash", "-c", py + `scripts/render-karaoke-video.py "$1" full "$2"`, "render", dir, out}, "Xuất video karaoke", nil
	}
	return nil, "", fmt.Errorf("unknown script action %q", action)
}

func defaultStillTimes(d ProjectDetail) []float64 {
	t := []float64{3}
	if len(d.Timings) > 0 {
		first := d.Timings[0].Start
		t = append(t, first-2.5, first+1.5)
		longest := d.Timings[0]
		for i, l := range d.Timings {
			if len(l.Text) > len(longest.Text) {
				longest = l
			}
			if i > 0 && l.Start-d.Timings[i-1].End > 8 { // first line after a long gap: countdown
				t = append(t, l.Start-2)
			}
		}
		t = append(t, (longest.Start+longest.End)/2)
		last := d.Timings[len(d.Timings)-1]
		t = append(t, last.End+4)
	}
	sort.Float64s(t)
	out := []float64{}
	for _, x := range t {
		if x >= 0 && (len(out) == 0 || x-out[len(out)-1] > 0.5) {
			out = append(out, x)
		}
	}
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

const skillPromptHeader = `Bạn đang sửa skill suno-que-huong ở %[1]s theo yêu cầu bên dưới, trong Quê Hương Studio (không có người trả lời giữa chừng).
- Đọc SKILL.md và các file liên quan trước khi sửa. Giữ nguyên hành vi đã được chấp nhận; chỉ đổi đúng phần được yêu cầu.
- Nếu có ~/.claude/skills/ak-skill-creator, làm theo luồng "update" của nó và chạy các bước kiểm tra (quick_validate, lint_cruft --routing); chạy test trong scripts/tests nếu có.
- Không ghi khoá API hay thông tin bí mật vào skill.
- Trả lời cuối bằng tiếng Việt: file nào đã sửa, vì sao, kết quả kiểm tra.

Yêu cầu: `
