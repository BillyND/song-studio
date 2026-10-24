# Song Studio

A Mac app for the `suno-que-huong` skill. Each song goes through nine steps, from lyrics to a
song, a 1080p karaoke video and a lead sheet. Every step either runs one of the skill's scripts
directly or hands the work to a coding agent (Claude Code, omp or Codex), and the studio shows the
agent's work live: its replies, thinking, each tool call with its output, cost and duration.

The app is a Swift window (`macapp/`, WKWebView) around the Go server (`que-huong-studio`, bundled
inside the app), which still serves the UI on `127.0.0.1:4317` only. Apple Silicon, macOS 13+.

## Install and open

For friends: download the newest `SongStudio-<version>.zip` from
<https://github.com/BillyND/que-huong-studio-releases/releases>, unzip, drag the app to
Applications, open it, and the first time allow it under System Settings → Privacy & Security →
"Open Anyway" (the app is not signed by Apple). The releases repo README says the same in Vietnamese.

From source:

```bash
cd ~/Coding/Personal/que-huong-studio
make install        # tests, builds out/Song Studio.app, installs it to ~/Applications
                    # and links ~/.local/bin/que-huong-studio to the bundled server
```

Opening the app starts its own server. A studio already on port 4317 (a terminal run or an older
app) is asked to quit first. Closing the window or Cmd+Q stops the server cleanly (queued work is
kept). Finder gives apps no PATH, so the app reads the
login shell's PATH (Homebrew, uv, nvm, `~/.local/bin`) when it starts.

The window behaves like a browser for the studio: Vietnamese menus with the Edit shortcuts
(Cmd+C/V/X/A/Z), Cmd+R reload, zoom, `confirm()` dialogs, file pickers and drag-and-drop,
downloads to `~/Downloads`, and links outside the studio open in the default browser. Trợ giúp →
"Mở trong trình duyệt" opens the same studio in a browser.

## Updates

Sparkle 2.10 checks <https://github.com/BillyND/que-huong-studio-releases/releases/latest/download/appcast.xml>
every hour (and on Song Studio → Kiểm tra cập nhật…), downloads in the background, verifies
the EdDSA signature, and installs on quit or when the user clicks "Cài đặt cập nhật". Before
relaunching it waits until no job is queued or running. Each release also carries a snapshot of
the skill; on start the server installs it into `~/.claude/skills/suno-que-huong` when it is new,
backing up local edits to `~/.local/share/que-huong-studio/skill-backups/` first.

The app was called Quê Hương Studio until 1.0.12. Sparkle updates in place, so an old copy keeps
its file name until its first start on 1.0.13 or later, when it renames itself to
`Song Studio.app` and reopens. Internal names (bundle id `local.que-huong-studio`, the update
feed, `~/.config/que-huong-studio`, `~/.local/share/que-huong-studio`, keychain service
`que-huong-studio`, the skill) are unchanged, so songs, settings and keys carry over.

Publishing a version (only from the Mac that holds the two keys below):

```bash
echo 1.2.0 > VERSION
NOTES="- what changed" make release   # build, sign, zip, sign the zip, upload zip + appcast.xml
```

Two secrets in the login keychain make releases trustworthy; back both up (Keychain Access →
export), or installed apps can never be updated again:

| Keychain item | Made by | Purpose |
|---|---|---|
| Sparkle EdDSA key, account `que-huong-studio` | `packaging/sparkle-tools/bin/generate_keys --account que-huong-studio` | signs each zip; the public half is `packaging/sparkle-public-key.txt` and goes into Info.plist |
| Code signing identity "Que Huong Studio Signing" (self-signed) | `packaging/make-signing-cert.sh` | signs the app; Sparkle only installs an update signed like the installed app, and ad-hoc signatures change with every build |

| Command | What it does |
|---|---|
| `make build` | the Go server alone, `bin/que-huong-studio` |
| `make app` | `out/Song Studio.app` |
| `make test` | gofmt, `go vet`, Go tests |
| `make run` | build the server and start it in a terminal |
| `make release` | publish `VERSION` to the releases repo |
| `make uninstall` | remove the app and the link |

Server flags: `-addr 127.0.0.1:4317`, `-no-open`, `-config`, `-data`, `-bundled-skill`, `-version`.

## Where things live

| Path | Content |
|---|---|
| `~/Music/suno-que-huong/<song>/` | one folder per song: `project.json`, `lyrics.txt`, `suno/`, audio, `img/`, `work/`, the MP4 |
| `~/.claude/skills/suno-que-huong/` | the skill the agents and scripts follow |
| `~/.config/que-huong-studio/config.json` | settings (default agent, permission mode, concurrency, CLI paths, folders, image provider and model; never keys) |
| login keychain, service `que-huong-studio` | image API keys (`image.hub`, `image.openai`, `image.gemini`), written through `security` on stdin |
| `~/.local/share/que-huong-studio/runs/<id>/` | each run: `meta.json`, `events.jsonl`, raw `stdout.log`; the newest 300 are kept |
| `~/.local/share/que-huong-studio/studio.log` | server log when started from the app (rotated to `studio.log.1` past 5 MB) |
| `~/.config/suno-que-huong/aihub.key` | older hub key file; still read when the image script runs outside the studio, and importable into the keychain from Cài đặt |

## Workflow steps

| Step | Done when | Actions |
|---|---|---|
| Lời, bài mẫu & yêu cầu | `lyrics.txt` has text | edit, lục bát check, reference songs (drop, pick from Downloads or paste a path; copied to `ref/`, analysed into `ref/<file>.analysis.txt`), the brief (`brief.md`) with prompt snippets, then **Tạo prompt Suno**. There is no arrangement picker: the arrangement always follows the reference song's measurements plus the brief, and falls back to the skill's default profile only when no reference is given |
| Prompt Suno | `suno/lyrics.txt` and `suno/style.txt` exist and are newer than the lyrics, brief and analyses | chain status, copy/edit blocks, "why this arrangement" (`suno/profile.md`), agent: fill suno.com/create via CUA (never presses Create) |
| Nhạc từ Suno | `project.json` `audio` points at a file | drop the M4A, pick a recent Downloads file, or paste a path |
| Canh lời | `work/timings.json` exists | script `karaoke-align.py` |
| Rút ngắn intro | intro cut, or the voice already enters before 15 s | script `trim-intro.py` (a no-op when nothing needs cutting) |
| Cảnh & ảnh AI | every non-reused scene has `img/<id>.png` | agent: plan scenes; script: generate missing images |
| Xem trước | stills exist and were rendered from the current inputs | script: render stills (`still auto` picks the checkpoints) |
| Xuất video | an MP4 exists and was rendered from the current inputs | **Làm video tự động** (whole chain), or render only |
| Bản nhạc | `<title> - Bản nhạc.musicxml` is newer than the timings | script `transcribe-sheet.py`: meter (2/4, 3/4, 4/4) and an optional key to transpose to; open the PDF, edit in MuseScore, reveal in Finder |

The lead sheet takes its pitch from the separated vocal (pYIN) and its rhythm from the
per-word timings on the song's beat grid, so each Vietnamese syllable becomes a note and a
syllable held on two clear pitches becomes a slurred melisma. It is a draft: on the Ninh Bình
song about 77–81% of the sung pitch lands within a semitone of the written notes, and some
rhythms need a few minutes of correction in MuseScore. The PDF and page previews need
MuseScore 4 (`brew install --cask musescore`); without it only the MusicXML and MIDI are written.

The New Song dialog takes the same reference songs and brief, and by default starts
**Tạo prompt Suno** right away, so a song goes from lyrics to a ready Suno prompt in one step.

### Always one obvious next action

- The song header has one primary button that follows progress: **Tạo prompt Suno** →
  **Thêm nhạc từ Suno** → **Làm video** → **Xem video** (or **Đang chạy…**, which opens the
  running job).
- Every step ends with a **Tiếp: …** bar pointing to the next step.
- As soon as the music is added, the Nhạc step shows **Làm video tự động**; one click runs the
  whole video chain (about 12–25 minutes) and the page can be closed meanwhile.

### Deleting a video or a song

- **Xoá video** (Xuất video step) and the trash button in the song header (**Xoá bài**) move
  the file or the whole song folder to the macOS Trash, so Finder's *Put Back* undoes them.
  Disk space is freed only when the Trash is emptied.
- Both are refused while any job of that song is queued or running.
- Copies outside the song folder, such as a video copied to `~/Downloads`, are not touched.

## Chains and parallel work

**Tạo prompt Suno** queues a chain: reference analysis (if any file is unanalysed) runs in
parallel with the lyric-review agent; the arrangement agent starts when both succeed and
writes Styles, section-tag descriptions, `suno/profile.md`, `vocal` and `profile`. A failed link
fails the links after it instead of leaving them waiting. **Làm video tự động** chains align →
trim → scenes agent → images → stills → render and skips links that are already done; the
lead sheet runs right after trim, beside the scene work. The
console shows the whole chain above the timeline; click a link to follow it.

Jobs are either heavy (vocal separation, alignment, analysis, stills, render: default 1 at a
time) or light (agents and image downloads: default 3 in parallel). Both limits are in Cài đặt.
Default credits for new songs and the image model are there too.

## Genres, sung language and video look

Each song has a **Thể loại** (quê hương, bolero, ballad, pop, rock, rap, EDM, acoustic, thiếu
nhi, thánh ca, nhạc cách mạng, or none) and a **Ngôn ngữ hát** (Vietnamese by default), chosen
in Bài hát mới or at the top of step 1. The UI stays Vietnamese.

- The agents are told both; the arrangement still follows the reference song and brief.
- The genre's suggested snippets appear first among the prompt chips.
- Canh lời aligns in the sung language; Kiểm tra lục bát only shows for Vietnamese lyrics
  whose lines mostly have 6–9 syllables.
- The scene planner writes an `image_style` for the whole song, seeded by the genre.
- **Kiểu video** (step Xem trước): Truyền thống, Hiện đại or Cổ điển. Picking a genre sets
  its look and karaoke colour; changing it makes the stills and video stale.

Languages without spaces between words (Chinese, Japanese) align and wipe a whole line at a
time rather than word by word.

## Prompt snippets

Cài đặt → **Prompt mở rộng** holds reusable instructions (for example "Không bộ gõ"). They
appear as one-click chips under every agent note, the free-task box, the skill change box and
the brief, after the current song's genre suggestions; clicking appends the text.

## Image provider and keys

Cài đặt → **Tạo ảnh** picks the provider: the current hub (OpenAI-compatible, default),
OpenAI direct or Google Gemini, with a model per provider, size (16:9, default 2048x1152) and
quality. Keys are pasted per provider, stored in the macOS keychain, shown only as their last
four characters, and checked with **Kiểm tra** (a free model listing). The scene-image run is
the only child that receives a key, through its environment when it starts; agents and every
other run have image key variables removed, and keys never appear in run history or logs.
Anything running under the same macOS account can still read the keychain, so use keys with
a spending limit. Gemini support follows Google's documented API but has not been tried with
a real key yet.

## Agents

Agent runs are headless, so they cannot ask for permission:

- **Toàn quyền** (default) maps to `claude --permission-mode bypassPermissions`,
  `codex exec --dangerously-bypass-approvals-and-sandbox`, and omp's default print mode.
- **Chỉ sửa file** maps to `acceptEdits` / `--sandbox workspace-write`; scripts will not run.
  omp has no such mode, so omp runs are refused while it is selected.

The console shows each agent's replies, thinking and tool calls with their output. omp's computer-use calls appear as **CUA** with the agent's stated intent and the exact code it ran ("Agent điền form Suno" drives Chrome through the `cua_repl` MCP server of the spawned agent, never pressing Create).

Every step prompt tells the agent to read the skill first, not to ask questions (open questions
go to `suno/notes.md`), never to print keys, and to answer in Vietnamese. The skill page sends
free-form change requests to an agent working inside the skill folder, which follows
ak-skill-creator's update flow and runs the validators.

Stopping a run
sends SIGTERM to its whole process group, then SIGKILL after 6 s. A run ends when its main
process exits, even if a detached child still holds its output open (3 s grace).

## Security model

The studio drives agents with broad permissions, so it only accepts requests that come from its
own page:

- listens on loopback only and refuses any other address;
- the `Host` header must be `127.0.0.1`, `localhost` or `[::1]` on its port (DNS rebinding);
- every POST/PUT/DELETE carries a per-launch token from the page and, when present, a same-origin
  `Origin` (CSRF); the page's CSP allows only its own scripts;
- requests a browser marks `Sec-Fetch-Site: cross-site` or `same-site` are refused, so other
  pages cannot even probe media or trigger read-only endpoints;
- `project.json` and skill edits carry the hash they were based on and are refused if an agent
  changed the file meanwhile;
- file access is confined to the skill folder and the songs folder, symlinks included.

Opening the studio from a phone or another computer is deliberately unsupported: it would
need a LAN listener plus a login. Small screens are covered by the responsive layout (checked
at 390 px and 768 px in Chrome) for a narrow browser window on the Mac.

## Reliability

- **Restart keeps the queue.** Quitting the studio leaves queued runs queued, puts running
  scripts back in the queue (they are safe to repeat) and marks running agents
  "Bị gián đoạn"; the next start carries on with the chain.
- **Time limits.** Agents and renders stop after 90 minutes, other heavy scripts after 45,
  light scripts after 30; the run fails with "quá giới hạn …".
- **No half files.** Videos, trimmed audio, timings, sheets and images are written to a
  `.part` file and renamed when complete.
- **Image errors.** Network errors, 429 and 5xx are retried with backoff; a content refusal
  or a rejected key stops that scene at once with the reason in Vietnamese.
- **Locked Python.** `scripts/requirements-video.lock` pins every package with hashes, so a
  friend's Mac gets the same environment.
- **Kiểm tra máy** (top of Cài đặt) checks ffmpeg, uv, MuseScore, the skill, the Python
  environment, model caches, agents, free disk space (warns under 5 GB) and the image key,
  each with how to fix it. **Xuất gói chẩn đoán** downloads a zip with those checks, the
  settings, the studio log tail and recent runs, keys redacted.

- Chain links skip per-step preconditions (for example "no timings yet") because the link
  before them produces those inputs; each script checks again when it actually runs.
- Every list in a song's data is an array, never `null`, and each page sits inside an error
  boundary: a broken view shows "Trang này gặp lỗi" with a retry instead of blanking the app.
- Layout follows the width of the main column (CSS container queries), so an open console never
  squeezes the song header or steps.
- After `make install`, an already-open tab shows "Studio vừa được cập nhật" with a reload button
  (the page compares its UI build hash with the server's).
- Progress bars from tools such as Demucs are filtered out of the run log.

## Verified end to end (2026-10-24)

Run on the installed app against a real song ("Ninh Bình – Đẹp Tựa Vần Thơ"):

- **Tạo prompt Suno** chain with two reference songs and a brief (Claude Haiku): analysis and
  lyric review in parallel, then arrangement; about 1.5 minutes.
- **Agent điền form Suno** (omp): filled suno.com/create through CUA in about 4 minutes, values
  verified, Create left for the user.
- **Làm video tự động** clicked in the UI: align → trim → scenes agent → images → stills → render
  produced a 1080p video in about 12 minutes; re-run on the final build skipped finished steps
  (images → stills → render) and a second click was refused while the chain ran.
- Every page of two songs (20 views) loads without JavaScript errors; no horizontal overflow at
  390 px and 768 px (mobile emulation); dark and light themes checked; `go test -race` passes.

Not covered: a physical phone (only emulation) and Suno's own generation (pressing Create is
left to the user by design).

## Troubleshooting

- **A CLI shows "không thấy"** in the sidebar: set its path in Cài đặt, or re-run `make install`
  from a terminal where the CLI works.
- **Canh lời fails with "No embedded lyric track"**: the audio is not Suno's M4A (WAV/MP3 carry
  no timed lyrics). Download M4A from Suno.
- **The first video step is slow**: `setup-video-env.sh` installs PyTorch, Whisper and Demucs
  once into `~/.local/share/suno-que-huong/`; later runs reuse it.
- **Port busy**: another program owns 4317. Change `addr` in the config file.
- **Images fail with "khoá API bị từ chối"**: paste the key again in Cài đặt → Tạo ảnh and
  press Kiểm tra. **"bị từ chối vì nội dung"**: reword that scene's prompt in project.json.
- **Something else is wrong**: Cài đặt → Kiểm tra máy shows what is missing; attach the
  diagnostics zip when asking for help.

## Development

Swift shell in `macapp/` (SwiftPM, AppKit + WebKit + Sparkle; `swift build -c release`).
Go stdlib server (`main.go`, `server.go`, `runs.go`, `parsers.go`, `projects.go`, `actions.go`,
`agents.go`, `skillfs.go`, `skillsync.go`, `security.go`, `secrets.go`, `doctor.go`, `genres.go`,
`config.go`) with the web UI embedded from `web/`
(Preact 10.29.8 + htm 3.1.1 as local ES modules, no build step). Parser tests replay recorded
CLI output from `testdata/`.
