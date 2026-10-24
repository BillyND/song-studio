package main

import (
	"encoding/json"
	"errors"
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

var defaultPresets = []Preset{
	{"Không bộ gõ", "Không dùng bộ gõ: không trống, không cajon, không hi-hat, không shaker; nhịp do đàn và câu hát dẫn dắt."},
	{"Dân ca mượt mà", "Phong cách dân ca mượt mà: câu hát liền mạch, luyến láy vừa phải ở cuối câu, không ề à, không kéo lê, tempo vừa phải."},
	{"Chất chèo Bắc Bộ", "Hơi hướng làn điệu chèo, dân ca đồng bằng Bắc Bộ (quê Ninh Bình) nhưng hiện đại, dễ nghe."},
	{"Đàn bầu dẫn", "Đàn bầu dẫn giai điệu dạo và gian tấu, sáo trúc và đàn tranh đệm, dây nhẹ phía sau."},
	{"Giọng nữ ngọt", "Giọng nữ trong, ngọt, luyến láy dân ca nhẹ nhàng."},
	{"Giọng nam ấm", "Giọng nam trung ấm, tròn, nhả chữ rõ."},
	{"Học phong cách, không chép", "Chỉ học phong cách từ bài mẫu (tiết tấu, chất vùng miền, dàn nhạc). Không chép giai điệu, lời hay câu nhạc đặc trưng của bài mẫu."},
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
		Credits:        [][2]string{{"Sáng tác: ", ""}, {"Phổ nhạc: ", ""}},
		// Image is filled by normalizeImage: sharing defaultImage.Models here would let
		// json.Unmarshal write a user's models into the package default
		Presets: defaultPresets,
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
	if len(cfg.Credits) == 0 {
		cfg.Credits = def.Credits
	}
	normalizeImage(&cfg.Image, cfg.ImageModel)
	cfg.ImageModel = ""
	if cfg.Agents == nil {
		cfg.Agents = map[string]AgentConfig{}
	}
	if cfg.Presets == nil {
		cfg.Presets = defaultPresets
	}
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = next
	if err := writeJSONAtomic(s.path, s.cfg, 0o600); err != nil {
		return Config{}, err
	}
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

func normalizeImage(im *ImageConfig, legacyModel string) {
	if !slices.Contains(imageProviders, im.Provider) {
		im.Provider = defaultImage.Provider
	}
	// plain http only to this machine (a local gateway); anything else must be https
	local := strings.HasPrefix(im.BaseURL, "http://127.0.0.1") || strings.HasPrefix(im.BaseURL, "http://localhost")
	if !strings.HasPrefix(im.BaseURL, "https://") && !local {
		im.BaseURL = defaultImage.BaseURL
	}
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
