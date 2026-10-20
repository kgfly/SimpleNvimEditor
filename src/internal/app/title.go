package editorapp

import "strconv"

// colorCodes are the two-letter title prefixes for each icon color.
var colorCodes = map[string]string{
	"blue":   "BU",
	"green":  "GN",
	"yellow": "YE",
	"red":    "RE",
	"orange": "OR",
	"purple": "PU",
	"pink":   "PI",
	"brown":  "BR",
	"black":  "BK",
	"white":  "WH",
	"gray":   "GY",
}

// titlePrefix returns the window-title prefix for an instance of color in
// slot, e.g. "RE1 ", or "RE " when the instance has no number.
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
