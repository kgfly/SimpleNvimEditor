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
	// Botline is Nvim's w_botline as sent: the 1-based number of the first
	// line not fully shown, so 0-based lines Topline..Botline-2 are visible.
	// It is LineCount+1 when the window shows the end of the buffer.
	Botline int
	// LineCount is the number of lines in the window's buffer.
	LineCount int
}

// Visible is how many buffer lines the window currently shows in full.
func (v Viewport) Visible() int {
	return max(0, min(v.Botline-1, v.LineCount)-v.Topline)
}

// RowView is a window's scroll state in screen rows rather than buffer
// lines, which also covers a view part-way through a wrapped line.
type RowView struct {
	// Total is the screen rows the whole buffer takes in the window.
	Total int
	// Top is how many of them are above the window's first row.
	Top int
}

// SetRowViews replaces the row views with rows, as sent by Nvim: a list of
// [win, total, top] triples. Windows missing from it have none.
func (s *State) SetRowViews(rows interface{}) {
	views := make(map[int]RowView)
	for _, r := range toSlice(rows) {
		t := toSlice(r)
		if len(t) < 3 {
			continue
		}
		views[toHandle(t[0])] = RowView{Total: toInt(t[1]), Top: toInt(t[2])}
	}
	s.mu.Lock()
	s.rowViews = views
	s.mu.Unlock()
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
