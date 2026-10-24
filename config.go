package main

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
)

// AgentConfig describes one coding CLI the studio can spawn.
type AgentConfig struct {
	Bin   string `json:"bin"`
	Model string `json:"model"`
}

// Config is persisted at ~/.config/que-huong-studio/config.json.
type Config struct {
	Addr           string                 `json:"addr"`
	SkillDir       string                 `json:"skillDir"`
	ProjectsDir    string                 `json:"projectsDir"`
	DefaultAgent   string                 `json:"defaultAgent"`
	PermissionMode string                 `json:"permissionMode"`       // "full" or "edits"
	MaxConcurrent  int                    `json:"maxConcurrent"`        // heavy jobs at once (CPU-bound scripts)
	MaxAgents      int                    `json:"maxAgents"`            // light jobs at once (agents, image downloads)
	Credits        [][2]string            `json:"credits"`              // prefilled on new songs
	ImageModel     string                 `json:"imageModel,omitempty"` // before Image existed; migrated into Image.Models["hub"]
	Image          ImageConfig            `json:"image"`
	Language       string                 `json:"language"` // UI language: "vi" (default) or "en"
	Agents         map[string]AgentConfig `json:"agents"`
	Presets        []Preset               `json:"presets"`
}

// ImageConfig picks the scene-image provider. Keys live in the keychain, never here.
type ImageConfig struct {
	Provider string            `json:"provider"` // hub, openai or gemini
	BaseURL  string            `json:"baseUrl"`  // the hub's OpenAI-compatible endpoint
	Models   map[string]string `json:"models"`   // model id per provider
	Size     string            `json:"size"`     // WxH for OpenAI-shaped APIs
	Quality  string            `json:"quality"`
}

var imageProviders = []string{"hub", "openai", "gemini"}

var defaultImage = ImageConfig{
	Provider: "hub",
	BaseURL:  "https://aihub.tigrendev.com/v1",
	Models:   map[string]string{"hub": "gpt-image-2.5", "openai": "gpt-image-2.5-flare", "gemini": "gemini-nano-banana-2.1"},
	Size:     "2048x1152",
	Quality:  "high",
}

var imageSize = regexp.MustCompile(`^[1-9][0-9]{2,3}x[1-9][0-9]{2,3}$`)

// Preset is a reusable instruction snippet the UI can append to any agent request.
type Preset struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}

// defaultPresets are genre-neutral; each genre adds its own suggestions (genres.go).
// Keyed by studio language: a list still equal to one language's defaults follows the language.
var defaultPresets = map[string][]Preset{
	"vi": {
		{"Không bộ gõ", "Không dùng bộ gõ: không trống, không cajon, không hi-hat, không shaker; nhịp do đàn và câu hát dẫn dắt."},
		{"Giọng nữ ấm", "Giọng nữ trong, ấm, nhả chữ rõ."},
		{"Giọng nam ấm", "Giọng nam trung ấm, tròn, nhả chữ rõ."},
		{"Điệp khúc bùng nổ", "Verse nhẹ, điệp khúc dày và bùng nổ, điệp khúc cuối lên cao trào."},
		{"Học phong cách, không chép", "Chỉ học phong cách từ bài mẫu (tiết tấu, dàn nhạc, màu giọng). Không chép giai điệu, lời hay câu nhạc đặc trưng của bài mẫu."},
	},
	"en": {
		{"No percussion", "No percussion: no drums, no cajon, no hi-hat, no shaker; the instruments and the vocal carry the rhythm."},
		{"Warm female vocal", "A clear, warm female vocal with crisp diction."},
		{"Warm male vocal", "A warm, rounded baritone male vocal with crisp diction."},
		{"Explosive chorus", "Light verses, a dense and explosive chorus, the final chorus rising to a climax."},
		{"Learn the style, don't copy", "Only learn the style of the reference songs (rhythm, arrangement, vocal colour). Do not copy their melody, lyrics or signature phrases."},
	},
}

// defaultCredits prefill the credit lines of a new song, keyed like defaultPresets.
var defaultCredits = map[string][][2]string{
	"vi": {{"Sáng tác: ", ""}, {"Phổ nhạc: ", ""}},
	"en": {{"Written by: ", ""}, {"Music by: ", ""}},
}

// localDefault returns the defaults for lang when cur is still one language's untouched
// defaults, so switching the studio language swaps them; edited lists are kept as they are.
func localDefault[T comparable](cur []T, defaults map[string][]T, lang string) []T {
	for _, d := range defaults {
		if len(cur) > 0 && slices.Equal(cur, d) {
			return slices.Clone(defaults[lang])
		}
	}
	return cur
}

var agentNames = []string{"claude", "omp", "codex"}

func defaultConfig(home string) Config {
	return Config{
		Addr:           "127.0.0.1:4317",
		SkillDir:       filepath.Join(home, ".claude", "skills", "suno-que-huong"),
		ProjectsDir:    filepath.Join(home, "Music", "suno-que-huong"),
		DefaultAgent:   "claude",
		PermissionMode: "full",
		MaxConcurrent:  1,
		MaxAgents:      3,
		// Image is filled by normalizeImage: sharing defaultImage.Models here would let
		// json.Unmarshal write a user's models into the package default. Credits and
		// Presets are cloned for the same reason.
		Credits: slices.Clone(defaultCredits["vi"]),
		Presets: slices.Clone(defaultPresets["vi"]),
		Agents: map[string]AgentConfig{
			"claude": {Bin: firstExisting("/opt/homebrew/bin/claude", "claude"), Model: ""},
			"omp":    {Bin: firstExisting(filepath.Join(home, ".local", "bin", "omp"), "omp"), Model: ""},
			"codex":  {Bin: firstExisting("/opt/homebrew/bin/codex", "codex"), Model: ""},
		},
	}
}

func firstExisting(paths ...string) string {
	for _, p := range paths {
		if filepath.IsAbs(p) {
			if _, err := os.Stat(p); err == nil {
				return p
			}
			continue
		}
		return p
	}
	return paths[len(paths)-1]
}

// ConfigStore guards the live config and writes it atomically.
type ConfigStore struct {
	mu   sync.RWMutex
	path string
	cfg  Config
}

func loadConfig(path, home string) (*ConfigStore, error) {
	cfg := defaultConfig(home)
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	normalizeConfig(&cfg, home)
	setUILang(cfg.Language)
	return &ConfigStore{path: path, cfg: cfg}, nil
}

func normalizeConfig(cfg *Config, home string) {
	def := defaultConfig(home)
	if cfg.Addr == "" {
		cfg.Addr = def.Addr
	}
	if cfg.SkillDir == "" {
		cfg.SkillDir = def.SkillDir
	}
	if cfg.ProjectsDir == "" {
		cfg.ProjectsDir = def.ProjectsDir
	}
	if cfg.PermissionMode != "edits" {
		cfg.PermissionMode = "full"
	}
	if cfg.MaxConcurrent < 1 {
		cfg.MaxConcurrent = 1
	}
	if cfg.MaxConcurrent > 4 {
		cfg.MaxConcurrent = 4
	}
	if cfg.MaxAgents < 1 {
		cfg.MaxAgents = def.MaxAgents
	}
	if cfg.MaxAgents > 6 {
		cfg.MaxAgents = 6
	}
	if !slices.Contains(uiLanguages, cfg.Language) {
		cfg.Language = "vi"
	}
	if len(cfg.Credits) == 0 {
		cfg.Credits = slices.Clone(defaultCredits[cfg.Language])
	}
	cfg.Credits = localDefault(cfg.Credits, defaultCredits, cfg.Language)
	normalizeImage(&cfg.Image, cfg.ImageModel)
	cfg.ImageModel = ""
	if cfg.Agents == nil {
		cfg.Agents = map[string]AgentConfig{}
	}
	if cfg.Presets == nil {
		cfg.Presets = slices.Clone(defaultPresets[cfg.Language])
	}
	cfg.Presets = localDefault(cfg.Presets, defaultPresets, cfg.Language)
	kept := cfg.Presets[:0:0]
	for _, p := range cfg.Presets {
		if p.Label != "" && p.Text != "" {
			kept = append(kept, p)
		}
	}
	cfg.Presets = kept
	for _, n := range agentNames {
		a := cfg.Agents[n]
		if a.Bin == "" {
			a.Bin = def.Agents[n].Bin
		}
		cfg.Agents[n] = a
	}
	if _, ok := cfg.Agents[cfg.DefaultAgent]; !ok || !contains(agentNames, cfg.DefaultAgent) {
		cfg.DefaultAgent = "claude"
	}
}

func (s *ConfigStore) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c := s.cfg
	c.Agents = map[string]AgentConfig{}
	for k, v := range s.cfg.Agents {
		c.Agents[k] = v
	}
	return c
}

// Update replaces editable fields; Addr changes apply on next start.
func (s *ConfigStore) Update(next Config, home string) (Config, error) {
	normalizeConfig(&next, home)
	if !validBaseURL(next.Image.BaseURL) {
		return Config{}, errors.New(tr("địa chỉ API ảnh (OpenAI-compatible) không hợp lệ: dùng https://…, hoặc http:// chỉ với 127.0.0.1 hay localhost", "invalid image API address (OpenAI-compatible): use https://…, or http:// only with 127.0.0.1 or localhost"))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = next
	if err := writeJSONAtomic(s.path, s.cfg, 0o600); err != nil {
		return Config{}, err
	}
	setUILang(next.Language)
	return s.cfg, nil
}

func writeJSONAtomic(path string, v any, mode os.FileMode) error {
	data, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, mode)
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// validBaseURL accepts https to any host, or plain http only to this machine (a local gateway).
// It refuses user info ("http://127.0.0.1@evil.example") and look-alike hosts.
func validBaseURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil || u.User != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	switch u.Scheme {
	case "https":
		return true
	case "http":
		h := u.Hostname()
		return h == "127.0.0.1" || h == "localhost" || h == "::1"
	}
	return false
}

func normalizeImage(im *ImageConfig, legacyModel string) {
	if !slices.Contains(imageProviders, im.Provider) {
		im.Provider = defaultImage.Provider
	}
	im.BaseURL = strings.TrimSpace(im.BaseURL)
	if im.BaseURL == "" {
		im.BaseURL = defaultImage.BaseURL
	}
	// an invalid URL is kept as typed: Update refuses it, and imageEnv sends no key to it
	models := map[string]string{}
	for _, p := range imageProviders {
		models[p] = strings.TrimSpace(im.Models[p])
		if models[p] == "" {
			models[p] = defaultImage.Models[p]
		}
	}
	if legacyModel != "" && strings.TrimSpace(im.Models["hub"]) == "" {
		models["hub"] = legacyModel
	}
	im.Models = models
	if !imageSize.MatchString(im.Size) {
		im.Size = defaultImage.Size
	}
	if !slices.Contains([]string{"low", "medium", "high"}, im.Quality) {
		im.Quality = defaultImage.Quality
	}
}
