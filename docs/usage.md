# Quê Hương Studio

Local web workspace for the `suno-que-huong` skill. Each song goes through eight steps, from
lyrics to a 1080p karaoke video. Every step either runs one of the skill's scripts directly or
hands the work to a coding agent (Claude Code, omp or Codex), and the studio shows the agent's
work live: its replies, thinking, each tool call with its output, cost and duration.

## Install and open

```bash
cd ~/Coding/Personal/que-huong-studio
make install        # tests, builds, installs ~/.local/bin/que-huong-studio and ~/Applications/Quê Hương Studio.app
```

Open **Quê Hương Studio** from Spotlight or the Dock (ready in about a second), or run
`que-huong-studio` in a terminal. A second launch only brings the open studio to the front.
Stop it with the ■ button at the bottom of the sidebar, or Ctrl-C in the terminal; running jobs
are stopped first.

Re-run `make install` from a normal terminal after changing PATH (new Homebrew, nvm or uv
location): the app bakes in the installing shell's PATH because Finder starts apps without one.

| Command | What it does |
|---|---|
| `make build` | `bin/que-huong-studio` (about 5 s cold, under 1 s warm) |
| `make test` | gofmt, `go vet`, Go tests |
| `make run` | build and start in the terminal |
| `make uninstall` | remove the binary and the app |

Flags: `-addr 127.0.0.1:4317`, `-no-open`, `-config`, `-data`, `-version`.

## Where things live

| Path | Content |
|---|---|
| `~/Music/suno-que-huong/<song>/` | one folder per song: `project.json`, `lyrics.txt`, `suno/`, audio, `img/`, `work/`, the MP4 |
| `~/.claude/skills/suno-que-huong/` | the skill the agents and scripts follow |
| `~/.config/que-huong-studio/config.json` | settings (default agent, permission mode, concurrency, CLI paths, folders) |
| `~/.local/share/que-huong-studio/runs/<id>/` | each run: `meta.json`, `events.jsonl`, raw `stdout.log`; the newest 300 are kept |
| `~/.local/share/que-huong-studio/studio.log` | server log when started from the app (rotated to `studio.log.1` past 5 MB) |
| `~/.config/suno-que-huong/aihub.key` | image API key read by the image script (mode 600); the studio never shows it |

## Workflow steps

| Step | Done when | Actions |
|---|---|---|
| Lời, bài mẫu & yêu cầu | `lyrics.txt` has text | edit, lục bát check, reference songs (drop, pick from Downloads or paste a path; copied to `ref/`, analysed into `ref/<file>.analysis.txt`), the brief (`brief.md`) with prompt snippets, the arrangement profile (read from the skill, or chosen automatically), then **Tạo prompt Suno** |
| Prompt Suno | `suno/lyrics.txt` and `suno/style.txt` exist and are newer than the lyrics, brief and analyses | chain status, copy/edit blocks, "why this arrangement" (`suno/profile.md`), agent: fill suno.com/create via CUA (never presses Create), agent: save the arrangement to the skill as a new profile |
| Nhạc từ Suno | `project.json` `audio` points at a file | drop the M4A, pick a recent Downloads file, or paste a path |
| Canh lời | `work/timings.json` exists | script `karaoke-align.py` |
| Rút ngắn intro | intro cut, or the voice already enters before 15 s | script `trim-intro.py` (a no-op when nothing needs cutting) |
| Cảnh & ảnh AI | every non-reused scene has `img/<id>.png` | agent: plan scenes; script: generate missing images |
| Xem trước | stills exist and were rendered from the current inputs | script: render stills (`still auto` picks the checkpoints) |
| Xuất video | an MP4 exists and was rendered from the current inputs | **Làm video tự động** (whole chain), or render only |

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
trim → scenes agent → images → stills → render and skips links that are already done. The
console shows the whole chain above the timeline; click a link to follow it.

Jobs are either heavy (vocal separation, alignment, analysis, stills, render: default 1 at a
time) or light (agents and image downloads: default 3 in parallel). Both limits are in Cài đặt.
Default credits for new songs and the image model are there too.

## Prompt snippets

Cài đặt → **Prompt mở rộng** holds reusable instructions (for example "Không bộ gõ", "Dân ca
mượt mà"). They appear as one-click chips under every agent note, the free-task box, the skill
change box and the brief; clicking appends the text.

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
- every POST/PUT carries a per-launch token from the page and, when present, a same-origin
  `Origin` (CSRF); the page's CSP allows only its own scripts;
- requests a browser marks `Sec-Fetch-Site: cross-site` or `same-site` are refused, so other
  pages cannot even probe media or trigger read-only endpoints;
- `project.json` and skill edits carry the hash they were based on and are refused if an agent
  changed the file meanwhile;
- file access is confined to the skill folder and the songs folder, symlinks included.

## Reliability

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

## Development

Go stdlib server (`main.go`, `server.go`, `runs.go`, `parsers.go`, `projects.go`, `actions.go`,
`agents.go`, `skillfs.go`, `security.go`, `config.go`) with the web UI embedded from `web/`
(Preact 10.29.8 + htm 3.1.1 as local ES modules, no build step). Parser tests replay recorded
CLI output from `testdata/`.
