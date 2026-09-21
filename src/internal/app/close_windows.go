//go:build windows && cgo

package editorapp

/*
#cgo windows LDFLAGS: -luser32

// Declared only; defined in drop_windows.c, which owns the window-procedure
// subclass that both the drop target and the WM_CLOSE veto hang off.
void snv_allow_close(void);
*/
import "C"

//export snv_onCloseRequest
func snv_onCloseRequest() {
	queueCloseRequest()
}

// installCloseHandler is a no-op: the WM_CLOSE veto lives in the same
// window-procedure subclass that installDropTarget installs, because a
// window can only have one procedure and chaining two subclasses of our own
// would just be the same hook twice.
func installCloseHandler(_ any) {}

func allowClose() {
	C.snv_allow_close()
}
