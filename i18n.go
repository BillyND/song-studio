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
