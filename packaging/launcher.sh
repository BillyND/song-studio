#!/bin/bash
# Entry point inside "Quê Hương Studio.app". Finder starts apps with a bare PATH, so `make install`
# bakes in the PATH of the shell that installed it (Homebrew, uv, nvm node for codex, ~/.local/bin).
export PATH="__STUDIO_PATH__:$PATH"
LOG="$HOME/.local/share/que-huong-studio/studio.log"
mkdir -p "$(dirname "$LOG")"
exec "$HOME/.local/bin/que-huong-studio" "$@" >>"$LOG" 2>&1
