//go:build darwin

package editorapp

/*
#cgo CFLAGS: -x objective-c -fmodules -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdint.h>

// Declarations only; defined in close_darwin.c.
void snv_install_close_handler(uintptr_t view);
void snv_allow_close(void);
*/
import "C"

import gioapp "gioui.org/app"

//export snv_onCloseRequest
func snv_onCloseRequest() {
	queueCloseRequest()
}

func installCloseHandler(view any) {
	event, ok := view.(gioapp.AppKitViewEvent)
	if !ok || event.View == 0 {
		return
	}
	C.snv_install_close_handler(C.uintptr_t(event.View))
}

func allowClose() {
	C.snv_allow_close()
}
