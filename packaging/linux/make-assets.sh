#!/usr/bin/env bash
# Generate the Linux desktop assets from src/internal/app/icon_bg_*.png:
#   build/linux/share/icons/hicolor/<N>x<N>/apps/simplenvim[-<color>].png
#   build/linux/share/applications/simplenvim-<color>.desktop
# Each non-default color gets an icon and a hidden .desktop entry named after
# the app ID the editor uses for it (X11 WM_CLASS / Wayland app_id), so docks
# and taskbars that look icons up by app ID show the right color.
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

for src in src/internal/app/icon_bg_*.png; do
  color="$(basename "${src}" .png)"
  color="${color#icon_bg_}"
  name="simplenvim-${color}"
  [ "${color}" = "blue" ] && name="simplenvim"

  for sz in 16 32 48 64 128 256 512; do
    dir="${OUT}/icons/hicolor/${sz}x${sz}/apps"
    mkdir -p "${dir}"
    "${IM[@]}" "${src}" -resize "${sz}x${sz}" "PNG32:${dir}/${name}.png"
  done

  [ "${color}" = "blue" ] && continue
  upper="$(echo "${color}" | tr '[:lower:]' '[:upper:]')"
  cat > "${OUT}/applications/${name}.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=SimpleNvimEditor (${color})
Comment=A simple, fast, native Neovim GUI
Exec=env SIMPLENVIM_BA_${upper}=1 simplenvim %F
Icon=${name}
StartupWMClass=${name}
Terminal=false
NoDisplay=true
EOF
done

find "${OUT}" -type f | sort
