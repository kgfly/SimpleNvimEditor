//go:build darwin

package editorapp

/*
#cgo CFLAGS: -x objective-c -fmodules -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

// Declaration only; defined in sidebutton_darwin.c.
void snv_install_side_buttons(void);
*/
import "C"

import gioapp "gioui.org/app"

//export snv_onSideButton
func snv_onSideButton(button C.int, press C.int) {
	name := "x1"
	if button == 4 {
		name = "x2"
	}
	action := "release"
	if press != 0 {
		action = "press"
	}
	queueSideButton(name, action)
}

// installSideButtons forwards mouse back/forward buttons, which Gio's macOS
// backend maps to no button at all. See sidebutton_darwin.c.
func installSideButtons(view any) {
	event, ok := view.(gioapp.AppKitViewEvent)
	if !ok || event.View == 0 {
		return
	}
	C.snv_install_side_buttons()
}
