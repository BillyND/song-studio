package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// keychainService names the studio's items in the login keychain.
const keychainService = "que-huong-studio"

// secretNames are the only keys the studio stores; each maps to an image provider.
var secretNames = []string{"image.hub", "image.openai", "image.gemini"}

var errNoSecret = errors.New("chưa có khoá")

// SecretStore keeps API keys out of config files, logs and process arguments.
type SecretStore interface {
	Get(name string) (string, error)
	Set(name, value string) error
	Delete(name string) error
}

// keychainStore uses the macOS `security` tool. Values travel on stdin (hex encoded),
// never in argv, so `ps` and the run history cannot show them.
type keychainStore struct{ bin string }

func newKeychainStore() keychainStore { return keychainStore{bin: "/usr/bin/security"} }

func (k keychainStore) Get(name string) (string, error) {
	out, err := exec.Command(k.bin, "find-generic-password", "-s", keychainService, "-a", name, "-w").Output()
	if err != nil {
		return "", errNoSecret
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func (k keychainStore) Set(name, value string) error {
	cmd := exec.Command(k.bin, "-i")
	cmd.Stdin = strings.NewReader(fmt.Sprintf("add-generic-password -U -s %s -a %s -l %s -X %s\n",
		keychainService, name, keychainService+"-"+name, hex.EncodeToString([]byte(value))))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil || stderr.Len() > 0 {
		return fmt.Errorf("không lưu được vào Keychain: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func (k keychainStore) Delete(name string) error {
	if err := exec.Command(k.bin, "delete-generic-password", "-s", keychainService, "-a", name).Run(); err != nil {
		return errNoSecret
	}
	return nil
}

// memStore is the test double.
type memStore struct {
	mu sync.Mutex
	m  map[string]string
}

func (s *memStore) Get(name string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.m[name]; ok {
		return v, nil
	}
	return "", errNoSecret
}

func (s *memStore) Set(name, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[string]string{}
	}
	s.m[name] = value
	return nil
}

func (s *memStore) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[name]; !ok {
		return errNoSecret
	}
	delete(s.m, name)
	return nil
}

// SecretInfo is what the UI may see about a key: whether it is set and its last 4 characters.
type SecretInfo struct {
	Name string `json:"name"`
	Set  bool   `json:"set"`
	Hint string `json:"hint,omitempty"`
}

func secretInfos(store SecretStore) []SecretInfo {
	out := []SecretInfo{}
	for _, n := range secretNames {
		info := SecretInfo{Name: n}
		if v, err := store.Get(n); err == nil && v != "" {
			info.Set = true
			if len(v) > 8 {
				info.Hint = "…" + v[len(v)-4:]
			}
		}
		out = append(out, info)
	}
	return out
}

// legacyHubKey is where the skill kept the image gateway key before the studio had a keychain.
func legacyHubKey(home string) string {
	return filepath.Join(home, ".config", "suno-que-huong", "aihub.key")
}

func readLegacyHubKey(home string) string {
	b, err := os.ReadFile(legacyHubKey(home))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// imageEnv is the environment the scene-image script gets, and only that script.
func imageEnv(cfg Config, store SecretStore) []string {
	im := cfg.Image
	env := []string{
		"QH_IMAGE_PROVIDER=" + im.Provider,
		"QH_IMAGE_MODEL=" + im.Models[im.Provider],
		"QH_IMAGE_SIZE=" + im.Size,
		"QH_IMAGE_QUALITY=" + im.Quality,
	}
	if im.Provider == "hub" {
		if !validBaseURL(im.BaseURL) {
			return env // no key for an address the user did not validly choose; the script reports it
		}
		env = append(env, "QH_IMAGE_BASE_URL="+im.BaseURL)
	}
	if key, err := store.Get("image." + im.Provider); err == nil && key != "" {
		env = append(env, "QH_IMAGE_API_KEY="+key)
	}
	return env
}

// checkImageKey makes a free, read-only call (listing models) to tell whether a key works.
func checkImageKey(cfg Config, provider, key string, client *http.Client) error {
	var req *http.Request
	switch provider {
	case "hub", "openai":
		base := "https://api.openai.com/v1"
		if provider == "hub" {
			if !validBaseURL(cfg.Image.BaseURL) {
				return errors.New("địa chỉ API ảnh (OpenAI-compatible) không hợp lệ")
			}
			base = strings.TrimRight(cfg.Image.BaseURL, "/")
		}
		req, _ = http.NewRequest("GET", base+"/models", nil)
		req.Header.Set("Authorization", "Bearer "+key)
	case "gemini":
		req, _ = http.NewRequest("GET", "https://generativelanguage.googleapis.com/v1beta/models?pageSize=1", nil)
		req.Header.Set("x-goog-api-key", key)
	default:
		return fmt.Errorf("nguồn ảnh không hỗ trợ: %s", provider)
	}
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("không kết nối được: %v", err)
	}
	res.Body.Close()
	switch {
	case res.StatusCode == 200:
		return nil
	case res.StatusCode == 401 || res.StatusCode == 403:
		return fmt.Errorf("khoá bị từ chối (%d)", res.StatusCode)
	default:
		return fmt.Errorf("máy chủ trả %d", res.StatusCode)
	}
}
