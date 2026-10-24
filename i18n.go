package main

import "sync/atomic"

// The studio speaks Vietnamese or English (Config.Language). Server-made text the UI shows,
// such as step titles, errors and machine checks, goes through tr with both versions inline.

var uiLang atomic.Value // "vi" or "en"

var uiLanguages = []string{"vi", "en"}

func setUILang(l string) { uiLang.Store(l) }

// tr returns en when the studio is set to English, otherwise vi.
func tr(vi, en string) string {
	if l, _ := uiLang.Load().(string); l == "en" {
		return en
	}
	return vi
}

// trError is a sentinel error whose message follows the studio language when it is shown,
// so a package-level error still matches with errors.Is after the language changes.
type trError struct{ vi, en string }

func (e *trError) Error() string { return tr(e.vi, e.en) }

func trErr(vi, en string) error { return &trError{vi, en} }
