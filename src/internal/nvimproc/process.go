// Package nvimproc owns the lifecycle of the backend Nvim process: spawning
// it, attaching the UI, and shuttling `redraw` notification batches out to
// whoever wants to apply them to a uistate.State. It knows nothing about
// Gio or rendering.
package nvimproc

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/neovim/go-client/nvim"
)

// forceQuitAfter is how long a quit request may stay unanswered before a
// repeated one (a second Cmd+Q / Alt+F4) is allowed to kill Nvim.
const forceQuitAfter = time.Second

// probeTimeout bounds the nvim_get_mode liveness probe. nvim_get_mode is a
// "fast" request Nvim answers even mid-prompt, so silence means its event
// loop is not running at all.
const probeTimeout = time.Second

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

	// cmds serializes outgoing calls (Input, InputMouse, OpenFile,
	// RequestQuit) through a single goroutine. This matters: each of
	// those methods is called from Gio's UI goroutine and previously
	// fired its RPC call on its own throwaway goroutine, which let
	// concurrent msgpack-rpc requests race and arrive at Nvim out of
	// order (visible as scrambled keystrokes when typing quickly). A
	// single FIFO queue guarantees they reach Nvim in the order the user
	// produced them, while still keeping the caller (e.g. the input
	// handler) non-blocking -- the queue is unbounded (see funcQueue), so
	// even a completely hung Nvim can never stall the UI goroutine.
	cmds *funcQueue

	// slow runs requests Nvim defers while it waits for input (hit-enter,
	// more-prompt): nvim_ui_try_resize, nvim_command. On cmds they would block
	// the very <CR> that dismisses the prompt, freezing the editor.
	slow *funcQueue

	// quitAt is when the outstanding `confirm qa` was requested (Unix
	// nanoseconds), or 0 when none is in flight. See RequestQuit.
	quitAt atomic.Int64

	// kill terminates the child process. See forceQuit.
	kill context.CancelFunc

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

	// Cancelling ctx makes exec.CommandContext kill the child: the only
	// Cancelling ctx kills the child when Nvim stops responding. On Linux,
	// it also closes the RPC read pipe so a child of Nvim cannot hold Serve
	// open after Nvim has been killed.
	ctx, kill := context.WithCancel(context.Background())
	v, waitChild, err := startChild(ctx, command, args)
	if err != nil {
		kill()
		return nil, fmt.Errorf("spawn nvim: %w", err)
	}

	p := &Process{
		Nvim:   v,
		Redraw: make(chan [][]interface{}),
		queue:  newRedrawQueue(),
		Exited: make(chan struct{}),
		cmds:   newFuncQueue(),
		slow:   newFuncQueue(),
		kill:   kill,
	}
	p.registerHandlers()
	go p.runCmds()
	go p.runSlow()

	go func() {
		p.ServeErr = v.Serve()
		waitChild()
		kill()
		// Closing the queue lets forwardRedraw drain what is left and
		// then return, which closes Redraw and ends the consumer's range
		// loop. Without this both goroutines would block forever.
		p.queue.close()
		p.cmds.close()
		p.slow.close()
		close(p.Exited)
	}()

	if err := v.AttachUI(cols, rows, UIOptions); err != nil {
		p.forceQuit()
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
func (p *Process) runCmds() { run(p.cmds) }

// runSlow executes deferrable requests in order; see the slow field.
func (p *Process) runSlow() { run(p.slow) }

func run(q *funcQueue) {
	for {
		fn, ok := q.pop()
		if !ok {
			return
		}
		fn()
	}
}

// Input forwards a string already in Nvim's `<...>` key-notation to the
// editor. It is fire-and-forget from the caller's perspective (queued, not
// blocking); Nvim surfaces real problems (like a bad mapping) through its
// own UI anyway.
func (p *Process) Input(keys string) {
	p.cmds.push(func() { _, _ = p.Nvim.Input(keys) })
}

// InputMouse forwards a mouse event. See `:h nvim_input_mouse` for the
// button/action/modifier vocabulary.
func (p *Process) InputMouse(button, action, modifier string, grid, row, col int) {
	p.cmds.push(func() { _ = p.Nvim.InputMouse(button, action, modifier, grid, row, col) })
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
	p.slow.push(func() {
		p.resizeMu.Lock()
		cols, rows := p.pendingCols, p.pendingRows
		p.resizeQueued = false
		p.resizeMu.Unlock()
		p.applyResize(cols, rows)
	})
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
	// Handed off via cmds so it still lands after input already queued.
	p.cmds.push(func() {
		p.slow.push(func() { _ = p.Nvim.Command("edit " + vimEscape(path)) })
	})
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
//
// A repeated request while an earlier one has gone unanswered for
// forceQuitAfter escalates: unless Nvim is waiting at a prompt for the
// user's answer, it is killed. Without that, a Nvim that is wedged (a
// runaway Lua loop, a synchronous system() call that never returns) would
// hold the window open forever, since `confirm qa` can never run.
func (p *Process) RequestQuit() {
	now := time.Now().UnixNano()
	for {
		at := p.quitAt.Load()
		if at != 0 {
			if time.Duration(now-at) >= forceQuitAfter {
				go p.forceQuitUnlessPrompting()
			}
			return
		}
		if p.quitAt.CompareAndSwap(0, now) {
			break
		}
	}
	// Queued like any other command so it lands after the input already
	// on its way, but run off the queue: `confirm qa` does not return
	// until the user answers the prompt, and their answer is keyboard
	// input that travels through this very queue. Waiting here would
	// deadlock the editor on its own question.
	p.cmds.push(func() {
		go func() {
			_ = p.Nvim.Command("confirm qa")
			// Answered (e.g. (C)ancel): a later request starts afresh
			// rather than counting as a repeat.
			p.quitAt.Store(0)
		}()
	})
}

// forceQuitUnlessPrompting kills Nvim unless it is blocked waiting for the
// user -- a "Save changes?" or hit-enter prompt, which the user can answer
// and which must not be destroyed along with their unsaved work.
func (p *Process) forceQuitUnlessPrompting() {
	if p.prompting(probeTimeout) {
		return
	}
	p.forceQuit()
}

// prompting reports whether Nvim answered nvim_get_mode within timeout
// while waiting at a prompt: mode "r" (hit-enter), "rm" (more), or "r?"
// (confirm -- reported with blocking=false while it runs inside our RPC
// request). The blocking flag alone does not identify a prompt: Nvim can
// also be waiting for a synchronous command to finish. No answer (event
// loop wedged) or an ordinary mode means the user cannot get out through Nvim.
func (p *Process) prompting(timeout time.Duration) bool {
	res := make(chan bool, 1)
	go func() {
		m, err := p.Nvim.Mode()
		res <- err == nil && strings.HasPrefix(m.Mode, "r")
	}()
	select {
	case blocking := <-res:
		return blocking
	case <-time.After(timeout):
		return false
	}
}

// forceQuit kills the child and closes the RPC connection. Either one ends
// Serve, which closes Exited and Redraw and so the window.
func (p *Process) forceQuit() {
	if p.kill != nil {
		p.kill()
	}
	go func() { _ = p.Nvim.Close() }()
}
