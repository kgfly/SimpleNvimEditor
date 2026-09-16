//go:build darwin

package editorapp

/*
#cgo CFLAGS: -x objective-c -fmodules -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdint.h>

// Declarations only. The implementation lives in openfile_darwin.c --
// this preamble is prepended to every translation unit cgo generates for
// the package, so defining the class or function here would compile them
// more than once and fail the link with duplicate symbols.
void snv_install_open_file_handler(void);
void snv_install_drop_target(uintptr_t view);
*/
import "C"

import gioapp "gioui.org/app"

//export snv_onOpenFile
func snv_onOpenFile(path *C.char) {
	queueOpenFile(C.GoString(path))
}

// InstallOpenFileHandler subscribes to Finder's "open document" events.
//
// It must be called before the app finishes launching, because the event
// for the file that *caused* the launch is delivered during startup: a
// handler installed after the first frame would miss it entirely, which
// looks exactly like the app ignoring the file it was asked to open.
func InstallOpenFileHandler() {
	C.snv_install_open_file_handler()
}

func installDropTarget(view any) {
	event, ok := view.(gioapp.AppKitViewEvent)
	if !ok || event.View == 0 {
		return
	}
	C.snv_install_drop_target(C.uintptr_t(event.View))
}
