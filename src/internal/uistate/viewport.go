package uistate

import "reflect"

// Viewport is one window's scroll position, per the `win_viewport` event.
// Nvim sends it for every window, terminals included, whenever the visible
// range or the buffer's line count changes.
type Viewport struct {
	// Win is the window handle, for addressing the window over RPC.
	Win int
	// Topline is the 0-based first visible buffer line.
	Topline int
	// Botline is the 0-based line just past the last visible one. It can
	// exceed LineCount when the window shows the end of the buffer.
	Botline int
	// LineCount is the number of lines in the window's buffer.
	LineCount int
}

// Visible is how many buffer lines the window currently shows.
func (v Viewport) Visible() int {
	return max(0, min(v.Botline, v.LineCount)-v.Topline)
}

// applyWinViewport handles `win_viewport`:
// [grid, win, topline, botline, curline, curcol, line_count, scroll_delta].
func (s *State) applyWinViewport(args []interface{}) {
	for _, a := range args {
		t := toSlice(a)
		if len(t) < 7 {
			continue
		}
		s.viewports[toInt(t[0])] = Viewport{
			Win:       toHandle(t[1]),
			Topline:   toInt(t[2]),
			Botline:   toInt(t[3]),
			LineCount: toInt(t[6]),
		}
	}
}

// toHandle reads a window/buffer handle. The RPC layer decodes msgpack ext
// types into its own named integer types (e.g. nvim.Window), which this
// package deliberately doesn't import, so match on the underlying kind.
func toHandle(v interface{}) int {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return int(rv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int(rv.Uint())
	}
	return 0
}
