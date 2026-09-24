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

//go:embed icon_bg_*.png
var iconFiles embed.FS

// iconColors are the available icon backgrounds, in the order the
// SIMPLENVIM_BG_<COLOR> variables are checked.
var iconColors = []string{"blue", "green", "yellow", "red", "orange", "purple", "pink", "brown", "black", "white", "gray"}

const (
	defaultIconColor  = "blue"
	terminalIconColor = "green"
	iconEnvPrefix     = "SIMPLENVIM_BG_"
)

// iconPNG returns the embedded icon for color, falling back to the default.
func iconPNG(color string) []byte {
	data, err := iconFiles.ReadFile("icon_bg_" + color + ".png")
	if err != nil {
		data, _ = iconFiles.ReadFile("icon_bg_" + defaultIconColor + ".png")
	}
	return data
}

// startupIconColor picks the icon color from a SIMPLENVIM_BG_<COLOR>
// variable, else green when Nvim is expected to open in a terminal.
func startupIconColor(environ, nvimArgs []string) (color string, fromEnv bool) {
	if c := envIconColor(environ); c != "" {
		return c, true
	}
	if startsInTerminal(nvimArgs) {
		return terminalIconColor, false
	}
	return defaultIconColor, false
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

// startsInTerminal predicts from Nvim's arguments whether its first buffer
// will be a terminal: no file is opened and a -c/+ command runs :terminal.
// Nvim's own report after startup corrects any misprediction.
func startsInTerminal(args []string) bool {
	term := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			return term && i == len(args)-1
		case arg == "-c":
			if i+1 < len(args) {
				i++
				term = term || isTerminalCmd(args[i])
			}
		case strings.HasPrefix(arg, "+"):
			term = term || isTerminalCmd(arg[1:])
		case arg == "--cmd" || arg == "-u" || arg == "-i" || arg == "-s" || arg == "-w" ||
			arg == "-W" || arg == "--listen" || arg == "--startuptime":
			i++
		case arg == "-" || arg == "-t" || arg == "-q" || arg == "-S":
			// These all load a buffer of their own before any -c command.
			return false
		case strings.HasPrefix(arg, "-"):
		default:
			return false
		}
	}
	return term
}

// isTerminalCmd reports whether cmd is an Ex :ter[minal] command.
func isTerminalCmd(cmd string) bool {
	cmd = strings.TrimLeft(cmd, " \t:")
	end := strings.IndexFunc(cmd, func(r rune) bool { return r < 'a' || r > 'z' })
	if end >= 0 {
		cmd = cmd[:end]
	}
	return len(cmd) >= 3 && strings.HasPrefix("terminal", cmd)
}

// appID is the X11 WM_CLASS / Wayland app_id that desktops match to a
// .desktop file (and so to its icon) for color.
func appID(color string) string {
	if color == defaultIconColor {
		return "simplenvim"
	}
	return "simplenvim-" + color
}

// setAppIdentity gives a non-default color its own desktop identity so
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
