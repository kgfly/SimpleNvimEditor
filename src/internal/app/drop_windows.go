//go:build windows && cgo

package editorapp

/*
#cgo windows LDFLAGS: -lshell32 -luser32

#include <stdint.h>

void snv_install_drop_target(uintptr_t hwnd);
*/
import "C"

import gioapp "gioui.org/app"

//export snv_onDroppedFile
func snv_onDroppedFile(path *C.char) {
	queueOpenFile(C.GoString(path))
}

func installDropTarget(view any) {
	event, ok := view.(gioapp.Win32ViewEvent)
	if !ok || event.HWND == 0 {
		return
	}
	C.snv_install_drop_target(C.uintptr_t(event.HWND))
}
