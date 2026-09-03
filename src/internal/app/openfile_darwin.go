//go:build darwin

package editorapp

/*
#cgo CFLAGS: -x objective-c -fmodules -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

// Declarations only. The implementation lives in openfile_darwin.m --
// this preamble is prepended to every translation unit cgo generates for
// the package, so defining the class or function here would compile them
// more than once and fail the link with duplicate symbols.
void snv_install_open_file_handler(void);
*/
import "C"

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
