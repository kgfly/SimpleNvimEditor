//go:build darwin

package editorapp

/*
#cgo CFLAGS: -x objective-c -fmodules -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

// Declaration only; defined in icon_darwin.c.
void snv_set_app_icon(const void *data, int len, int preferBundle);
*/
import "C"

import "unsafe"

// setWindowIcon sets the Dock icon; macOS has no per-window icon. The
// default color keeps the bundle's own AppIcon.icns when there is one.
func setWindowIcon(_ any, color string) {
	preferBundle := 0
	if color == defaultIconColor {
		preferBundle = 1
	}
	png := iconPNG(color)
	C.snv_set_app_icon(unsafe.Pointer(&png[0]), C.int(len(png)), C.int(preferBundle))
}
