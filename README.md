# Song Studio

Song Studio is a macOS app that turns lyrics into a finished song. It takes a song from its lyrics to a Suno track, a 1080p karaoke video and a lead sheet. It is built around the `suno-que-huong` skill.

Each song moves through eight steps. A step either runs one of the skill's scripts or hands the work to a coding agent: Claude Code, Codex or omp. The studio shows the agent working live, including its replies, thinking, tool calls, cost and duration. A chat panel lets you keep talking to an agent across turns.

## Features

- **Eight-step workflow.** It covers lyrics, the Suno prompt, the Suno audio, word-level lyric alignment, scenes and AI images, frame previews, the karaoke video and the sheet music.
- **Agent chat.** Open it from the bubble at the bottom right or with Cmd+J.
  - Each message resumes the agent's previous session, so it keeps the context and can ask back.
  - Messages you send while the agent is working wait in a queue. **Stop & send** interrupts the current turn instead.
  - **Reply** continues the session of any finished step run.
- **Run queue that survives restarts.** Heavy jobs and agents get separate capacity, and a timeout ends each run.
- **Image providers.** It supports any OpenAI-compatible endpoint, OpenAI and Gemini. API keys are kept in the macOS Keychain.
- **Bilingual interface.** The app is in English or Vietnamese, and agents reply in the language you choose.
- **Self-updating.** Sparkle installs new versions from this repo's [Releases](https://github.com/BillyND/song-studio/releases) when you quit the app.

## Install

Song Studio needs an Apple Silicon Mac with macOS 13 or later.

1. Download the newest `SongStudio-<version>.zip` from [Releases](https://github.com/BillyND/song-studio/releases).
2. Unzip it and drag **Song Studio** into **Applications**.
3. Open the app. It is not signed by Apple, so macOS blocks it the first time. Go to **System Settings → Privacy & Security**, click **Open Anyway**, then open the app again.
4. In the app, open **Settings → System check**. It lists anything still missing, such as ffmpeg, uv, MuseScore or an image API key.

The interface can be switched between English and Vietnamese under **Settings → Language**.

## Updates

The app checks for updates every hour and installs the new version when you quit. To update right away, choose **Song Studio → Check for Updates…**. Versions up to 1.0.15 were published in [song-studio-releases](https://github.com/BillyND/song-studio-releases). Those copies update from there once, to a version that follows this repo.

## Build from source

```bash
make install   # gofmt, vet and tests, then builds out/Song Studio.app and installs it to ~/Applications
make run       # the Go server alone, served on 127.0.0.1:4317
make test
```

The project has two parts:

- **Server:** a Go standard-library HTTP server. It embeds the Preact UI from `web/`.
- **Shell:** a Swift app in `macapp/`, built with AppKit, WKWebView and Sparkle 2.

Packaging and release scripts are in `packaging/`.

## Docs

[docs/usage.md](docs/usage.md) covers the workflow, where data lives, agents, security, reliability and troubleshooting.
