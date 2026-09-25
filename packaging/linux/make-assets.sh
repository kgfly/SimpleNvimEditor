#!/usr/bin/env bash
# Generate the Linux desktop assets from src/internal/app/icons/icon_bg_*.png:
#   build/linux/share/icons/hicolor/<N>x<N>/apps/simplenvim[-<color>[-<n>]].png
#   build/linux/share/applications/simplenvim-<color>[-<n>].desktop
# Each icon but the default gets a hidden .desktop entry named after the app
# ID the editor uses for it (X11 WM_CLASS / Wayland app_id), so docks and
# taskbars that look icons up by app ID show the right color and number.
# Requires ImageMagick (`magick` or `convert`).
set -euo pipefail

cd "$(dirname "$0")/../.."

if command -v magick >/dev/null 2>&1; then
  IM=(magick)
elif command -v convert >/dev/null 2>&1; then
  IM=(convert)
else
  echo "ImageMagick not found (need 'magick' or 'convert')" >&2
  exit 1
fi

OUT="build/linux/share"
rm -rf "${OUT}"
mkdir -p "${OUT}/applications"

for src in src/internal/app/icons/icon_bg_*.png; do
  icon="$(basename "${src}" .png)"
  icon="${icon#icon_bg_}" # <color> or <color>_<n>
  color="${icon%%_*}"
  name="simplenvim-${icon//_/-}"
  [ "${icon}" = "black" ] && name="simplenvim"

  for sz in 16 32 48 64 128 256 512; do
    dir="${OUT}/icons/hicolor/${sz}x${sz}/apps"
    mkdir -p "${dir}"
    "${IM[@]}" "${src}" -resize "${sz}x${sz}" "PNG32:${dir}/${name}.png"
  done

  [ "${icon}" = "black" ] && continue
  env=""
  if [ "${color}" != "black" ]; then
    env="env SIMPLENVIM_BG_$(echo "${color}" | tr '[:lower:]' '[:upper:]')=1 "
  fi
  cat > "${OUT}/applications/${name}.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=SimpleNvimEditor (${icon/_/ })
Comment=A simple, fast, native Neovim GUI
Exec=${env}simplenvim %F
Icon=${name}
StartupWMClass=${name}
Terminal=false
NoDisplay=true
EOF
done

find "${OUT}" -type f | sort
