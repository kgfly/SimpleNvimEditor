package editorapp

import (
	"image"

	"gioui.org/io/pointer"

	"github.com/kgfly/SimpleNvimEditor/internal/input"
	"github.com/kgfly/SimpleNvimEditor/internal/render"
	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

// scrollDrag is a left-button press on a scrollbar that hasn't been
// released yet. bar is the geometry at press time: mapping the pointer
// through it keeps the thumb under the pointer even while Nvim's view,
// and so the current frame's bar, lags behind.
type scrollDrag struct {
	bar render.Scrollbar
	// grab is how far below the thumb's top edge the pointer holds it.
	grab int
}

// scrollbars returns the bars to draw and hit-test, or nil when the
// editor.scrollbar setting is off.
func (a *App) scrollbars(snap uistate.Snapshot) []render.Scrollbar {
	if !a.cfg.Editor.Scrollbar {
		return nil
	}
	return render.Scrollbars(snap, a.fonts.Metrics)
}

// onScrollbarPointer handles the left button on a scrollbar and reports
// whether it consumed e. Pressing the thumb starts a drag; pressing the
// track elsewhere jumps so the thumb is centred on the pointer, and can be
// dragged on from there. Neither moves focus to the scrolled window. Nvim
// never sees these events, so it doesn't also click the text underneath.
func (a *App) onScrollbarPointer(e pointer.Event, snap uistate.Snapshot) bool {
	pos := image.Pt(int(e.Position.X), int(e.Position.Y))
	switch e.Kind {
	case pointer.Press:
		if input.MouseButtonFor(e) != "left" {
			return false
		}
		for _, b := range a.scrollbars(snap) {
			if !pos.In(b.Track) {
				continue
			}
			d := &scrollDrag{bar: b, grab: pos.Y - b.Thumb.Min.Y}
			if !pos.In(b.Thumb) {
				d.grab = b.Thumb.Dy() / 2
				a.proc.ScrollWindow(b.Win, b.ToplineAt(pos.Y-d.grab))
			}
			a.scrollDrag = d
			return true
		}
	case pointer.Drag:
		if d := a.scrollDrag; d != nil {
			a.proc.ScrollWindow(d.bar.Win, d.bar.ToplineAt(pos.Y-d.grab))
			return true
		}
	case pointer.Release, pointer.Cancel:
		if a.scrollDrag != nil {
			a.scrollDrag = nil
			return true
		}
	}
	return false
}
