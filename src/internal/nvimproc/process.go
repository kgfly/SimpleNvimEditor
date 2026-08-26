// Package nvimproc owns the lifecycle of the backend Nvim process: spawning
// it, attaching the UI, and shuttling `redraw` notification batches out to
// whoever wants to apply them to a uistate.State. It knows nothing about
// Gio or rendering.
package nvimproc

import (
	"fmt"

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
	// the screen is consistent again). It is buffered so a burst of
	// events doesn't stall the msgpack-rpc read loop; callers should
	// drain it promptly.
	Redraw chan [][]interface{}

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
}

// Spawn starts `command --embed [extraArgs...] [userArgs...]` as a child
// process and attaches a UI to it with the given initial grid size.
//
// extraArgs come from the config file; userArgs come from the command line
// (pass-through arguments and files alike, already in the order Nvim should
// see them). Both are forwarded verbatim: this package deliberately does
// not interpret Nvim's own flag vocabulary, so `-c`, `-O`, `-u` and friends
// keep working without changes here.
//
// command is resolved the same way on every OS: exec.Command performs a
// PATH lookup, so "nvim" works on Linux, macOS, and Windows alike as long
// as the binary is installed and on PATH (or an absolute path is given).
func Spawn(command string, extraArgs, userArgs []string, cols, rows int) (*Process, error) {
	args := make([]string, 0, len(extraArgs)+len(userArgs)+1)
	args = append(args, "--embed")
	args = append(args, extraArgs...)
	args = append(args, userArgs...)

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
		Redraw: make(chan [][]interface{}, 256),
		Exited: make(chan struct{}),
		cmds:   make(chan func(), 1024),
	}
	p.registerHandlers()
	go p.runCmds()

	go func() {
		p.ServeErr = v.Serve()
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
		select {
		case p.Redraw <- updates:
		default:
			// The consumer is falling behind; drop the oldest batch rather
			// than blocking the RPC read loop indefinitely.
			select {
			case <-p.Redraw:
			default:
			}
			p.Redraw <- updates
		}
	})
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
func (p *Process) Resize(cols, rows int) {
	p.cmds <- func() { _ = p.Nvim.TryResizeUI(cols, rows) }
}

// RequestQuit asks Nvim to quit, honoring unsaved-changes prompts. Because
// this client doesn't yet render Nvim's confirmation dialog specially, an
// interactive "Save changes?" prompt will appear as normal grid text.
func (p *Process) RequestQuit() {
	p.cmds <- func() { _ = p.Nvim.Command("confirm qa") }
}
