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
| `~/.local/share/que-huong-studio/studio.log` | server log when started from the app |
| `~/.config/suno-que-huong/aihub.key` | image API key read by the image script (mode 600); the studio never shows it |

## Workflow steps

| Step | Done when | Actions |
|---|---|---|
| Lời bài hát | `lyrics.txt` has text | edit, lục bát check, agent: review + sectioning + Styles |
| Prompt Suno | `suno/lyrics.txt` and `suno/style.txt` exist and are newer than the lyrics | copy blocks, edit, agent: fill suno.com/create via CUA (never presses Create) |
| Nhạc từ Suno | `project.json` `audio` points at a file | drop the M4A, pick a recent Downloads file, or paste a path |
| Canh lời | `work/timings.json` exists | script `karaoke-align.py` |
| Rút ngắn intro | intro cut, or the voice already enters before 15 s | script `trim-intro.py` |
| Cảnh & ảnh AI | every non-reused scene has `img/<id>.png` | agent: plan scenes; script: generate missing images |
| Xem trước | stills exist and were rendered from the current inputs | script: render stills |
| Xuất video | an MP4 exists and was rendered from the current inputs | script: full render; agent: finish all video steps |

"Current inputs" is a fingerprint of the video-relevant `project.json` keys, the timings and the
images, recorded when a stills or render run succeeds (`work/studio-stamps.json`). Editing other
keys, such as `vocal`, does not mark the video stale.

## Agents

Agent runs are headless, so they cannot ask for permission:

- **Toàn quyền** (default) maps to `claude --permission-mode bypassPermissions`,
  `codex exec --dangerously-bypass-approvals-and-sandbox`, and omp's default print mode.
- **Chỉ sửa file** maps to `acceptEdits` / `--sandbox workspace-write`; scripts will not run.
  omp has no such mode, so omp runs are refused while it is selected.

Every step prompt tells the agent to read the skill first, not to ask questions (open questions
go to `suno/notes.md`), never to print keys, and to answer in Vietnamese. The skill page sends
free-form change requests to an agent working inside the skill folder, which follows
ak-skill-creator's update flow and runs the validators.

Jobs queue: by default one runs at a time because the machine is often loaded. Stopping a run
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
