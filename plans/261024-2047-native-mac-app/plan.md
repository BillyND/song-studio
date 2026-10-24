# Native macOS app with auto-update

Status: done (2026-10-24); verified 1.0.8 → 1.0.9 auto-update with relaunch. Decision: Swift shell + existing Go server; Sparkle 2.10 for updates; releases in a separate public repo `BillyND/que-huong-studio-releases` (source stays private). Unsigned (ad-hoc), friends-only.

## Outcome
Opening "Quê Hương Studio" shows its own window (no browser). When a new version is released, every installed copy finds it, installs it and relaunches into it.

## Constraints
- Go server and web UI stay as they are; the Swift part only hosts the window, menus, server lifecycle and Sparkle.
- Local-only server on 127.0.0.1:4317; keys stay in the keychain.
- No Apple signing; updates verified with Sparkle's EdDSA key (private key in the login keychain of the releasing Mac).
- Quitting or updating must not lose queued work (server shutdown already re-queues).

## Non-goals
App Store, Developer ID notarization, Windows/Linux, delta updates, in-app notifications.

## Acceptance
1. App opens its own window with the studio; Cmd+C/V/X/A/Z, Cmd+Q, Cmd+R work; `confirm()`, file pickers, drag-and-drop, downloads and external links work.
2. Quitting stops the server cleanly (queued runs survive); "Tắt studio" in the UI quits the app; an older studio already on the port is replaced.
3. `make release VERSION=x.y.z` builds, signs (ad-hoc + EdDSA), uploads the zip and appcast to the public repo.
4. An installed 1.0.1 updates itself to 1.0.2 from the public repo and relaunches as 1.0.2; a zip with a bad signature is refused.
5. The bundled skill installs into `~/.claude/skills/suno-que-huong` on a Mac that lacks it, updates it on new versions, and backs up local edits before overwriting.

## Steps
1. Go: `-bundled-skill` sync (hash manifest, backup of local edits), version via ldflags.
2. Swift: `macapp/` SwiftPM package (AppKit + WKWebView + Sparkle).
3. Packaging: Info.plist template, Makefile `app`, `install`, `release`; Sparkle tools and keys.
4. Releases repo; release 1.0.1, install, release 1.0.2, verify auto-update end to end.
5. Docs.

## Findings
- Ad-hoc signatures differ per build, so Sparkle logged "Code signature of the new version doesn't
  match" and installed without relaunching. Fixed with a self-signed identity (stable designated
  requirement `certificate root = H"6c61…"`).
- Sparkle asks the app to quit from inside a main-queue block; a `.terminateLater` reply queued on
  the main queue deadlocked. Quit now stops the server synchronously and answers `.terminateNow`.
- zsh's `log` builtin shadows `/usr/bin/log`; use the full path to read Sparkle's os_log.
- A 1-byte change to a release zip fails `sign_update --verify` with the studio key.
