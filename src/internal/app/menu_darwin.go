//go:build darwin

package editorapp

/*
#cgo CFLAGS: -x objective-c -fmodules -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>

// Declaration only; defined in menu_darwin.c.
void snv_install_app_menu(const char *version);
*/
import "C"

import "unsafe"

// installAppMenu adds "About SimpleNvimEditor" to the application menu so the
// version is discoverable from a Finder/Dock launch, where `simplenvim
// --version` is unavailable because the binary is not on $PATH.
func installAppMenu(version string) {
	if version == "" {
		version = "dev"
	}
	cVersion := C.CString(version)
	defer C.free(unsafe.Pointer(cVersion))
	C.snv_install_app_menu(cVersion)
}
