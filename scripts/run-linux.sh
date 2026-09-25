#!/usr/bin/env bash
# Build SimpleNvimEditor and run it from build/ without installing it.
#
# On Wayland the dock looks the icon up by app ID in an installed .desktop
# file, so an uninstalled build shows a generic one. X11 (or Xwayland) takes
# the window's own icon instead, so this script runs on X11 when it can.
# SIMPLENVIM_WAYLAND=1 keeps Wayland.
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$PWD"

echo "==> Building"
mkdir -p "$ROOT/build"
(cd src && go build -o "$ROOT/build/simplenvim" ./cmd/simplenvim)

if [ -n "${WAYLAND_DISPLAY:-}" ] && [ -n "${DISPLAY:-}" ] && [ -z "${SIMPLENVIM_WAYLAND:-}" ]; then
  echo "==> Running on Xwayland"
  # Empty, not unset: libwayland would otherwise fall back to wayland-0.
  export WAYLAND_DISPLAY=
fi

exec "$ROOT/build/simplenvim" "$@"
