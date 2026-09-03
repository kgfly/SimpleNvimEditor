#!/usr/bin/env bash
# Build SimpleNvimEditor into a signed .app and launch it.
#
# Why this exists: `go build && ./simplenvim` produces a working editor whose
# voice dictation is silently broken. macOS resolves Dictation, microphone
# permission and the Dock icon through the app's LaunchServices *bundle
# identity*, and a bare Mach-O has no Info.plist, hence no identity -- it
# inherits whatever launched it:
#
#     bare binary : CFBundleIdentifier=[ NULL ]  fileType="????"  -> no dictation
#     .app bundle : CFBundleIdentifier=io...     fileType="APPL"  -> dictation
#
# The Dock icon is the tell: the app's own icon means a real bundle identity,
# a generic/terminal icon means it is running with none.
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$PWD"
APP="$ROOT/build/SimpleNvimEditor.app"
BUNDLE_ID="io.github.kgfly.simplenvimeditor"

echo "==> Building"
mkdir -p "$ROOT/build"
(cd src && go build -o "$ROOT/build/simplenvim" ./cmd/simplenvim)

echo "==> Assembling bundle"
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
mv "$ROOT/build/simplenvim" "$APP/Contents/MacOS/simplenvim"
# Generate AppIcon.icns from the embedded PNG, same as make-dmg.sh does.
# Without this the bundle has no icon and the Dock falls back to a generic
# one -- which also happens to be the visible symptom of a missing bundle
# identity, so a missing icon here makes a working app look broken.
ICONSET="$ROOT/build/AppIcon.iconset"
rm -rf "$ICONSET"; mkdir -p "$ICONSET"
for sz in 16 32 64 128 256 512; do
  sips -z $sz $sz "$ROOT/src/internal/app/icon.png" \
    --out "$ICONSET/icon_${sz}x${sz}.png" >/dev/null
  sips -z $((sz * 2)) $((sz * 2)) "$ROOT/src/internal/app/icon.png" \
    --out "$ICONSET/icon_${sz}x${sz}@2x.png" >/dev/null
done
iconutil -c icns "$ICONSET" -o "$APP/Contents/Resources/AppIcon.icns"
rm -rf "$ICONSET"

cat > "$APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>SimpleNvimEditor</string>
  <key>CFBundleDisplayName</key><string>SimpleNvimEditor</string>
  <key>CFBundleIdentifier</key><string>${BUNDLE_ID}</string>
  <key>CFBundleExecutable</key><string>simplenvim</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleVersion</key><string>dev</string>
  <key>CFBundleShortVersionString</key><string>dev</string>
  <key>CFBundleIconFile</key><string>AppIcon</string>
  <key>CFBundleDocumentTypes</key>
  <array>
    <dict>
      <key>CFBundleTypeName</key><string>Text Documents</string>
      <key>CFBundleTypeRole</key><string>Editor</string>
      <key>CFBundleTypeExtensions</key>
      <array>
        <string>txt</string>
        <string>log</string>
        <string>t</string>
      </array>
      <key>LSHandlerRank</key><string>Alternate</string>
    </dict>
  </array>
  <key>LSMinimumSystemVersion</key><string>11.0</string>
  <key>NSHighResolutionCapable</key><true/>
  <key>NSMicrophoneUsageDescription</key><string>Voice dictation into the editor.</string>
</dict>
</plist>
PLIST
plutil -lint "$APP/Contents/Info.plist"

# Ad-hoc signing (-s -) needs no certificate and no Apple account. Without it
# the bundle keeps the Go linker's default signature (Identifier=a.out, with
# Info.plist unsealed) and `codesign --verify` fails.
echo "==> Signing"
codesign --force --deep --sign - --identifier "$BUNDLE_ID" "$APP"
codesign --verify --strict "$APP"

# Launch through LaunchServices. Running the binary directly is what silently
# breaks dictation, so never do that here.
echo "==> Launching"
open "$APP" --args "$@"

sleep 2
PID="$(pgrep -n simplenvim || true)"
if [ -n "$PID" ]; then
  ID="$(lsappinfo info -only bundleid "$(lsappinfo find pid="$PID" | head -1)" 2>/dev/null || true)"
  echo "==> Running as pid $PID"
  echo "    ${ID:-<no bundle id>}"
  case "$ID" in
    *"$BUNDLE_ID"*) echo "    identity OK -- dictation should work" ;;
    *)              echo "    WARNING: no bundle identity; dictation will NOT work" >&2 ;;
  esac
fi
