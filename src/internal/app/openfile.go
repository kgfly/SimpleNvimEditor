package editorapp

import (
	"bufio"
	"io"
	"net/url"
	"runtime"
	"strings"
	"sync"
)

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
	wake  func()
}{}

// queueOpenFile records a path to be opened as soon as the editor can.
// Safe to call from any thread, at any point in the lifecycle.
func queueOpenFile(path string) {
	if path == "" {
		return
	}
	pendingOpens.mu.Lock()
	pendingOpens.paths = append(pendingOpens.paths, path)
	wake := pendingOpens.wake
	pendingOpens.mu.Unlock()
	if wake != nil {
		wake()
	}
}

func setOpenFileWake(wake func()) {
	pendingOpens.mu.Lock()
	pendingOpens.wake = wake
	pendingOpens.mu.Unlock()
}

// queueDroppedURIList decodes the text/uri-list format used by X11 and
// Wayland file managers and adds local files to the normal desktop-open
// queue. Invalid entries and non-file URLs are ignored independently so one
// bad item cannot discard the rest of a multi-file drop.
func queueDroppedURIList(data io.Reader) {
	scanner := bufio.NewScanner(io.LimitReader(data, 4<<20))
	scanner.Buffer(make([]byte, 4096), 4<<20)
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fileURL, err := url.Parse(line)
		if err != nil || !strings.EqualFold(fileURL.Scheme, "file") ||
			fileURL.Path == "" || fileURL.User != nil ||
			fileURL.RawQuery != "" || fileURL.Fragment != "" ||
			strings.ContainsRune(fileURL.Path, '\x00') {
			continue
		}

		path := fileURL.Path
		if runtime.GOOS == "windows" {
			if fileURL.Host != "" && fileURL.Host != "localhost" {
				path = `\\` + fileURL.Host + strings.ReplaceAll(path, "/", `\`)
			} else if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
				path = path[1:]
			}
			path = strings.ReplaceAll(path, "/", `\`)
		} else if fileURL.Host != "" && fileURL.Host != "localhost" {
			continue
		}

		queueOpenFile(path)
	}
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
