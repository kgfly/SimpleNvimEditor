//go:build darwin

package editorapp

/*
#cgo CFLAGS: -x objective-c -fmodules -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

// Declaration only; defined in icon_darwin.c.
void snv_set_app_icon(const void *data, int len);
*/
import "C"

import "unsafe"

// setWindowIcon sets the Dock icon; macOS has no per-window icon. The
// default color restores the bundle's own AppIcon.icns.
func setWindowIcon(_ any, color string) {
	if color == defaultIconColor {
		C.snv_set_app_icon(nil, 0)
		return
	}
	png := iconPNG(color)
	C.snv_set_app_icon(unsafe.Pointer(&png[0]), C.int(len(png)))
}
