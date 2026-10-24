#!/usr/bin/env bash
# Publish a new version: build the app, zip it, sign the zip with the Sparkle key in the keychain,
# write appcast.xml and upload both as a GitHub release of the public releases repo.
# Installed apps find it through SUFeedURL (…/releases/latest/download/appcast.xml).
# Usage: packaging/release.sh <version> <skill-dir> [notes]   (notes may also come from $NOTES)
set -euo pipefail
VERSION="$1"
SKILL="$2"
NOTES="${3:-${NOTES:-}}"
REPO="BillyND/song-studio-releases"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/out"
SIGN="$ROOT/packaging/sparkle-tools/bin/sign_update"
ZIP="$OUT/SongStudio-$VERSION.zip"

[ -x "$SIGN" ] || { echo "missing Sparkle tools: run make sparkle-tools" >&2; exit 1; }
if gh release view "v$VERSION" --repo "$REPO" >/dev/null 2>&1; then
  echo "v$VERSION is already released; bump VERSION" >&2; exit 1
fi
LATEST="$(gh release view --repo "$REPO" --json tagName -q .tagName 2>/dev/null | sed 's/^v//' || true)"
if [ -n "$LATEST" ] && [ "$(printf '%s\n%s\n' "$LATEST" "$VERSION" | sort -V | tail -1)" != "$VERSION" ]; then
  echo "v$VERSION is not newer than the published v$LATEST" >&2; exit 1
fi
if [ -z "$NOTES" ]; then # default notes: commit subjects since the previous release tag
  PREV="$(git -C "$ROOT" describe --tags --abbrev=0 --match 'v*' 2>/dev/null || true)"
  NOTES="$(git -C "$ROOT" log --format='- %s' ${PREV:+$PREV..}HEAD | head -30)"
fi

"$ROOT/packaging/build-app.sh" "$VERSION" "$SKILL" >/dev/null
rm -f "$ZIP"
ditto -c -k --sequesterRsrc --keepParent "$OUT/Song Studio.app" "$ZIP"
SIG_ATTRS="$("$SIGN" --account que-huong-studio "$ZIP")"   # sparkle:edSignature="…" length="…"
URL="https://github.com/$REPO/releases/download/v$VERSION/$(basename "$ZIP")"
NOTES_HTML="$(printf '%s\n' "$NOTES" | sed -e 's/&/\&amp;/g' -e 's/</\&lt;/g' -e 's/^- \(.*\)$/<li>\1<\/li>/')"
cat > "$OUT/appcast.xml" <<EOF
<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0" xmlns:sparkle="http://www.andymatuschak.org/xml-namespaces/sparkle">
  <channel>
    <title>Song Studio</title>
    <item>
      <title>Version $VERSION</title>
      <pubDate>$(LC_ALL=C date -u '+%a, %d %b %Y %H:%M:%S +0000')</pubDate>
      <sparkle:version>$VERSION</sparkle:version>
      <sparkle:shortVersionString>$VERSION</sparkle:shortVersionString>
      <sparkle:minimumSystemVersion>13.0</sparkle:minimumSystemVersion>
      <description><![CDATA[<ul>$NOTES_HTML</ul>]]></description>
      <enclosure url="$URL" type="application/octet-stream" $SIG_ATTRS/>
    </item>
  </channel>
</rss>
EOF
gh release create "v$VERSION" "$ZIP" "$OUT/appcast.xml" --repo "$REPO" --title "Song Studio $VERSION" --notes "$NOTES"
git -C "$ROOT" tag -f "v$VERSION" >/dev/null
echo "released v$VERSION: $URL"
