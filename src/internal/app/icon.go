package editorapp

import (
	"bytes"
	"embed"
	"image"
	"image/draw"
	_ "image/png"
	"runtime"
	"strings"

	gioapp "gioui.org/app"
)

//go:generate go run ./icongen icons
//go:embed icons/icon_bg_*.png
var iconFiles embed.FS

// iconColors are the available icon backgrounds, in the order the
// SIMPLENVIM_BG_<COLOR> variables are checked.
var iconColors = []string{"blue", "green", "yellow", "red", "orange", "purple", "pink", "brown", "black", "white", "gray"}

const (
	defaultIconColor = "black"
	iconEnvPrefix    = "SIMPLENVIM_BG_"
)

// iconPNG returns the embedded icon for icon (a color, optionally with an
// _<n> instance number), falling back to the default.
func iconPNG(icon string) []byte {
	data, err := iconFiles.ReadFile("icons/icon_bg_" + icon + ".png")
	if err != nil {
		data, _ = iconFiles.ReadFile("icons/icon_bg_" + defaultIconColor + ".png")
	}
	return data
}

// startupIconColor picks the icon color from a SIMPLENVIM_BG_<COLOR>
// variable, else the default.
func startupIconColor(environ []string) string {
	if c := envIconColor(environ); c != "" {
		return c
	}
	return defaultIconColor
}

// envIconColor returns the color of the first SIMPLENVIM_BG_<COLOR>
// variable set in environ (names compared case-insensitively), or "".
func envIconColor(environ []string) string {
	set := map[string]bool{}
	for _, kv := range environ {
		name, _, _ := strings.Cut(kv, "=")
		if len(name) > len(iconEnvPrefix) && strings.EqualFold(name[:len(iconEnvPrefix)], iconEnvPrefix) {
			set[strings.ToLower(name[len(iconEnvPrefix):])] = true
		}
	}
	for _, c := range iconColors {
		if set[c] {
			return c
		}
	}
	return ""
}

// appID is the X11 WM_CLASS / Wayland app_id that desktops match to a
// .desktop file (and so to its icon) for icon, e.g. simplenvim-green-2.
func appID(icon string) string {
	if icon == defaultIconColor {
		return "simplenvim"
	}
	return "simplenvim-" + strings.ReplaceAll(icon, "_", "-")
}

// setAppIdentity gives a non-default icon its own desktop identity so
// taskbars show and group it separately. It must run before any window
// exists: none of these identities can be changed on an existing window.
func setAppIdentity(color string) {
	if color == defaultIconColor {
		return
	}
	switch runtime.GOOS {
	case "linux", "freebsd", "openbsd":
		gioapp.ID = appID(color)
	case "windows":
		setProcessAppID(color)
	}
}

func decodeIcon(data []byte) *image.NRGBA {
	src, _, _ := image.Decode(bytes.NewReader(data))
	img, ok := src.(*image.NRGBA)
	if ok {
		return img
	}
	b := src.Bounds()
	img = image.NewNRGBA(b)
	draw.Draw(img, b, src, b.Min, draw.Src)
	return img
}
