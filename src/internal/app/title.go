package editorapp

import "strconv"

// colorCodes are the two-letter title prefixes for each icon color.
var colorCodes = map[string]string{
	"blue":   "bu",
	"green":  "gn",
	"yellow": "ye",
	"red":    "re",
	"orange": "or",
	"purple": "pu",
	"pink":   "pi",
	"brown":  "br",
	"black":  "bk",
	"white":  "wh",
	"gray":   "gy",
}

// titlePrefix returns the window-title prefix for an instance of color in
// slot, e.g. "re1 ", or "re " when the instance has no number.
func titlePrefix(color string, slot int) string {
	code, ok := colorCodes[color]
	if !ok {
		return ""
	}
	if slot >= 1 && slot <= maxSlot {
		code += strconv.Itoa(slot)
	}
	return code + " "
}

// windowTitle prepends prefix to Nvim's title, using the app name while
// Nvim has not sent a title yet.
func windowTitle(prefix, title string) string {
	if prefix == "" {
		return title
	}
	if title == "" {
		title = "SimpleNvimEditor"
	}
	return prefix + title
}
