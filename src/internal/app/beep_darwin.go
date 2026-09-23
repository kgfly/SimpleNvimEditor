//go:build darwin

package editorapp

/*
#cgo CFLAGS: -x objective-c -fmodules -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdint.h>

// Declaration only; defined in beep_darwin.c.
void snv_install_beep_suppressor(uintptr_t view);
*/
import "C"

import gioapp "gioui.org/app"

// installBeepSuppressor stops AppKit from beeping on key chords (Ctrl+F and
// friends) that Cocoa's key bindings map to text-editing actions nothing in
// the app implements. See beep_darwin.c.
func installBeepSuppressor(view any) {
	event, ok := view.(gioapp.AppKitViewEvent)
	if !ok || event.View == 0 {
		return
	}
	C.snv_install_beep_suppressor(C.uintptr_t(event.View))
}
