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

// genreList builds the genres in the studio language at request time; IDs, themes and
// image styles are the same in both languages.
func genreList() []Genre {
	return []Genre{
		{ID: "que-huong", Label: tr("Quê hương, dân ca", "Homeland, folk"), Theme: "truyen-thong",
			Presets: []Preset{
				{tr("Dân ca mượt mà", "Smooth folk"), tr("Phong cách dân ca mượt mà: câu hát liền mạch, luyến láy vừa phải ở cuối câu, không ề à, tempo vừa phải.", "Smooth folk style: seamless phrases, moderate ornaments at line ends, no dragging, medium tempo.")},
				{tr("Nhạc cụ dân tộc", "Traditional instruments"), tr("Sáo trúc, đàn tranh, đàn bầu dẫn giai điệu; dây nhẹ phía sau.", "Bamboo flute, đàn tranh zither and đàn bầu monochord lead the melody; light strings behind.")},
				{tr("Không bộ gõ", "No percussion"), tr("Không dùng bộ gõ: không trống, không cajon, không hi-hat; nhịp do đàn và câu hát dẫn dắt.", "No percussion: no drums, no cajon, no hi-hat; the instruments and the vocal carry the rhythm.")},
			},
			ImageStyle: "Photorealistic cinematic photograph of Vietnamese countryside and landmarks, warm golden-hour light, rich greens, natural Vietnamese faces and clothing."},
		{ID: "bolero", Label: tr("Bolero, trữ tình", "Bolero, sentimental"), Theme: "co-dien",
			Presets: []Preset{
				{tr("Bolero chuẩn", "Classic bolero"), tr("Nhịp bolero chậm, guitar phím lõm và bass đệm theo tiết tấu bolero, dàn dây nhẹ; giọng hát tình cảm, nhả chữ rõ.", "Slow bolero rhythm, scalloped-fret guitar and bass on the bolero pattern, light strings; heartfelt vocal with clear diction.")},
				{tr("Trữ tình sang trọng", "Elegant sentimental"), tr("Phối trữ tình sang trọng: piano, dàn dây, saxophone dạo; không điện tử.", "Elegant sentimental arrangement: piano, strings, saxophone fills; no electronics.")},
			},
			ImageStyle: "Nostalgic cinematic photograph, soft warm film tones, gentle rain or dusk light, quiet streets, rivers and old houses, emotional and tender mood."},
		{ID: "ballad", Label: tr("Ballad, nhạc nhẹ", "Ballad, easy listening"), Theme: "hien-dai",
			Presets: []Preset{
				{"Piano ballad", tr("Piano dẫn, dây vào ở điệp khúc, trống nhẹ từ verse 2; cao trào ở điệp khúc cuối.", "Piano leads, strings enter at the chorus, light drums from verse 2; climax on the final chorus.")},
				{tr("Acoustic ấm", "Warm acoustic"), tr("Guitar mộc và piano, không điện tử, giọng hát gần và ấm.", "Acoustic guitar and piano, no electronics, a close and warm vocal.")},
			},
			ImageStyle: "Cinematic photograph with soft natural light, shallow depth of field, intimate emotional moments, muted pastel colors."},
		{ID: "pop", Label: tr("Pop, nhạc trẻ", "Pop, V-pop"), Theme: "hien-dai",
			Presets: []Preset{
				{tr("Pop hiện đại", "Modern pop"), tr("Pop hiện đại, nhịp vừa nhanh, hook điệp khúc dễ nhớ, synth và trống điện tử sạch.", "Modern pop, upbeat medium tempo, a catchy chorus hook, clean synths and electronic drums.")},
				{tr("V-pop sáng", "Bright V-pop"), tr("Màu V-pop tươi sáng, điệp khúc bắt tai, pre-chorus dồn lên.", "Bright V-pop colour, a catchy chorus, a pre-chorus that builds up.")},
			},
			ImageStyle: "Vibrant modern photograph, bold colors, city lights and youthful energy, clean composition."},
		{ID: "rock", Label: "Rock", Theme: "hien-dai",
			Presets: []Preset{
				{tr("Rock ban nhạc", "Band rock"), tr("Ban nhạc rock: guitar điện distortion, bass, trống mạnh; giọng hát khoẻ.", "Rock band: distorted electric guitar, bass, hard-hitting drums; a powerful vocal.")},
				{"Pop rock", tr("Pop rock dễ nghe, guitar điện sáng, điệp khúc bùng nổ.", "Easy-listening pop rock, bright electric guitar, an explosive chorus.")},
			},
			ImageStyle: "High-contrast cinematic photograph, dramatic light, stage lights or wild landscapes, gritty texture."},
		{ID: "rap", Label: "Rap, hip-hop", Theme: "hien-dai",
			Presets: []Preset{
				{"Hip-hop", tr("Beat hip-hop rõ, kick và snare chắc, 808 bass; flow rap rõ chữ, điệp khúc hát.", "A clear hip-hop beat, solid kick and snare, 808 bass; a crisp rap flow with a sung chorus.")},
				{"Lo-fi chill", tr("Lo-fi hip-hop thư giãn, piano lo-fi, trống nhẹ có swing.", "Relaxed lo-fi hip-hop, lo-fi piano, soft swung drums.")},
			},
			ImageStyle: "Urban cinematic photograph, night city, neon and street details, strong contrast."},
		{ID: "edm", Label: "EDM, dance", Theme: "hien-dai",
			Presets: []Preset{
				{tr("EDM sôi động", "Energetic EDM"), tr("EDM sôi động, build-up và drop rõ ràng, synth lead sáng, nhịp four-on-the-floor.", "Energetic EDM, clear build-ups and drops, a bright synth lead, four-on-the-floor beat.")},
			},
			ImageStyle: "Energetic photograph with colorful lights, motion and celebration, glowing night atmosphere."},
		{ID: "acoustic", Label: "Acoustic, indie", Theme: "hien-dai",
			Presets: []Preset{
				{tr("Indie mộc", "Unplugged indie"), tr("Indie mộc: guitar thùng, ukulele hoặc piano, trống cajon nhẹ; giọng hát tự nhiên.", "Unplugged indie: acoustic guitar, ukulele or piano, light cajon; a natural vocal.")},
			},
			ImageStyle: "Natural film photograph, soft daylight, simple everyday scenes, cozy and honest mood."},
		{ID: "thieu-nhi", Label: tr("Thiếu nhi", "Children's songs"), Theme: "truyen-thong",
			Presets: []Preset{
				{tr("Vui tươi", "Cheerful"), tr("Vui tươi, nhịp nhảy, nhạc cụ sáng (xylophone, ukulele, piano), giai điệu dễ hát theo.", "Cheerful, bouncy rhythm, bright instruments (xylophone, ukulele, piano), an easy sing-along melody.")},
			},
			ImageStyle: "Bright cheerful photograph or soft 3D illustration style, playful colors, happy children and friendly animals."},
		{ID: "thanh-ca", Label: tr("Thánh ca, nhạc đạo", "Hymns, sacred music"), Theme: "co-dien",
			Presets: []Preset{
				{tr("Trang nghiêm", "Solemn"), tr("Trang nghiêm, organ hoặc piano, dàn dây và hợp xướng nhẹ, tempo chậm.", "Solemn, organ or piano, strings and a soft choir, slow tempo.")},
			},
			ImageStyle: "Serene cinematic photograph, soft divine light, candles, churches or temples, peaceful nature."},
		{ID: "cach-mang", Label: tr("Nhạc cách mạng", "Revolutionary songs"), Theme: "co-dien",
			Presets: []Preset{
				{tr("Hào hùng", "Heroic"), tr("Hào hùng, hành khúc, kèn đồng và trống quân đội, dàn dây; hợp xướng ở điệp khúc.", "Heroic march, brass and military drums, strings; a choir on the chorus.")},
			},
			ImageStyle: "Epic cinematic photograph, flags, crowds and historic landmarks, golden light, proud mood."},
	}
}

// Language is a sung language karaoke alignment knows.
type Language struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// languageList names the sung languages in the studio language.
func languageList() []Language {
	return []Language{
		{"vi", tr("Tiếng Việt", "Vietnamese")},
		{"en", tr("Tiếng Anh", "English")},
		{"es", tr("Tiếng Tây Ban Nha", "Spanish")},
		{"fr", tr("Tiếng Pháp", "French")},
	}
}

// Chinese, Japanese and Korean are left out until the video has CJK fonts and per-character
// wipe tokens: the karaoke fonts have no CJK glyphs and lyrics split on spaces.

var themeKaraoke = map[string]string{"truyen-thong": "pink", "hien-dai": "blue", "co-dien": "gold"}

func findGenre(id string) *Genre {
	gs := genreList()
	for i := range gs {
		if gs[i].ID == id {
			return &gs[i]
		}
	}
	return nil
}

func validLanguage(id string) bool {
	for _, l := range languageList() {
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
		return fmt.Errorf(tr("%w: project.json đang trống hoặc đang được ghi, thử lại sau", "%w: project.json is empty or being written, try again shortly"), errConflict)
	}
	if m.Genre != nil {
		g := findGenre(*m.Genre)
		if g == nil && *m.Genre != "" {
			return errors.New(tr("thể loại không hợp lệ", "invalid genre"))
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
			return errors.New(tr("ngôn ngữ không hợp lệ", "invalid language"))
		}
		cfg["language"] = *m.Language
	}
	if m.Theme != nil {
		if _, ok := themeKaraoke[*m.Theme]; !ok {
			return errors.New(tr("kiểu video không hợp lệ", "invalid video style"))
		}
		cfg["video_theme"], cfg["karaoke_color"] = *m.Theme, themeKaraoke[*m.Theme]
	}
	return writeJSONAtomic(filepath.Join(dir, "project.json"), cfg, 0o644)
}

// genreLine tells an agent what kind of song this is, in the studio language.
func genreLine(cfg projectConfig) string {
	lang := tr("tiếng Việt", "Vietnamese")
	for _, l := range languageList() {
		if l.ID == cfg.Language {
			lang = l.Label
		}
	}
	if g := findGenre(cfg.Genre); g != nil {
		return fmt.Sprintf(tr("Thể loại: %s. Ngôn ngữ hát: %s.", "Genre: %s. Sung language: %s."), g.Label, lang)
	}
	return fmt.Sprintf(tr("Thể loại: chưa chọn (suy ra từ bài mẫu và yêu cầu). Ngôn ngữ hát: %s.", "Genre: not chosen (infer it from the references and the brief). Sung language: %s."), lang)
}
