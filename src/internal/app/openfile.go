package editorapp

import "sync"

// pendingOpens holds file paths the desktop environment has asked us to
// open, until the editor is ready to act on them.
//
// A queue is required rather than a direct call, for two reasons:
//
//   - The request can arrive before Nvim exists. On macOS the Apple Event
//     that carries the filename is delivered during application launch,
//     which is well before the first frame has run and spawned the child
//     process. Dropping it there is what makes an app appear to open with
//     an empty buffer.
//   - It crosses threads. The platform delivers the path on the AppKit main
//     thread, while Nvim is driven from Gio's event loop.
var pendingOpens = struct {
	mu    sync.Mutex
	paths []string
}{}

// queueOpenFile records a path to be opened as soon as the editor can.
// Safe to call from any thread, at any point in the lifecycle.
func queueOpenFile(path string) {
	if path == "" {
		return
	}
	pendingOpens.mu.Lock()
	defer pendingOpens.mu.Unlock()
	pendingOpens.paths = append(pendingOpens.paths, path)
}

// takeQueuedOpens removes and returns every queued path.
func takeQueuedOpens() []string {
	pendingOpens.mu.Lock()
	defer pendingOpens.mu.Unlock()
	if len(pendingOpens.paths) == 0 {
		return nil
	}
	paths := pendingOpens.paths
	pendingOpens.paths = nil
	return paths
}

// drainOpenRequests opens any files the desktop environment has requested
// since the last frame. It is a no-op until Nvim is running, and the paths
// stay queued until then.
func (a *App) drainOpenRequests() {
	if a.proc == nil {
		return
	}
	for _, path := range takeQueuedOpens() {
		a.proc.OpenFile(path)
	}
}
