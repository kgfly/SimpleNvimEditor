// Package nvimproc owns the lifecycle of the backend Nvim process: spawning
// it, attaching the UI, and shuttling `redraw` notification batches out to
// whoever wants to apply them to a uistate.State. It knows nothing about
// Gio or rendering.
package nvimproc

import (
	"fmt"
	"strings"
	"sync"

	"github.com/neovim/go-client/nvim"
)

// UIOptions are the `nvim_ui_attach` options this client always requests.
// ext_linegrid is not listed because modern Nvim enables it unconditionally
// for any UI that attaches; ext_cmdline/ext_messages/ext_popupmenu/
// ext_tabline are deliberately left off for the MVP so Nvim draws those
// directly into the grid like a normal terminal UI (see
// IMPLEMENTATION_PLAN.md Phase 3 for externalizing them).
var UIOptions = map[string]interface{}{
	"rgb":           true,
	"ext_multigrid": true,
	"ext_hlstate":   true,
}

// Process wraps a running Nvim instance attached as a UI client.
type Process struct {
	Nvim *nvim.Nvim

	// Redraw receives every `redraw` notification as it arrives: each
	// item is a full batch of [event-name, args...] tuples, exactly as
	// Nvim grouped them (a batch always ends with a "flush" tuple once
	// the screen is consistent again).
	//
	// Every batch is delivered; none are ever dropped, however far behind
	// the consumer falls. Batches are incremental and Nvim never resends
	// them, so discarding one corrupts the display permanently. Backlog
	// is absorbed by an unbounded queue instead (see redrawQueue).
	Redraw chan [][]interface{}

	// queue buffers batches between the RPC read loop and Redraw.
	queue *redrawQueue

	// Exited is closed when the Serve loop returns, i.e. when the Nvim
	// process (or its stdio pipes) has gone away.
	Exited chan struct{}
	// ServeErr is set (before Exited is closed) if Serve returned an
	// error rather than a clean shutdown.
	ServeErr error

	// cmds serializes every outgoing call (Input, InputMouse, Resize,
	// RequestQuit) through a single goroutine. This matters: each of
	// those methods is called from Gio's UI goroutine and previously
	// fired its RPC call on its own throwaway goroutine, which let
	// concurrent msgpack-rpc requests race and arrive at Nvim out of
	// order (visible as scrambled keystrokes when typing quickly). A
	// single FIFO queue guarantees they reach Nvim in the order the user
	// produced them, while still keeping the caller (e.g. the input
	// handler) non-blocking.
	cmds chan func()

	// resizeMu guards the coalescing state below. A window drag produces
	// far more resizes than Nvim can usefully apply; see Resize.
	resizeMu     sync.Mutex
	pendingCols  int
	pendingRows  int
	resizeQueued bool

	// resizeFn, when non-nil, replaces the TryResizeUI round trip. Only
	// tests set it; see applyResize.
	resizeFn func(cols, rows int)
}

// Spawn starts `command --embed [extraArgs...] [nvimArgs...]` as a child
// process and attaches a UI to it with the given initial grid size.
//
// command is resolved the same way on every OS: exec.Command performs a
// PATH lookup, so "nvim" works on Linux, macOS, and Windows alike as long
// as the binary is installed and on PATH (or an absolute path is given).
func Spawn(command string, extraArgs, nvimArgs []string, cols, rows int) (*Process, error) {
	args := make([]string, 0, len(extraArgs)+len(nvimArgs)+2)
	args = append(args, "--embed")
	args = append(args, extraArgs...)
	args = append(args, nvimArgs...)

	v, err := nvim.NewChildProcess(
		nvim.ChildProcessCommand(command),
		nvim.ChildProcessArgs(args...),
		nvim.ChildProcessServe(false),
	)
	if err != nil {
		return nil, fmt.Errorf("spawn nvim: %w", err)
	}

	p := &Process{
		Nvim:   v,
		Redraw: make(chan [][]interface{}),
		queue:  newRedrawQueue(),
		Exited: make(chan struct{}),
		cmds:   make(chan func(), 1024),
	}
	p.registerHandlers()
	go p.runCmds()

	go func() {
		p.ServeErr = v.Serve()
		// Closing the queue lets forwardRedraw drain what is left and
		// then return, which closes Redraw and ends the consumer's range
		// loop. Without this both goroutines would block forever.
		p.queue.close()
		close(p.Exited)
	}()

	if err := v.AttachUI(cols, rows, UIOptions); err != nil {
		return nil, fmt.Errorf("attach ui: %w", err)
	}
	return p, nil
}

// registerHandlers wires the "redraw" msgpack-rpc notification (the one
// Nvim sends for every UI update) to the Redraw channel. Each call carries
// one whole batch of event tuples; see the Redraw field doc.
func (p *Process) registerHandlers() {
	_ = p.Nvim.RegisterHandler("redraw", func(updates ...[]interface{}) {
		// Queue rather than send directly: the queue grows instead of
		// blocking the RPC read loop, and unlike a fixed channel it can
		// never discard a batch. See redrawQueue for why dropping one is
		// unrecoverable.
		p.queue.push(updates)
	})
	go p.forwardRedraw()
}

// forwardRedraw moves queued batches onto the Redraw channel, which keeps
// the public API a plain channel that callers can range over.
func (p *Process) forwardRedraw() {
	defer close(p.Redraw)
	for {
		batch, ok := p.queue.pop()
		if !ok {
			return
		}
		p.Redraw <- batch
	}
}

// runCmds executes queued outgoing calls one at a time, in submission
// order, for the lifetime of the process.
func (p *Process) runCmds() {
	for fn := range p.cmds {
		fn()
	}
}

// Input forwards a string already in Nvim's `<...>` key-notation to the
// editor. It is fire-and-forget from the caller's perspective (queued, not
// blocking); Nvim surfaces real problems (like a bad mapping) through its
// own UI anyway.
func (p *Process) Input(keys string) {
	p.cmds <- func() { _, _ = p.Nvim.Input(keys) }
}

// InputMouse forwards a mouse event. See `:h nvim_input_mouse` for the
// button/action/modifier vocabulary.
func (p *Process) InputMouse(button, action, modifier string, grid, row, col int) {
	p.cmds <- func() { _ = p.Nvim.InputMouse(button, action, modifier, grid, row, col) }
}

// Resize asks Nvim to change the size of the base grid.
//
// Dragging a window edge produces a new size every frame -- roughly fifty
// of them for one gesture -- and each TryResizeUI is a blocking round trip
// that also makes Nvim reflow and redraw the whole screen. Sending all of
// them queues hundreds of milliseconds of work whose results are obsolete
// on arrival, so the grid visibly lags the window edge and keeps repainting
// after the mouse stops.
//
// Only the latest size matters, so a pending resize that has not started
// yet is replaced rather than queued behind. The size is read at execution
// time (not captured), which is what lets a later call overwrite it.
func (p *Process) Resize(cols, rows int) {
	p.resizeMu.Lock()
	defer p.resizeMu.Unlock()

	p.pendingCols, p.pendingRows = cols, rows
	if p.resizeQueued {
		return
	}
	p.resizeQueued = true
	p.cmds <- func() {
		p.resizeMu.Lock()
		cols, rows := p.pendingCols, p.pendingRows
		p.resizeQueued = false
		p.resizeMu.Unlock()
		p.applyResize(cols, rows)
	}
}

// applyResize performs the actual resize round trip. It is a field-backed
// indirection purely so tests can observe coalescing without a live Nvim.
func (p *Process) applyResize(cols, rows int) {
	if p.resizeFn != nil {
		p.resizeFn(cols, rows)
		return
	}
	_ = p.Nvim.TryResizeUI(cols, rows)
}

// OpenFile tells Nvim to edit the given path in the current window.
//
// The path is sent as a command rather than as keystrokes because it is
// untrusted input: a filename can contain characters that nvim_input would
// interpret as key notation ("<Esc>"), and typing it would also depend on
// the editor's current mode. `:edit` takes the name as data.
//
// fnameescape is applied inside Nvim so that spaces, '#', '%' and other
// characters with meaning to the command line are treated literally --
// doing it here would mean reimplementing Vim's escaping rules in Go.
func (p *Process) OpenFile(path string) {
	if path == "" {
		return
	}
	p.cmds <- func() {
		_ = p.Nvim.Command("edit " + vimEscape(path))
	}
}

// vimEscape quotes path for use inside a Vim command line by deferring to
// Nvim's own fnameescape() at evaluation time.
func vimEscape(path string) string {
	// Single-quoted Vim strings are literal; the only escape is a doubled
	// quote. Wrapping in fnameescape() then handles command-line metachars.
	return "`=fnameescape('" + strings.ReplaceAll(path, "'", "''") + "')`"
}

// RequestQuit asks Nvim to quit, honoring unsaved-changes prompts. Because
// this client doesn't yet render Nvim's confirmation dialog specially, an
// interactive "Save changes?" prompt will appear as normal grid text.
func (p *Process) RequestQuit() {
	p.cmds <- func() { _ = p.Nvim.Command("confirm qa") }
}
