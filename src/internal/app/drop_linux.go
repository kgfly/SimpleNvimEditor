//go:build linux

package editorapp

/*
#cgo pkg-config: x11 wayland-client

// Declarations only; the implementations live in drop_x11.c and
// drop_wayland.c. A cgo preamble is prepended to every translation unit in
// the package, so defining them here would duplicate the symbols.
void snv_x11_drop_loop(unsigned long window);
void snv_wl_drop_loop(void *display, void *surface);
*/
import "C"

import (
	"bytes"
	"sync"
	"unsafe"

	gioapp "gioui.org/app"
)

//export snv_onDroppedURIList
func snv_onDroppedURIList(data *C.char, length C.int) {
	queueDroppedURIList(bytes.NewReader(C.GoBytes(unsafe.Pointer(data), length)))
}

// dropTargetOnce keeps the drop listener to one per process. Gio re-sends
// ViewEvent for the same window, and each listener owns a thread and (on
// X11) a connection.
var dropTargetOnce sync.Once

// installDropTarget starts a listener for whichever display server Gio
// ended up on. Both run on their own thread and block there for the
// lifetime of the process.
func installDropTarget(view any) {
	switch e := view.(type) {
	case gioapp.X11ViewEvent:
		if e.Window == 0 {
			return
		}
		window := C.ulong(e.Window)
		dropTargetOnce.Do(func() {
			go func() { C.snv_x11_drop_loop(window) }()
		})
	case gioapp.WaylandViewEvent:
		if e.Display == nil || e.Surface == nil {
			return
		}
		display, surface := e.Display, e.Surface
		dropTargetOnce.Do(func() {
			go func() { C.snv_wl_drop_loop(display, surface) }()
		})
	}
}
