package editorapp

import (
	"sync"

	"gioui.org/io/system"
)

// closeRequests carries "the user asked this window to close" from the
// native hooks in close_darwin.c / drop_windows.c to the editor's event
// loop.
//
// Those hooks run on the platform's UI thread and have already *cancelled*
// the close, which is the whole point: Gio has no close-request event, so
// by the time it reports DestroyEvent the window is gone and Nvim's
// "save changes?" prompt would have nowhere to appear. Cancelling first and
// asking Nvim second is what turns Cmd+Q / Alt+F4 / the close button into
// the same `:confirm qa` a user would type.
//
// The handoff mirrors pendingOpens: queue the request, then wake the event
// loop from another goroutine, because Gio's wakeup runs inline when it is
// already on the platform's UI thread and would deadlock (see
// queueOpenFile).
var closeRequests = struct {
	mu      sync.Mutex
	pending bool
	wake    func()
}{}

// queueCloseRequest records a cancelled native close. Safe to call from any
// thread.
func queueCloseRequest() {
	closeRequests.mu.Lock()
	closeRequests.pending = true
	wake := closeRequests.wake
	closeRequests.mu.Unlock()
	if wake != nil {
		go wake()
	}
}

func setCloseRequestWake(wake func()) {
	closeRequests.mu.Lock()
	closeRequests.wake = wake
	closeRequests.mu.Unlock()
}

func takeCloseRequest() bool {
	closeRequests.mu.Lock()
	defer closeRequests.mu.Unlock()
	pending := closeRequests.pending
	closeRequests.pending = false
	return pending
}

// drainCloseRequest hands a cancelled native close to Nvim as `confirm qa`.
// Nvim exiting is what actually closes the window (see pumpRedraw), so a
// modified buffer simply leaves the window open with the prompt showing.
func (a *App) drainCloseRequest() {
	if !takeCloseRequest() {
		return
	}
	if a.proc == nil {
		// Nothing to ask: honor the close the hook just cancelled.
		allowClose()
		a.win.Perform(system.ActionClose)
		return
	}
	a.proc.RequestQuit()
}
