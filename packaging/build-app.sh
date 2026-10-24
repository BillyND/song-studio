#!/usr/bin/env bash
# Assemble "out/Quê Hương Studio.app": the Swift shell, the Go server, Sparkle and a snapshot of the skill.
# Usage: packaging/build-app.sh <version> <skill-dir>
set -euo pipefail
VERSION="$1"
SKILL="$2"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/out"
APP="$OUT/Quê Hương Studio.app"
KEY="$(tr -d '\n' < "$ROOT/packaging/sparkle-public-key.txt")"

[ -f "$SKILL/SKILL.md" ] || { echo "skill not found at $SKILL" >&2; exit 1; }
mkdir -p "$OUT"
(cd "$ROOT" && go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$OUT/que-huong-studio" .)
(cd "$ROOT/macapp" && swift build -c release --arch arm64 >&2)
SWIFT_OUT="$(cd "$ROOT/macapp" && swift build -c release --arch arm64 --show-bin-path)"

rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources" "$APP/Contents/Frameworks"
sed -e "s#__VERSION__#$VERSION#g" -e "s#__SPARKLE_KEY__#$KEY#g" "$ROOT/packaging/Info.plist" > "$APP/Contents/Info.plist"
plutil -lint "$APP/Contents/Info.plist" >/dev/null
cp "$SWIFT_OUT/QueHuongStudio" "$APP/Contents/MacOS/QueHuongStudio"
cp "$OUT/que-huong-studio" "$APP/Contents/MacOS/que-huong-studio"
ditto "$SWIFT_OUT/Sparkle.framework" "$APP/Contents/Frameworks/Sparkle.framework"
cp "$ROOT/packaging/AppIcon.icns" "$APP/Contents/Resources/AppIcon.icns"
# the skill snapshot, without caches or work notes; the server installs it on first start
rsync -a --delete --exclude '__pycache__' --exclude '*.pyc' --exclude '.DS_Store' --exclude '/plans' --exclude '.git' \
  "$SKILL/" "$APP/Contents/Resources/skill/"

# ad-hoc signatures (no Apple account): inner code first, then the app
codesign --force --sign - "$APP/Contents/Frameworks/Sparkle.framework/Versions/B/XPCServices/Installer.xpc"
codesign --force --sign - "$APP/Contents/Frameworks/Sparkle.framework/Versions/B/XPCServices/Downloader.xpc"
codesign --force --sign - "$APP/Contents/Frameworks/Sparkle.framework/Versions/B/Autoupdate"
codesign --force --sign - "$APP/Contents/Frameworks/Sparkle.framework/Versions/B/Updater.app"
codesign --force --sign - "$APP/Contents/Frameworks/Sparkle.framework"
codesign --force --sign - "$APP/Contents/MacOS/que-huong-studio"
codesign --force --sign - "$APP"
codesign --verify --deep --strict "$APP"
echo "$APP"
