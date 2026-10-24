# Quê Hương Studio: production readiness for non-developer users

Date: 2026-10-24. Scope: distribution, LLM backend, job robustness, observability, testing, accessibility, licensing. The UI stays Vietnamese-only, per a scope update from the user.
Grounded in the current code (`runs.go`, `agents.go`, `actions.go`, `main.go`, `packaging/`, `Makefile`) and the skill at `~/.claude/skills/suno-que-huong/scripts`.

## Outcome first

The app is already well built for its author: process groups, `WaitDelay`, atomic `meta.json`, input fingerprints and a heavy/light concurrency split are all present. Three things stop a musician from using it, and they decide the order of the work:

1. **It assumes a developer machine.** The PATH is baked into a bash launcher at `make install`. ffmpeg, uv, Python, the skill folder in `~/.claude/skills` and an agent CLI all have to be installed already. Nothing is signed.
2. **Its text steps need Claude Code, Codex or omp, and run them with `bypassPermissions` by default.** `config.go` sets `PermissionMode: "full"`. Three of the five agent actions (`lyrics.agent`, `style.agent`, `scenes.agent`) are plain text-in, files-out tasks. One direct LLM call with a JSON schema can replace each of them.
3. **Some of the legal ground is unexamined.** The Demucs weights are "not covered by the MIT license, and are provided only for scientific purposes" (maintainer, [demucs#327](https://github.com/facebookresearch/demucs/issues/327)). The video render uses `libx264`, which needs a GPL ffmpeg build. Suno changed its download and commercial terms on 2026-09-03.

Recommended order:
- **Phase 1:** packaging plus first-run bootstrap.
- **Phase 2:** direct LLM backend.
- **Phase 3:** job-system hardening.
- **Phase 4:** doctor page, diagnostics and tests.
- **Phase 5:** accessibility.

Licensing items run alongside these phases, before anyone else receives a build.

## Gaps found in the current code

| Area | Observation | Where |
|---|---|---|
| Launch | `CFBundleExecutable` is a bash script that bakes the installer's `$PATH` into the app. A non-developer's Mac has no such PATH. | `packaging/launcher.sh`, `Makefile` |
| Agent safety | The default `permissionMode` is `"full"`, so Claude runs with `bypassPermissions` and Codex with `--dangerously-bypass-approvals-and-sandbox`. The prompts embed user-supplied content (the brief and reference analyses), which makes prompt injection possible. | `config.go:62`, `agents.go` |
| Restart | `loadHistory` marks **queued** runs that never started as `interrupted`, just like running ones. A queued chain is lost on restart. | `runs.go` `loadHistory` |
| Timeouts | Runs have no wall-clock or inactivity timeout. A hung agent or script holds a slot forever. | `runs.go` `execute` |
| Retries | The image script retries any `Exception` with linear backoff, including non-retryable 4xx errors such as a content-policy refusal. | `generate-scene-images.py:50` |
| Partial outputs | Scripts write their outputs in place. A stopped run can leave a half-written file that later looks "done". | skill scripts |
| Reproducibility | `requirements-video.txt` pins only top-level packages, so transitive dependencies float. The `uv pip install` runs without hashes. | `setup-video-env.sh` |
| Language | Alignment hardcodes `language="vi"`. The lục bát check and the presets are Vietnamese-only, which conflicts with the "any genre/language" goal. | `karaoke-align.py:176`, `config.go` presets |
| Trash | `moveToTrash` calls `os.Rename` into `~/.Trash`. This fails across volumes (`EXDEV`), for example for projects on an external drive. It probably does not record Finder's Put Back location either (not verified). | `projects.go:641` |
| Paths | Data lives in `~/.local/share` and `~/.config`, which are hidden dot-folders a musician cannot find. | `main.go` |
| Licensing | There is no LICENSE or third-party notices file in the app repo. | repo root |

---

## 1. Distribution on macOS

### Signing options, ranked

| Option | Cost | UX on macOS 15/26 | Homebrew | Verdict |
|---|---|---|---|---|
| **A. Developer ID signed, notarized and stapled DMG** | $99/yr Apple Developer Program, plus 1–2 days of CI setup | Double-click opens it | Own tap works | **Recommended** once anyone other than the author installs it |
| B. Ad-hoc signed (unsigned) DMG | $0 | Blocked on first open. The user must go to System Settings → Privacy & Security → "Open Anyway", because Sequoia removed the Control-click bypass ([AppleInsider](https://appleinsider.com/inside/macos-sequoia/tips/whats-changed-in-runtime-protection-for-macos-sequoia), [The Hacker News](https://thehackernews.com/2024/08/apples-new-macos-sequoia-tightens.html)) | Not viable | Acceptable only for 1–3 known users with written steps |
| C. Official homebrew-cask | — | — | Homebrew 5.0 deprecated casks that fail Gatekeeper checks, to be disabled on 2026-09-01, and deprecated `--no-quarantine` ([Workbrew](https://workbrew.com/blog/what-homebrew-5-0-0-means-for-your-mac-fleet), [CopyQ issue](https://goodfirstissue.org/hluk/CopyQ/issues/3498)) | Requires option A first, and non-developers do not use brew anyway |

The anti-pattern to avoid is the one UVR documents: telling users to run `sudo spctl --master-disable` and `xattr -rd com.apple.quarantine` ([UVR README](https://github.com/Anjok07/ultimatevocalremovergui)). The first command turns off Gatekeeper for the whole Mac.

### Proper .app bundle (must)

- **Main executable.** Make the Go Mach-O binary the main executable (`Contents/MacOS/que-huong-studio`) and delete the bash launcher. Apple DTS guidance is that non-Mach-O files signed as code keep their signature in extended attributes, which break in transit ([Apple forums 781885](https://origin-devforums.apple.com/forums/thread/781885), [113954](https://developer.apple.com/forums/thread/113954)).
- **Tool resolution without PATH.** The Go binary should find tools itself, in this order:
  1. app-managed tools in `~/Library/Application Support/QueHuongStudio/bin` and bundled tools in `Contents/Resources/bin`;
  2. `/opt/homebrew/bin`, `/usr/local/bin` and `/Applications/MuseScore 4.app/...`;
  3. only then `$PATH`.

  Pass the resolved absolute paths to scripts through env vars such as `QHS_FFMPEG` and `QHS_PYTHON`, instead of letting them call `ffmpeg` by name.
- **Ship the skill inside the app.** Put it under `Contents/Resources/skill/` (scripts, references, assets) and version it with the app. User-editable data (new style profiles, presets) moves to Application Support. The app must not depend on `~/.claude/skills` existing. Keep a config override (`skillDir`) so the developer can keep working from the live skill.
- **Paths.**
  - App data: `~/Library/Application Support/QueHuongStudio/` (config, runs, venv, managed Python, uv).
  - Models: `~/Library/Caches/QueHuongStudio/models`. They can be re-downloaded, so Caches is acceptable.
  - Logs: `~/Library/Logs/QueHuongStudio/`.
  - Migrate the old `~/.local/share` and `~/.config` folders once, on first launch.
  - Projects stay in `~/Music/...`, which users can see.
- **Bundle ID and architecture.** Use a reverse-DNS bundle ID you control instead of `local.que-huong-studio`. Build **arm64 only**: PyTorch stopped shipping macOS x86_64 wheels after 2.2 ([PyTorch dev-discuss](https://dev-discuss.pytorch.org/t/pytorch-macos-x86-builds-deprecation-starting-january-2024/1690)) and the venv pins torch 2.14.1. An arm64-only app is refused cleanly on Intel Macs.
- **Signing order** (CI on a macOS runner, tag builds only):
  1. Sign nested binaries first (bundled `uv`, `ffmpeg`, `ffprobe`) with `codesign --force --options runtime --timestamp -s "Developer ID Application: …"`.
  2. Sign the app.
  3. Create the DMG with `hdiutil create -format UDZO` and sign it.
  4. Notarize with `xcrun notarytool submit --wait --timeout 2h`, then `xcrun stapler staple`.

  Notarization queues stalled for hours in March–May 2026 ([Apple forums](https://origin-devforums.apple.com/forums/topics/code-signing-topic/code-signing-topic-notarization)), so make the CI step resumable by submission ID. `gon` is archived ([mitchellh/gon](https://github.com/mitchellh/gon)), so use Apple's own tools directly.
- Do not set `LSFileQuarantineEnabled`. Files that Go's `net/http` and uv download are then not quarantined, so managed Python and ffmpeg run without Gatekeeper prompts.

### Auto-update, ranked

1. **Should: "update available" notice.** Fetch a JSON manifest from GitHub Releases containing the version, DMG URL, sha256 and an ed25519 signature checked with stdlib `crypto/ed25519` and an embedded public key. Show a banner, download the DMG with a progress bar, verify it, open it, then quit. This is about 150 lines of stdlib Go and keeps the user in control.
2. **Could: in-place swap, Ollama-style.** Ollama's Go updater stages the download by ETag, verifies the extracted bundle's code signature (`SecStaticCodeCheckValidity` via cgo), and swaps the bundle at next start ([ollama/app/updater](https://github.com/ollama/ollama/tree/main/app/updater)). Without cgo, the check can shell out to `codesign --verify --deep --strict` and `spctl -a -t exec`, and compare the `TeamIdentifier` from `codesign -dv`.
3. **Not recommended: Sparkle.** It is a Cocoa framework, so using it from Go needs cgo and an Objective-C shim. `sparkle-cli` is no longer in the binary distribution as of Sparkle 2.9 and has to be built from source ([Sparkle upgrading](https://sparkle-project.org/documentation/upgrading/), [sparkle-cli](https://sparkle-project.org/documentation/sparkle-cli/)). That is too much machinery for a single-binary Go app.

Updating the skill and venv is already half solved. `setup-video-env.sh` stamps a sha256 of the requirements file. Keep that idea, but have the stamp hash a lockfile (see below).

### First-run bootstrap (must)

Build a setup wizard page that runs as ordinary queued runs, so progress, logs and cancellation come for free. Every step is idempotent and leaves a stamp. The Doctor page offers "Repair" to re-run any step.

1. **Preflight.** Check for arm64, the minimum macOS version, and free disk space with `syscall.Statfs` (`Bavail*Bsize`) on Application Support, Caches and the projects folder. Measure the real footprint on a clean Mac first: torch plus the venv, Whisper `tiny.pt` (75 MB locally), the htdemucs weights and managed Python. Refuse to start below that figure plus a margin. Run the same check before each heavy run. A Demucs pass writes WAV stems, and a 1080p render writes hundreds of MB.
2. **uv.** Bundle the pinned `uv` binary (MIT/Apache-2.0, latest 0.13.5 as of [2026-10-22](https://github.com/astral-sh/uv/releases)), signed as nested code. That avoids a download-to-run step. Set `UV_PYTHON_INSTALL_DIR` ([uv storage docs](https://github.com/astral-sh/uv/blob/main/docs/reference/storage.md)) and `UV_CACHE_DIR` to app-owned folders so nothing touches the user's own Python. This is the same approach as ComfyUI Desktop, which installs its Python dependencies with uv on startup ([Comfy-Org/desktop](https://github.com/Comfy-Org/desktop)).
3. **Python environment.**
   1. Replace `requirements-video.txt` with a fully locked, hashed file (`uv pip compile --generate-hashes`, or a `pyproject.toml` plus `uv.lock`).
   2. Install with `uv python install 3.12`, `uv venv`, then `uv pip sync` against the lock.
   3. Stamp the lock's sha256.

   With every transitive dependency pinned, two machines get the same environment.
4. **ffmpeg.** See section 7. The recommendation is a self-built LGPL ffmpeg/ffprobe pair that uses `h264_videotoolbox`, bundled and signed.
5. **Models.** Add a Go downloader with HTTP `Range` resume (`If-Range` on the ETag; accept `206`, restart on `200`), writing `*.part` files and renaming them after the sha256 check.
   - Write the files to the exact paths the libraries already look in: `$XDG_CACHE_HOME/whisper/<name>.pt` and `$TORCH_HOME/hub/checkpoints/<file>.th`. Set those env vars for child processes.
   - Expected hashes are available without guessing. Whisper puts the sha256 in the URL path and verifies it ([whisper/__init__.py](https://github.com/openai/whisper/blob/main/whisper/__init__.py)). Demucs calls `torch.hub.load_state_dict_from_url(check_hash=True)`, and the filename carries the hash prefix ([demucs/repo.py](https://github.com/facebookresearch/demucs/blob/main/demucs/repo.py)).
   - Neither library resumes partial downloads, which is why the Go downloader is needed.
6. **MuseScore (optional).** Detect it. If it is missing, show "Install MuseScore 4 for PDF" with a link to musescore.org. Never bundle it (GPL, large, separate updates).
7. **Offline mode.** After bootstrap, set `UV_OFFLINE=1` for all runs (supported since uv 0.5, [changelog](https://github.com/astral-sh/uv/blob/main/changelogs/0.5.x.md)). Whisper and Demucs then load from cache. Only LLM calls (unless the provider is local), image generation and the update check need the network. Detect connectivity cheaply (a HEAD to the configured provider with a 2 s timeout) and disable those actions with a Vietnamese reason, rather than letting runs fail.

---

## 2. Replacing agent CLIs for text tasks

### Options, ranked

| Option | Non-developer setup | Reliability | Determinism and testability | Safety | Cost |
|---|---|---|---|---|---|
| **A. Direct LLM API with a JSON schema; Go validates and writes the files** | Paste one API key, or run Ollama/LM Studio | High: schema-constrained output and one HTTP call | Output is data. Go checks domain rules and caches by input hash | No tool use, no shell, no file access | Small: a few thousand tokens per step, and the API returns usage |
| B. Keep agent CLIs as the only path | Install Node or Homebrew, a CLI and a login | Medium: multi-turn, tool loops, CLI version drift | Low: the agent edits files directly | Low with `bypassPermissions` | Subscription or API |
| C. Local model only (Ollama / LM Studio) | Large model download and RAM | Depends on the model. Vietnamese lyric editing quality with 7–14B models is unverified | As in A | High | $0 |

**Recommendation: A by default, C as a selectable provider, B kept only for the genuinely agentic actions.** Those are `suno.agent` (CUA form fill), `ref.save.agent` (edits the skill) and the free-form console. Hide them behind an "Advanced: coding agents" setting that defaults to the `edits` permission mode.

### Provider abstraction: two wire formats cover everything

- **OpenAI-compatible Chat Completions with `response_format: {type: "json_schema", strict: true}`.** This one format covers:
  - OpenAI.
  - OpenRouter. Set `provider.require_parameters: true` so routing only reaches endpoints that support structured outputs; support varies per endpoint ([OpenRouter docs](https://openrouter.ai/docs/guides/features/structured-outputs)).
  - LM Studio ([docs](https://lmstudio.ai/docs/developer/openai-compat/structured-output)).
  - Ollama, which accepts `response_format` on `/v1` ([openai-compatibility.mdx](https://github.com/ollama/ollama/blob/main/docs/api/openai-compatibility.mdx)) as well as native `format` ([blog](https://registry.ollama.ai/blog/structured-outputs)).
  - Gemini through its compatibility layer. Late in 2025 that layer was reported to ignore schemas ([forum](https://discuss.ai.google.dev/t/structured-output-not-working-via-the-openai-compatible-layer/108341)), so test it, or call the native API with `responseJsonSchema` ([Google blog](https://blog.google/technology/developers/gemini-api-structured-outputs/)).
- **Anthropic native Messages API with `output_config.format: {type: "json_schema"}`.** This is GA and needs no beta header ([Claude docs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs)). Anthropic's OpenAI-compatibility layer **ignores** `response_format` and is "not considered a long-term or production-ready solution" ([Claude OpenAI SDK docs](https://platform.claude.com/docs/en/api/openai-sdk)), so it cannot stand in for the native API.

Both formats are about 200 lines of `net/http` with no SDK, which keeps the binary stdlib-only.

### Concrete design

- **Split each text action into "propose" and "apply".**
  - `lyrics.agent` returns `{title, subtitle, sections:[{tag, lines[]}], fixes:[{old,new}], suggestions[], questions[]}`.
  - `style.agent` returns `{style, section_tags:[{tag, description}], vocal, profile_md}`.
  - `scenes.agent` returns `{scenes:[{id,start,prompt,reuse?}]}`.
  - Go then validates and writes `suno/lyrics.txt`, `suno/style.txt`, `suno/profile.md`, `notes.md` and `project.json` atomically.
- **Validate domain rules in Go after schema validation.** The schema guarantees shape, not meaning. Check that:
  - the sung lines are unchanged except for listed fixes;
  - the style is under 1000 characters;
  - scene starts match line starts;
  - enum values compare case-insensitively, since Claude may vary their capitalisation (Claude docs).

  On failure, retry once with the validator's error appended, then surface the error.
- **Write schemas within each provider's limits.** Anthropic requires `additionalProperties:false` and rejects `minimum`/`maxLength`-style constraints with a 400 error. Enforce those constraints in Go. The first request with a new schema is slower because the grammar compiles, then it is cached for 24 h (Claude docs).
- **Handle stop reasons.** `stop_reason: "refusal"` or `"max_tokens"` can return non-conforming JSON with HTTP 200. Treat both as failures with a clear Vietnamese message.
- **Determinism.** Claude 4.7 and later reject `temperature` below 1 (Claude OpenAI-compat docs). Do not design around temperature 0. Get determinism from the schema, the validation and **result caching**: key on the sha256 of (prompt template version, inputs, model, schema), so re-clicking with unchanged inputs costs nothing and returns the same result. Regenerating is an explicit user action.
- **Prompts stay in the skill.** Move the prompt templates into `skill/prompts/*.md` so the agent path and the API path share one source.
- **Keys.** Store keys as a mode-0600 file in Application Support, the same pattern as `aihub.key`. Never return them to the UI, and redact them from logs and diagnostics. Keychain via `/usr/bin/security` is optional (could).
- **Settings UI.** Fields: Provider, Base URL, Model, Key, and a "Test" button that sends a 1-token request and runs one schema round-trip.
- **Change the default permission mode to `edits` now, before any other work.** This is a one-line change in `config.go` and closes the prompt-injection-to-shell path.

---

## 3. Job system robustness

### SQLite or files

| | Files (current) | SQLite (`modernc.org/sqlite`) |
|---|---|---|
| Scale needed | Up to 300 runs per user, 1 process | Built for far more |
| Crash safety | Already atomic `meta.json`, plus append-only `events.jsonl` | Transactions and WAL |
| Dependencies | Stdlib only | Large transpiled-C module and several MB more binary ([size notes](https://github.com/ncruces/go-sqlite-bench)) |
| Debuggability | `cat` / Finder | Needs a tool |

**Keep files.** No current requirement needs SQL (cross-run queries, multiple writer processes). Revisit only if run history must be searchable across thousands of runs.

### Must

- **Re-queue on restart.** In `loadHistory`, runs whose status is `queued` and that have no `Started` go back to `queued`, and the scheduler is poked. Only `running` runs become `interrupted`. Add a **"Resume chain"** button that re-submits the interrupted step and its dependents. This is safe because the steps are fingerprinted (next point).
- **Generalise fingerprints to every step.** `InputsHash` and `RecordStamp` already exist for stills and render.
  - Each action declares its inputs: files, parameters, a script version hash and a model ID.
  - The fingerprint is the sha256 of those inputs, stored in `project.json` stamps.
  - A step whose stamp matches its current inputs is skipped with the status "already up to date".
  - LLM steps use the same cache key described in section 2.
- **Atomic outputs in Python.** Write to `name.tmp` and then `os.replace`. Install a SIGTERM handler that deletes temp files. A stopped run then never leaves a half-file that looks complete.
- **Timeouts.** Use per-action-kind limits:
  - a wall-clock maximum (for example 2 h for a render, 10 min for an LLM call);
  - an **inactivity watchdog** that terminates a run after N minutes with no progress event, which suits long media jobs better than wall-clock limits.

  Reuse `terminate()` (SIGTERM to the group, then SIGKILL). Record `status: "timeout"` as its own state with a Vietnamese label in `statusLabel`.
- **Classified retries.** Retry only network errors, 408, 429 and 5xx. Use exponential backoff with full jitter (base 2 s, cap 60 s, 4 attempts) and honour `Retry-After`. Never retry other 4xx errors such as a content-policy refusal. Apply this in three places: the Go LLM client, the Go model downloader, and the image script (replace its catch-all `except Exception`).

### Should

- **Structured progress over a dedicated pipe.**
  - Pass a pipe as fd 3 (`cmd.ExtraFiles`) and set `QHS_PROGRESS_FD=3`.
  - Python writes JSON Lines to that fd through one shared helper, `skill/scripts/qhs_progress.py`.
  - stdout and stderr stay raw logs, so torch and tqdm noise can never corrupt the protocol.
  - When the env var is missing (script run by hand), the helper prints human-readable text.

  Message shapes:

  ```json
  {"v":1,"type":"stage","id":"separate","label_key":"stage.separate"}
  {"v":1,"type":"progress","pct":0.42,"eta_s":95}
  {"v":1,"type":"artifact","path":"work/timings.json"}
  {"v":1,"type":"error","code":"NO_SUBTITLE_TRACK","hint_key":"err.no_subtitle_track"}
  {"v":1,"type":"heartbeat"}
  ```

  Errors carry **stable codes**, and the UI maps each code to a Vietnamese message and fix hint. The Python scripts print English, so their raw text would otherwise reach a Vietnamese-only UI. Stage-level progress plus a 5 s heartbeat is enough: Demucs and Whisper progress come from tqdm and are not worth parsing.
- **Consolidate cancellation.** Move to `exec.CommandContext` with `cmd.Cancel` set to signal the group, keeping `Setpgid` and `WaitDelay`. Timeouts and the Stop button then share one path ([pattern notes](https://keploy.io/blog/technology/managing-go-processes), [Go tools example](https://go.googlesource.com/tools/+/HEAD/cmd/bisect/go120.go)).
- **Concurrency.** Keep heavy=1 and light=N. Add a `network` class (image generation, LLM, downloads) so a slow API cannot starve agent slots. Do not build RAM or GPU accounting: one heavy job at a time is already the right limit on an M-series laptop.

### Could

- Use `testing/synctest` (Go 1.25+) to test the scheduler's timers and the watchdog without sleeps.
- Use `os.Root` (Go 1.24+) for all project-relative file access, as defence in depth against path traversal in API handlers.

---

## 4. Observability

- **Must: structured logs.**
  - Use stdlib `log/slog` with a JSON handler writing to `~/Library/Logs/QueHuongStudio/studio.log`.
  - Rotate at startup and when the file passes 10 MB, keeping 5 files. This is Ollama's approach in about 30 lines of stdlib code ([ollama/app/logrotate](https://github.com/ollama/ollama/blob/main/app/logrotate/logrotate.go)).
  - Attach `run_id`, `project` and `step` attributes.
  - Remove the bash-level redirect.
- **Must: Doctor page.** Each check returns `{id, status: ok|warn|fail, detail, fix_key, fix_action}`. The checks:
  - arm64 and the macOS version;
  - free disk space;
  - uv version;
  - managed Python version;
  - lock hash equals the venv stamp;
  - `python -c "import torch, demucs, stable_whisper, librosa, music21"` with versions, plus whether `torch.backends.mps.is_available()`;
  - ffmpeg and ffprobe versions, and whether `-encoders` lists `h264_videotoolbox`;
  - model files present with the right hash;
  - MuseScore present (optional);
  - LLM provider configured, with "Test" run only on click;
  - projects folder writable;
  - skill version matches the app.

  Fix actions map to bootstrap steps ("Repair Python", "Re-download model"). Cache the results for 5 minutes, like `agentInfos`.
- **Should: diagnostics bundle.** Add a "Export diagnostics" button that builds a zip on the Desktop with `archive/zip` and reveals it in Finder. Contents:
  - app and skill version, `sw_vers`, architecture;
  - Doctor results;
  - the redacted config;
  - the last 20 runs' `meta.json` plus the tails of `events.jsonl` and `stdout.log` (last 500 lines each);
  - the last 2 rotated logs.

  Redaction rules:
  - drop config keys matching `(?i)key|token|secret|password|authorization`;
  - regex-scrub `sk-[A-Za-z0-9_-]{16,}`, `Bearer \S+` and the per-launch token;
  - replace the home path with `~`.

  **Exclude audio, images and lyrics by default.** They are the user's creative work; offer a checkbox to include lyrics. Show the file list before export.
- **Could:** a "Copy error details" button on failed runs that copies the run's error code, the last 50 log lines and versions, ready to paste into a message.

---

## 5. Testing

| Layer | What | Where it runs |
|---|---|---|
| Go unit | Keep `parsers_test`/`core_test`. Add Manager tests driven by **fake child scripts** (tiny shell or Python files in `testdata/`) that emit the fd-3 protocol, hang, ignore SIGTERM, or exit non-zero. These cover restart re-queue, timeout, watchdog, cancel and retry classification | `go test`, any OS |
| Go LLM client | `httptest.Server` returning recorded provider responses, including refusal, `max_tokens`, 429 with `Retry-After`, and schema-invalid output for the validator | `go test` |
| Python contract | For each script: `--help` exits 0; on a fixture project it emits valid protocol lines and writes outputs that validate against **JSON schemas shared with Go** (one `skill/schemas/*.json`, used by both sides) | `pytest`, local and nightly |
| Media golden | Compare with tolerances, never bytes. Timings: every word within ±80 ms of golden. MusicXML: same note count and pitches within ±1 semitone at ≥ 90%. Render: `ffprobe` duration ±0.1 s, 1920×1080, H.264 plus AAC, faststart. Stills: perceptual hash distance under a threshold. Force `--device cpu` for repeatability | local `make test-media` and a nightly macOS job |
| UI smoke | Playwright (in a separate `e2e/` folder with its own `package.json`; Node stays test-only). Start the binary with `-no-open -data $tmp`, read the token, then: open the app, create a song, and check that step states render. Add `@axe-core/playwright` with WCAG A/AA tags ([Playwright a11y docs](https://playwright.dev/docs/accessibility-testing)) | CI |

**Fixtures.** Use a 10–15 s clip so tests stay fast:
- Generate the vocal fixture once, from a CC0 or own-recorded snippet, or as a sung or spoken line from macOS `say` with a Vietnamese voice.
- **Commit the resulting WAV**, because regenerating it on another macOS version changes the audio.
- Add one English fixture for the multi-language goal.
- Store the expected timings next to the fixtures.

**CI cost.** The repo is **private**. GitHub's Free plan includes 2,000 minutes a month and macOS runners cost $0.062/min ([GitHub billing](https://docs.github.com/en/billing/concepts/product-billing/github-actions)). Split the jobs:
- **ubuntu-latest:** gofmt, vet, Go tests and the Playwright smoke on every push. This works if the server builds on Linux; it uses only `syscall.Kill`/`Setpgid`, which are POSIX.
- **macos (arm64):** sign, notarize and DMG on tags only.
- Media golden tests run nightly or manually, with the model cache restored from `actions/cache`.

---

## 6. Accessibility and song language

Scope update: the UI stays Vietnamese-only, so UI i18n is dropped. `<html lang="vi">` is already correct.

- **Must: song (content) language, which is separate from UI language.** This is needed for the "any genre/language" goal, not for UI translation.
  - Content language and genre are **per-project** fields in `project.json` (`language: "vi"`), passed to scripts (for example `--language` for stable-ts in place of the hardcoded `"vi"`).
  - Vietnamese-only checks (lục bát) and presets turn on only when `language == "vi"`.
- **A11y basics, targeting WCAG 2.2 AA:**
  - semantic `<button>` and `<label for>` elements;
  - `aria-live="polite"` for run status and progress;
  - visible focus everywhere (focus rings currently exist only on `.btn`, inputs and `select`);
  - keyboard operation for drag-and-drop zones, with a "Choose file" button next to each;
  - contrast checked in both themes;
  - `prefers-reduced-motion`;
  - a minimum 24×24 px target size.

  axe in CI catches only part of this. A manual keyboard-only pass per release is still needed (Playwright docs).

---

## 7. Licensing and compliance checklist

| Component | License | How the app uses it | Obligation or risk | Action |
|---|---|---|---|---|
| App code | none yet | — | Recipients have no grant | **Must:** add a LICENSE and `THIRD_PARTY_NOTICES` shown in About |
| Preact, htm, x/text | MIT, Apache-2.0, BSD-3 | Embedded | Keep the notices | Include in NOTICES |
| ffmpeg | LGPL-2.1+ by default; GPL when built with `--enable-gpl` (required for libx264) ([FFmpeg LICENSE](https://doc.qt.io/qt-6.10/qtwebengine-3rdparty-ffmpeg.html)) | Executed as a separate program | Redistributing a GPL binary obliges you to provide the corresponding source. LGPL is lighter | **Must:** pick one route (ranked below) |
| MuseScore 4 | GPL-3.0 | External CLI the user installs | None if you do not bundle it. Exec of a separate program is "aggregation" in the GPL FAQ's terms ([summary](https://lists.opensource.org/pipermail/license-discuss_lists.opensource.org/2012-January/017511.html)) | Do not bundle; link to musescore.org |
| uv, python-build-standalone | MIT/Apache-2.0, MPL-2.0 + PSF | Bundled or downloaded | Keep the notices | NOTICES |
| torch, torchaudio, numpy, librosa, music21, soundfile | BSD and ISC family | Installed from PyPI on the user's Mac | Not redistributed by you | Generate a list from the venv metadata into NOTICES |
| Demucs code | MIT, **repo archived** | PyPI | Adoption risk: no upstream fixes | Pinned; fine for now |
| **htdemucs weights** | "not covered by the MIT license… only for scientific purposes" ([demucs#327](https://github.com/facebookresearch/demucs/issues/327)). A newer HF mirror is tagged MIT, which conflicts ([HF discussion](https://huggingface.co/adefossez/HTDemucs/discussions/1)) | Downloaded from Meta's URL on the user's machine. Only the vocal stem is used, internally, for alignment and transcription | Commercial use is unclear. Re-hosting the weights would be redistribution | **Must:** never re-host. Disclose in About and Doctor. **Should:** evaluate a separation model with a clear commercial license, behind the same script interface |
| lameenc (Demucs dependency) | Was GPLv3, now LGPL ([demucs#519](https://github.com/facebookresearch/demucs/issues/519)) | PyPI | Matters only if bundled | Pin a version whose metadata says LGPL |
| Whisper code and weights | MIT ([README](https://github.com/openai/whisper)) | Downloaded, hash-checked | Notice | NOTICES |
| stable-ts | MIT, **repo archived** (last push 2026-05) | PyPI | Adoption risk | Pinned; watch for a successor |
| Fonts: Anton, Lobster | SIL OFL 1.1 | Bundled, burned into videos | Ship the OFL text and do not sell the fonts alone. Video output is unrestricted | Already ship `*-OFL.txt`; list them in NOTICES |
| Image provider ("aihub", gpt-image) | Provider ToS | Network API | Output rights and content policy follow the provider, and a proxy adds its own terms | Show the provider name and a ToS link in Settings |

**ffmpeg routes, ranked:**

1. **Self-built LGPL ffmpeg with `h264_videotoolbox`** (no x264), bundled, signed, with the exact source tarball and configure line published next to each release. This is the cleanest to redistribute and uses hardware encoding. The trade-off: VideoToolbox needs a higher bitrate than x264 CRF 18 for the same quality, so use `-b:v 8M` or more at 1080p for karaoke and compare visually once. The render script needs a codec switch, or should prefer `libx264` when the found ffmpeg has it.
2. Bundle a third-party GPL static build, with the GPL text and a source offer. Simple, but it carries GPL obligations on every release.
3. Require Homebrew ffmpeg. Zero obligations, but non-developers cannot do it.

**Notes for users in About/Help (should):**
- **Suno.** Since 2026-09-03:
  - Free accounts get 7 lifetime trial downloads, which are not eligible for commercial use.
  - Pro gets 20 downloads a month and Premier 60.
  - Downloads on paid plans "remain yours to use commercially or personally" ([Suno blog, 2026-08-10](https://blog.suno.com/blog/suno-updates-tos)).

  The workflow consumes one download per song (the M4A carries the timed lyrics). The UI should say this.
- **Copyright.**
  - Lyrics the user wrote are theirs.
  - US: prompts alone do not make the user the author of AI output; human selection, arrangement and edits can be protected ([USCO Part 2 summary](https://copyrightalliance.org/ai-report-part-2-copyrightability/)).
  - Vietnam: amended IP Law 131/2025/QH15, in force since 2026-04-01, protects AI-assisted output only where human creativity is present ([Vietnam Briefing](https://www.vietnam-briefing.com/news/vietnam-amends-intellectual-property-law-key-changes-for-businesses.html), [Tilleke on Decree 134/2026](https://www.tilleke.com/insights/new-decree-strengthens-vietnams-copyright-framework-in-the-ai-era)).
- **Reference songs** are analysed locally and never uploaded. Keep the existing "learn style, do not copy" rule visible.
- **Automating suno.com.** I could not confirm whether Suno's ToS bans automated form filling ([Conduct Atlas excerpts](https://conductatlas.com/platform/suno/suno-terms-of-service/provision/CA-P-044763/prohibition-on-service-interference-or-disruption/)). Keep the CUA fill optional, never press Create automatically (already the case), and make copy-to-clipboard the default path.

---

## Prioritized checklist

### Must (before a non-developer installs it)

1. Change the default `permissionMode` to `edits` (`config.go`).
2. Make the Go binary the bundle's main executable, resolve tool paths without PATH, and ship the skill inside the app (`Resources/skill`). Move data to `~/Library/...` with a one-time migration.
3. Make an explicit signing decision. Developer ID plus notarized DMG for distribution beyond yourself; otherwise documented "Open Anyway" steps. Never `spctl --master-disable`.
4. Build the first-run bootstrap wizard: preflight (arm64, disk), bundled uv, managed Python, a **hash-locked** environment, ffmpeg, and resumable model downloads verified by sha256. Every step stamped and repairable.
5. Add the direct LLM backend (OpenAI-compatible plus Anthropic native) with JSON schemas, Go-side validation and atomic writes, for lyrics, style and scenes. Agent CLIs become optional and advanced.
6. Job robustness: re-queue never-started runs on restart; atomic outputs and a SIGTERM handler in Python; wall-clock and inactivity timeouts; classified retries with jitter and `Retry-After`.
7. Make content language a per-project field passed to the scripts (remove hardcoded `"vi"`), with Vietnamese-only checks gated on it.
8. Doctor page with fix actions.
9. Licensing: add a LICENSE and NOTICES, choose an ffmpeg route, never re-host the Demucs weights, and write the Demucs/Suno disclosures.
10. Replace `os.Rename` into `~/.Trash` with `/usr/bin/trash`, which is present on macOS 26.5 (verified locally; minimum macOS version not verified). Fall back to the current code when it is absent, and show an error rather than failing silently with `EXDEV`.

### Should

1. slog logging with rotation; a diagnostics bundle with redaction.
2. Structured fd-3 JSON progress protocol with error codes, plus a shared Python helper.
3. Generalise input fingerprints and the LLM result cache to every step, and add a "Resume chain" button.
4. Signed update-available check (manifest plus ed25519, open the DMG).
5. Accessibility fixes (section 6) plus axe in CI.
6. Tests: Manager tests with fake children, recorded LLM fixtures, Python contract tests against shared schemas, a Playwright smoke on ubuntu, and media golden tests nightly.
7. Offline mode (`UV_OFFLINE=1` and capability-gated actions).
8. A `network` concurrency class.
9. Evaluate an alternative vocal-separation model with a clear commercial license.

### Could

1. Ollama-style in-place update swap with `codesign`/`spctl` verification.
2. Keychain storage for API keys.
3. `testing/synctest` for scheduler tests; `os.Root` for project file access.
4. Own Homebrew tap (only after notarization).
5. Local LLM presets (Ollama/LM Studio) after a Vietnamese quality check.
6. A "Copy error details" button.

## Anti-patterns to avoid

- Baking the installing shell's `$PATH` into the app, or calling tools by bare name from scripts.
- Making an app run by telling users to disable Gatekeeper or strip quarantine (the UVR README pattern).
- Running agents with `bypassPermissions` over user-supplied text by default.
- Using an LLM agent to *edit files* where a schema-constrained response plus a Go writer would do the job.
- Using Anthropic's OpenAI-compatibility layer for structured output: it silently ignores `response_format`.
- Assuming `temperature: 0` determinism on current Claude models.
- Installing Python packages without a hash-locked lockfile, or into the user's system or Homebrew Python.
- Re-hosting model weights, or bundling a GPL ffmpeg without the source obligations.
- Mixing machine-readable progress into stdout or stderr, where library output (tqdm, torch warnings) shares the stream.
- Retrying deterministic failures (4xx, content policy); retrying without jitter.
- Writing outputs in place, so that a cancelled run leaves files that look current.
- Adding SQLite, Sparkle, Electron or a JS build pipeline to solve problems this app does not have.
- Putting user audio and lyrics into diagnostics bundles by default.

## Limitations of this research

- I did not measure the clean-install disk footprint or the bootstrap time: the local venv folder was blocked by a scout hook. Measure both on a fresh macOS user account before setting the preflight thresholds.
- I did not test VideoToolbox against x264 quality, the Gemini compatibility layer's schema handling as of today, or Vietnamese output quality from local 7–14B models. Each needs a hands-on trial.
- Copyright and license points are research summaries, not legal advice. The Demucs weights question in particular has no authoritative resolution.
- I did not confirm the macOS version that introduced `/usr/bin/trash`, or whether Put Back works for items moved by `os.Rename` versus `trash`.
- Whether Homebrew actually carried out the 2026-09-01 cask disablement was not confirmed.

## Unresolved questions

1. Who is the audience: one musician friend (option B with written steps may be enough) or public distribution (option A, $99/yr)? This decides how much of section 1 is a must.
2. Will users publish or monetize the videos? If so, the Demucs weights' "scientific purposes only" statement needs a decision: accept the risk, ask Meta, or swap the separation model.
3. Which default LLM provider and model should a non-developer get (OpenRouter with one key, direct Anthropic or OpenAI, or local)? Is the user expected to bring their own key?
4. Should the Suno CUA fill stay in the product at all, given the unconfirmed ToS position and that it needs an agent CLI plus Computer Use?
5. Is the skill meant to stay shared with Claude Code (`~/.claude/skills`) for the developer, with the app carrying a copy? If so, how are the two versions kept in sync?
6. What is the app's own license?

## Sources

- Apple and Gatekeeper: [AppleInsider, Sequoia runtime protection](https://appleinsider.com/inside/macos-sequoia/tips/whats-changed-in-runtime-protection-for-macos-sequoia) · [The Hacker News](https://thehackernews.com/2024/08/apples-new-macos-sequoia-tightens.html) · [Apple forums: xattr signatures](https://origin-devforums.apple.com/forums/thread/781885) · [Apple forums: scripts in bundles](https://developer.apple.com/forums/thread/113954) · [Apple forums: notarization 2026](https://origin-devforums.apple.com/forums/topics/code-signing-topic/code-signing-topic-notarization) · [mitchellh/gon (archived)](https://github.com/mitchellh/gon)
- Homebrew: [Workbrew on Homebrew 5.0](https://workbrew.com/blog/what-homebrew-5-0-0-means-for-your-mac-fleet) · [CopyQ cask deprecation](https://goodfirstissue.org/hluk/CopyQ/issues/3498)
- Updates: [Sparkle upgrading](https://sparkle-project.org/documentation/upgrading/) · [sparkle-cli](https://sparkle-project.org/documentation/sparkle-cli/) · [Ollama updater source](https://github.com/ollama/ollama/tree/main/app/updater) · [Ollama logrotate](https://github.com/ollama/ollama/blob/main/app/logrotate/logrotate.go)
- Comparable apps: [Comfy-Org/desktop README](https://github.com/Comfy-Org/desktop) · [UVR README](https://github.com/Anjok07/ultimatevocalremovergui) · [Stability Matrix](https://lykos-ai.itch.io/stability-matrix)
- Python and models: [uv releases](https://github.com/astral-sh/uv/releases) · [uv storage docs](https://github.com/astral-sh/uv/blob/main/docs/reference/storage.md) · [uv 0.5 changelog](https://github.com/astral-sh/uv/blob/main/changelogs/0.5.x.md) · [uv hashes how-to](https://pydevtools.com/handbook/how-to/how-to-pin-dependencies-with-hashes-in-uv/) · [PyTorch macOS x86 deprecation](https://dev-discuss.pytorch.org/t/pytorch-macos-x86-builds-deprecation-starting-january-2024/1690) · [whisper `__init__.py`](https://github.com/openai/whisper/blob/main/whisper/__init__.py) · [demucs repo.py](https://github.com/facebookresearch/demucs/blob/main/demucs/repo.py)
- LLM APIs: [Claude structured outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs) · [Claude OpenAI SDK compatibility](https://platform.claude.com/docs/en/api/openai-sdk) · [OpenRouter structured outputs](https://openrouter.ai/docs/guides/features/structured-outputs) · [Ollama structured outputs](https://registry.ollama.ai/blog/structured-outputs) · [Ollama OpenAI compatibility](https://github.com/ollama/ollama/blob/main/docs/api/openai-compatibility.mdx) · [LM Studio structured output](https://lmstudio.ai/docs/developer/openai-compat/structured-output) · [Gemini structured outputs](https://blog.google/technology/developers/gemini-api-structured-outputs/) · [Gemini compat-layer report](https://discuss.ai.google.dev/t/structured-output-not-working-via-the-openai-compatible-layer/108341)
- Go: [process-group pattern](https://keploy.io/blog/technology/managing-go-processes) · [Go tools cmd.Cancel/WaitDelay](https://go.googlesource.com/tools/+/HEAD/cmd/bisect/go120.go) · [go-sqlite-bench](https://github.com/ncruces/go-sqlite-bench)
- Testing and a11y: [Playwright accessibility testing](https://playwright.dev/docs/accessibility-testing) · [GitHub Actions billing](https://docs.github.com/en/billing/concepts/product-billing/github-actions)
- Licensing and legal: [FFmpeg license text (Qt mirror)](https://doc.qt.io/qt-6.10/qtwebengine-3rdparty-ffmpeg.html) · [GPL aggregation discussion](https://lists.opensource.org/pipermail/license-discuss_lists.opensource.org/2012-January/017511.html) · [demucs#327 weights license](https://github.com/facebookresearch/demucs/issues/327) · [demucs#519 lameenc](https://github.com/facebookresearch/demucs/issues/519) · [HTDemucs HF license discussion](https://huggingface.co/adefossez/HTDemucs/discussions/1) · [Whisper README](https://github.com/openai/whisper) · [Suno ToS and downloads update](https://blog.suno.com/blog/suno-updates-tos) · [USCO Part 2 summary](https://copyrightalliance.org/ai-report-part-2-copyrightability/) · [IPWatchdog on USCO Part 2](https://ipwatchdog.com/2025/01/29/part-two-copyright-office-ai-report-says-creative-prompting-doesnt-constitute-authorship/) · [Vietnam IP law amendment](https://www.vietnam-briefing.com/news/vietnam-amends-intellectual-property-law-key-changes-for-businesses.html) · [Tilleke on Decree 134/2026](https://www.tilleke.com/insights/new-decree-strengthens-vietnams-copyright-framework-in-the-ai-era) · [Conduct Atlas Suno ToS excerpts](https://conductatlas.com/platform/suno/suno-terms-of-service/provision/CA-P-044763/prohibition-on-service-interference-or-disruption/)
