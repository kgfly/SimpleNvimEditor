package input

import "gioui.org/io/pointer"

// MouseButtonFor picks the primary/secondary/tertiary button implicated by
// a pointer event, in the vocabulary nvim_input_mouse expects. Returns ""
// for events (e.g. plain Move) that have no associated button.
func MouseButtonFor(e pointer.Event) string {
	switch {
	case e.Buttons.Contain(pointer.ButtonPrimary):
		return "left"
	case e.Buttons.Contain(pointer.ButtonSecondary):
		return "right"
	case e.Buttons.Contain(pointer.ButtonTertiary):
		return "middle"
	default:
		return ""
	}
}

// MouseAction maps a pointer.Kind to the nvim_input_mouse action string.
func MouseAction(kind pointer.Kind) string {
	switch kind {
	case pointer.Press:
		return "press"
	case pointer.Release:
		return "release"
	case pointer.Drag:
		return "drag"
	default:
		return ""
	}
}

// ScrollDirection turns a pointer.Event's Scroll delta into the ("wheel",
// action) pair nvim_input_mouse wants for wheel events. ok is false if
// there was no meaningful scroll delta on either axis.
func ScrollDirection(e pointer.Event) (action string, ok bool) {
	switch {
	case e.Scroll.Y < 0:
		return "up", true
	case e.Scroll.Y > 0:
		return "down", true
	case e.Scroll.X < 0:
		return "left", true
	case e.Scroll.X > 0:
		return "right", true
	default:
		return "", false
	}
}
