package editorapp

import (
	"sync"

	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

// sideButtons carries back/forward mouse button events from native hooks
// (sidebutton_darwin.c) that Gio's backend drops, to the editor's event loop.
// The handoff mirrors closeRequests.
type sideButtonEvent struct {
	button string // "x1" or "x2"
	action string // "press" or "release"
}

var sideButtons = struct {
	mu      sync.Mutex
	pending []sideButtonEvent
	wake    func()
}{}

// queueSideButton is safe to call from any thread.
func queueSideButton(button, action string) {
	sideButtons.mu.Lock()
	sideButtons.pending = append(sideButtons.pending, sideButtonEvent{button, action})
	wake := sideButtons.wake
	sideButtons.mu.Unlock()
	if wake != nil {
		go wake()
	}
}

func setSideButtonWake(wake func()) {
	sideButtons.mu.Lock()
	sideButtons.wake = wake
	sideButtons.mu.Unlock()
}

func (a *App) drainSideButtons() {
	sideButtons.mu.Lock()
	events := sideButtons.pending
	sideButtons.pending = nil
	sideButtons.mu.Unlock()
	if a.proc == nil || len(events) == 0 {
		return
	}
	grid, row, col := 1, a.hoverRow, a.hoverCol
	if g, gr, gc, ok := uistate.HitTest(a.state.Snapshot().Windows, row, col); ok {
		grid, row, col = g, gr, gc
	}
	for _, e := range events {
		a.proc.InputMouse(e.button, e.action, "", grid, row, col)
	}
}
