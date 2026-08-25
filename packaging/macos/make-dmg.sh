#!/usr/bin/env bash
# Build a minimal .app bundle around the simplenvim binary and wrap it in a
# .dmg. Uses only hdiutil, which ships with macOS -- no paid tooling.
#
# Expects (from package.yml):
#   VERSION  release version without the leading v, e.g. 1.2.3
#   ARCH     amd64 | arm64
#   incoming/  containing the .tar.gz produced by build-matrix.yml
#
# The result is UNSIGNED and NOT notarized (Phase 1). Gatekeeper will warn on
# first launch; the README documents the `xattr -c` workaround.
set -euo pipefail

: "${VERSION:?VERSION must be set}"
: "${ARCH:?ARCH must be set}"

APP_NAME="SimpleNvimEditor"
BUNDLE="${APP_NAME}.app"

rm -rf build dist
mkdir -p build dist

echo "==> Extracting binary"
tar xzf incoming/*.tar.gz --strip-components=1 -C build
chmod +x build/simplenvim

echo "==> Assembling ${BUNDLE}"
mkdir -p "build/${BUNDLE}/Contents/MacOS"
mkdir -p "build/${BUNDLE}/Contents/Resources"
mv build/simplenvim "build/${BUNDLE}/Contents/MacOS/simplenvim"

# Build an .icns icon set from the embedded icon.png.
ICONSET="build/AppIcon.iconset"
mkdir -p "${ICONSET}"
ICON_SRC="src/internal/app/icon.png"
for sz in 16 32 64 128 256 512; do
  sips -z ${sz} ${sz} "${ICON_SRC}" --out "${ICONSET}/icon_${sz}x${sz}.png" >/dev/null
  dbl=$((sz * 2))
  sips -z ${dbl} ${dbl} "${ICON_SRC}" --out "${ICONSET}/icon_${sz}x${sz}@2x.png" >/dev/null
done
iconutil -c icns "${ICONSET}" -o "build/${BUNDLE}/Contents/Resources/AppIcon.icns"

cat > "build/${BUNDLE}/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key>              <string>${APP_NAME}</string>
  <key>CFBundleDisplayName</key>       <string>${APP_NAME}</string>
  <key>CFBundleIdentifier</key>        <string>io.github.kgfly.simplenvimeditor</string>
  <key>CFBundleVersion</key>           <string>${VERSION}</string>
  <key>CFBundleShortVersionString</key><string>${VERSION}</string>
  <key>CFBundleExecutable</key>        <string>simplenvim</string>
  <key>CFBundlePackageType</key>       <string>APPL</string>
  <key>CFBundleSignature</key>         <string>????</string>
  <key>CFBundleIconFile</key>          <string>AppIcon</string>
  <key>LSMinimumSystemVersion</key>    <string>11.0</string>
  <key>NSHighResolutionCapable</key>   <true/>
  <key>NSSupportsAutomaticGraphicsSwitching</key><true/>
</dict>
</plist>
PLIST

# Ad-hoc sign the bundle. This is NOT Gatekeeper notarization (that needs a
# paid Apple account); it establishes the app's *identity*, which is a
# separate thing and matters functionally:
#
# Without this step the bundle inherits the Go linker's default ad-hoc
# signature, which reports `Identifier=a.out` and leaves Info.plist unsealed
# ("Info.plist=not bound"), so `codesign --verify --strict` fails outright.
# macOS ties per-app services -- notably Dictation/NSTextInputContext, which
# looks the app up by bundle identity -- to a verifiable signature, so an
# unsigned bundle can silently lose voice typing.
#
# `-s -` means ad-hoc (no certificate required), so this works in CI with no
# secrets and no paid account.
echo "==> Signing ${BUNDLE} (ad-hoc)"
codesign --force --deep --sign - \
  --identifier io.github.kgfly.simplenvimeditor \
  "build/${BUNDLE}"
codesign --verify --strict "build/${BUNDLE}"
codesign -dvv "build/${BUNDLE}" 2>&1 | grep -E '^Identifier|Info.plist'

echo "==> Building .dmg"
STAGE="build/dmg-root"
mkdir -p "${STAGE}"
cp -R "build/${BUNDLE}" "${STAGE}/"
ln -s /Applications "${STAGE}/Applications"
cp LICENSE README.md "${STAGE}/" 2>/dev/null || true

DMG="dist/simplenvim_${VERSION}_darwin_${ARCH}.dmg"
hdiutil create \
  -volname "${APP_NAME}" \
  -srcfolder "${STAGE}" \
  -ov -format UDZO \
  "${DMG}"

echo "==> Done"
ls -lh "${DMG}"
