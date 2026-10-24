package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Genre gives the agents and the video a starting point for one kind of song. The style
// itself still comes from the reference song and the brief; this only sets sensible defaults.
type Genre struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Theme   string   `json:"theme"` // video_theme for the title card and karaoke colours
	Presets []Preset `json:"presets"`
	// ImageStyle is the photographic direction for scene images, in English for the image model.
	ImageStyle string `json:"imageStyle"`
}

var genres = []Genre{
	{ID: "que-huong", Label: "Quê hương, dân ca", Theme: "truyen-thong",
		Presets: []Preset{
			{"Dân ca mượt mà", "Phong cách dân ca mượt mà: câu hát liền mạch, luyến láy vừa phải ở cuối câu, không ề à, tempo vừa phải."},
			{"Nhạc cụ dân tộc", "Sáo trúc, đàn tranh, đàn bầu dẫn giai điệu; dây nhẹ phía sau."},
			{"Không bộ gõ", "Không dùng bộ gõ: không trống, không cajon, không hi-hat; nhịp do đàn và câu hát dẫn dắt."},
		},
		ImageStyle: "Photorealistic cinematic photograph of Vietnamese countryside and landmarks, warm golden-hour light, rich greens, natural Vietnamese faces and clothing."},
	{ID: "bolero", Label: "Bolero, trữ tình", Theme: "co-dien",
		Presets: []Preset{
			{"Bolero chuẩn", "Nhịp bolero chậm, guitar phím lõm và bass đệm theo tiết tấu bolero, dàn dây nhẹ; giọng hát tình cảm, nhả chữ rõ."},
			{"Trữ tình sang trọng", "Phối trữ tình sang trọng: piano, dàn dây, saxophone dạo; không điện tử."},
		},
		ImageStyle: "Nostalgic cinematic photograph, soft warm film tones, gentle rain or dusk light, quiet streets, rivers and old houses, emotional and tender mood."},
	{ID: "ballad", Label: "Ballad, nhạc nhẹ", Theme: "hien-dai",
		Presets: []Preset{
			{"Piano ballad", "Piano dẫn, dây vào ở điệp khúc, trống nhẹ từ verse 2; cao trào ở điệp khúc cuối."},
			{"Acoustic ấm", "Guitar mộc và piano, không điện tử, giọng hát gần và ấm."},
		},
		ImageStyle: "Cinematic photograph with soft natural light, shallow depth of field, intimate emotional moments, muted pastel colors."},
	{ID: "pop", Label: "Pop, nhạc trẻ", Theme: "hien-dai",
		Presets: []Preset{
			{"Pop hiện đại", "Pop hiện đại, nhịp vừa nhanh, hook điệp khúc dễ nhớ, synth và trống điện tử sạch."},
			{"V-pop sáng", "Màu V-pop tươi sáng, điệp khúc bắt tai, pre-chorus dồn lên."},
		},
		ImageStyle: "Vibrant modern photograph, bold colors, city lights and youthful energy, clean composition."},
	{ID: "rock", Label: "Rock", Theme: "hien-dai",
		Presets: []Preset{
			{"Rock ban nhạc", "Ban nhạc rock: guitar điện distortion, bass, trống mạnh; giọng hát khoẻ."},
			{"Pop rock", "Pop rock dễ nghe, guitar điện sáng, điệp khúc bùng nổ."},
		},
		ImageStyle: "High-contrast cinematic photograph, dramatic light, stage lights or wild landscapes, gritty texture."},
	{ID: "rap", Label: "Rap, hip-hop", Theme: "hien-dai",
		Presets: []Preset{
			{"Hip-hop", "Beat hip-hop rõ, kick và snare chắc, 808 bass; flow rap rõ chữ, điệp khúc hát."},
			{"Lo-fi chill", "Lo-fi hip-hop thư giãn, piano lo-fi, trống nhẹ có swing."},
		},
		ImageStyle: "Urban cinematic photograph, night city, neon and street details, strong contrast."},
	{ID: "edm", Label: "EDM, dance", Theme: "hien-dai",
		Presets: []Preset{
			{"EDM sôi động", "EDM sôi động, build-up và drop rõ ràng, synth lead sáng, nhịp four-on-the-floor."},
		},
		ImageStyle: "Energetic photograph with colorful lights, motion and celebration, glowing night atmosphere."},
	{ID: "acoustic", Label: "Acoustic, indie", Theme: "hien-dai",
		Presets: []Preset{
			{"Indie mộc", "Indie mộc: guitar thùng, ukulele hoặc piano, trống cajon nhẹ; giọng hát tự nhiên."},
		},
		ImageStyle: "Natural film photograph, soft daylight, simple everyday scenes, cozy and honest mood."},
	{ID: "thieu-nhi", Label: "Thiếu nhi", Theme: "truyen-thong",
		Presets: []Preset{
			{"Vui tươi", "Vui tươi, nhịp nhảy, nhạc cụ sáng (xylophone, ukulele, piano), giai điệu dễ hát theo."},
		},
		ImageStyle: "Bright cheerful photograph or soft 3D illustration style, playful colors, happy children and friendly animals."},
	{ID: "thanh-ca", Label: "Thánh ca, nhạc đạo", Theme: "co-dien",
		Presets: []Preset{
			{"Trang nghiêm", "Trang nghiêm, organ hoặc piano, dàn dây và hợp xướng nhẹ, tempo chậm."},
		},
		ImageStyle: "Serene cinematic photograph, soft divine light, candles, churches or temples, peaceful nature."},
	{ID: "cach-mang", Label: "Nhạc cách mạng", Theme: "co-dien",
		Presets: []Preset{
			{"Hào hùng", "Hào hùng, hành khúc, kèn đồng và trống quân đội, dàn dây; hợp xướng ở điệp khúc."},
		},
		ImageStyle: "Epic cinematic photograph, flags, crowds and historic landmarks, golden light, proud mood."},
}

// Languages are the sung languages karaoke alignment knows; the UI itself stays Vietnamese.
var languages = []struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}{{"vi", "Tiếng Việt"}, {"en", "Tiếng Anh"}, {"es", "Tiếng Tây Ban Nha"}, {"fr", "Tiếng Pháp"}}

// Chinese, Japanese and Korean are left out until the video has CJK fonts and per-character
// wipe tokens: the karaoke fonts have no CJK glyphs and lyrics split on spaces.

var themeKaraoke = map[string]string{"truyen-thong": "pink", "hien-dai": "blue", "co-dien": "gold"}

func findGenre(id string) *Genre {
	for i := range genres {
		if genres[i].ID == id {
			return &genres[i]
		}
	}
	return nil
}

func validLanguage(id string) bool {
	for _, l := range languages {
		if l.ID == id {
			return true
		}
	}
	return false
}

// SongMeta is what the UI may change in project.json without the full editor.
type SongMeta struct {
	Genre    *string `json:"genre"`
	Language *string `json:"language"`
	Theme    *string `json:"videoTheme"`
}

// SetMeta merges genre, language and video theme into project.json. Choosing a genre also
// picks its video theme and karaoke colour, unless a theme is given in the same call.
func (p Projects) SetMeta(slug string, m SongMeta) error {
	dir, err := p.dir(slug)
	if err != nil {
		return err
	}
	// never rebuild project.json from nothing: an empty read means another writer is mid-write
	raw, err := os.ReadFile(filepath.Join(dir, "project.json"))
	cfg := map[string]any{}
	if err != nil || len(raw) == 0 || json.Unmarshal(raw, &cfg) != nil || len(cfg) == 0 {
		return fmt.Errorf("%w: project.json đang trống hoặc đang được ghi, thử lại sau", errConflict)
	}
	if m.Genre != nil {
		g := findGenre(*m.Genre)
		if g == nil && *m.Genre != "" {
			return errors.New("thể loại không hợp lệ")
		}
		// follow the new genre's look only while the song still has the old genre's default,
		// so a theme the user picked in Xem trước is kept
		old, _ := cfg["genre"].(string)
		cur, _ := cfg["video_theme"].(string)
		oldDefault := "truyen-thong"
		if og := findGenre(old); og != nil {
			oldDefault = og.Theme
		}
		cfg["genre"] = *m.Genre
		if g != nil && m.Theme == nil && (cur == "" || cur == oldDefault) {
			cfg["video_theme"], cfg["karaoke_color"] = g.Theme, themeKaraoke[g.Theme]
		}
	}
	if m.Language != nil {
		if !validLanguage(*m.Language) {
			return errors.New("ngôn ngữ không hợp lệ")
		}
		cfg["language"] = *m.Language
	}
	if m.Theme != nil {
		if _, ok := themeKaraoke[*m.Theme]; !ok {
			return errors.New("kiểu video không hợp lệ")
		}
		cfg["video_theme"], cfg["karaoke_color"] = *m.Theme, themeKaraoke[*m.Theme]
	}
	return writeJSONAtomic(filepath.Join(dir, "project.json"), cfg, 0o644)
}

// genreLine tells an agent what kind of song this is, in the prompt's language.
func genreLine(cfg projectConfig) string {
	lang := "tiếng Việt"
	for _, l := range languages {
		if l.ID == cfg.Language {
			lang = l.Label
		}
	}
	if g := findGenre(cfg.Genre); g != nil {
		return "Thể loại: " + g.Label + ". Ngôn ngữ hát: " + lang + "."
	}
	return "Thể loại: chưa chọn (suy ra từ bài mẫu và yêu cầu). Ngôn ngữ hát: " + lang + "."
}
