package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// useEnglish switches the studio language for one test and puts Vietnamese back afterwards.
func useEnglish(t *testing.T) {
	t.Helper()
	setUILang("en")
	t.Cleanup(func() { setUILang("vi") })
}

func TestEnglishStudioText(t *testing.T) {
	home := t.TempDir()
	cs, err := loadConfig(filepath.Join(home, "c.json"), home) // loading sets the language, so switch after
	if err != nil {
		t.Fatal(err)
	}
	useEnglish(t)

	steps := deriveSteps(t.TempDir(), projectConfig{}, ProjectDetail{}, nil)
	if steps[0].Title != "Lyrics, references & brief" || steps[0].Detail != "No lyrics yet" {
		t.Fatalf("step text should be English: %+v", steps[0])
	}

	m, _ := newManager(t.TempDir(), func() (int, int) { return 1, 1 })
	defer m.Shutdown(time.Second)
	store := &memStore{}
	store.Set("image.hub", "hub-secret-key-777777")
	app := &App{cfg: cs, runs: m, projects: Projects{cfg: cs}, secrets: store, token: "tok", home: home, version: "t", build: "b", started: time.Now()}
	checks := map[string]Check{}
	for _, c := range app.doctor() {
		checks[c.ID] = c
	}
	if c := checks["imagekey"]; c.Label != "Image API key" || c.Detail != "OpenAI-compatible (Keychain)" {
		t.Fatalf("image key check should be English and name the provider for people: %+v", c)
	}
	if c := checks["skill"]; c.Fix != "Fix the Skill folder in Settings" {
		t.Fatalf("skill fix should be English: %+v", c)
	}

	if g := findGenre("bolero"); g == nil || g.Label != "Bolero, sentimental" || g.Theme != "co-dien" {
		t.Fatalf("genre label should be English with the same theme: %+v", g)
	}
	if l := languageList()[0]; l.ID != "vi" || l.Label != "Vietnamese" {
		t.Fatalf("language label should be English: %+v", l)
	}
	if acts := workflowActions(); acts[0].ID != "ref.analyze" || acts[0].Label != "Analyze references" {
		t.Fatalf("action label should be English: %+v", acts[0])
	}
	if got := statusLabel("succeeded"); got != "Succeeded" {
		t.Fatalf("status label: %q", got)
	}
	if !strings.Contains(projectPromptHeader("/s", "/p"), "in English") {
		t.Fatal("agents should be told to write user-facing notes in English")
	}

	// sentinel errors read in the current language and still match
	wrapped := fmt.Errorf("save: %w", errConflict)
	if !errors.Is(wrapped, errConflict) || errConflict.Error() != "file changed on disk since it was opened" {
		t.Fatalf("sentinel error: %v", wrapped)
	}
}

func TestVietnameseStaysDefault(t *testing.T) {
	steps := deriveSteps(t.TempDir(), projectConfig{}, ProjectDetail{}, nil)
	if steps[0].Title != "Lời, bài mẫu & yêu cầu" {
		t.Fatalf("default language should be Vietnamese: %q", steps[0].Title)
	}
	if strings.Contains(projectPromptHeader("/s", "/p"), "English") {
		t.Fatal("a Vietnamese studio should not ask agents for English")
	}
	if g := findGenre("bolero"); g.Label != "Bolero, trữ tình" {
		t.Fatalf("genre label: %q", g.Label)
	}
}

func TestDefaultsFollowStudioLanguage(t *testing.T) {
	fresh := Config{Language: "en"}
	normalizeConfig(&fresh, "/h")
	if !slices.Equal(fresh.Presets, defaultPresets["en"]) || !slices.Equal(fresh.Credits, defaultCredits["en"]) {
		t.Fatalf("an English studio starts with English defaults: %+v %+v", fresh.Presets, fresh.Credits)
	}
	// switching language swaps untouched defaults but keeps edited lists
	untouched := Config{Language: "en", Presets: slices.Clone(defaultPresets["vi"]), Credits: slices.Clone(defaultCredits["vi"])}
	normalizeConfig(&untouched, "/h")
	if !slices.Equal(untouched.Presets, defaultPresets["en"]) || !slices.Equal(untouched.Credits, defaultCredits["en"]) {
		t.Fatalf("untouched defaults should follow the language: %+v", untouched.Presets)
	}
	edited := Config{Language: "en", Presets: []Preset{{"Mine", "my text"}}, Credits: [][2]string{{"Lời: ", "Tôi"}}}
	normalizeConfig(&edited, "/h")
	if len(edited.Presets) != 1 || edited.Presets[0].Label != "Mine" || edited.Credits[0][1] != "Tôi" {
		t.Fatalf("edited lists must be kept: %+v %+v", edited.Presets, edited.Credits)
	}
	emptied := Config{Language: "en", Presets: []Preset{}}
	normalizeConfig(&emptied, "/h")
	if len(emptied.Presets) != 0 {
		t.Fatalf("a preset list the user emptied stays empty: %+v", emptied.Presets)
	}
	// the package defaults are never shared with a config, so decoding into one cannot change them
	def := defaultConfig("/h")
	def.Presets[0].Label = "changed"
	if defaultPresets["vi"][0].Label == "changed" {
		t.Fatal("defaultConfig must not alias the package presets")
	}
}

func TestKeyNameEn(t *testing.T) {
	for in, want := range map[string]string{"g minor": "G minor", "b- major": "B♭ major", "f# minor": "F♯ minor", "": ""} {
		if got := keyNameEn(in); got != want {
			t.Errorf("keyNameEn(%q) = %q, want %q", in, got, want)
		}
	}
}
