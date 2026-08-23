// Package editorapp wires together configuration, the Nvim backend process,
// the uistate protocol model, and Gio's window/event loop into a running
// editor. It is the only package that imports both gioui.org and
// nvimproc/uistate — see IMPLEMENTATION_PLAN.md §4 for why that separation
// matters (rendering never blocks on RPC, and vice versa).
package editorapp

import (
	"image"
	"time"

	gioapp "gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/kgfly/SimpleNvimEditor/internal/config"
	"github.com/kgfly/SimpleNvimEditor/internal/input"
	"github.com/kgfly/SimpleNvimEditor/internal/nvimproc"
	"github.com/kgfly/SimpleNvimEditor/internal/render"
	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

// rootTag identifies the single, whole-window input target. Later phases
// that add real widgets (cmdline, popupmenu, ...) will introduce their own
// tags; the MVP only needs one.
var rootTag = new(int)

// anyModifier is every modifier Gio can report. It is used as the Optional
// set of our key filters so that modified keypresses reach us at all: Gio
// discards events carrying modifiers outside a filter's Required|Optional
// set, and Nvim needs to see the full range ("<C-w>", "<A-x>", "<D-v>",
// and shifted glyphs like ":").
const anyModifier = key.ModCtrl | key.ModCommand | key.ModShift | key.ModAlt | key.ModSuper

// App owns everything needed to run one editor window.
type App struct {
	cfg   config.Config
	files []string

	win   *gioapp.Window
	fonts render.Fonts
	state *uistate.State
	proc  *nvimproc.Process

	cols, rows int
	focused    bool
	title      string
	view       any
}

// New creates an App that will open the given files (may be empty).
func New(cfg config.Config, files []string) *App {
	return &App{
		cfg:   cfg,
		files: files,
		state: uistate.New(),
	}
}

// Run drives win's event loop until the window is closed. It blocks the
// calling goroutine, matching Gio's own convention (see gioui.org/app doc).
func (a *App) Run(win *gioapp.Window) error {
	a.win = win
	win.Option(
		gioapp.Size(unit.Dp(1000), unit.Dp(650)),
	)

	a.fonts = render.Fonts{
		Shaper: render.NewShaper(a.cfg.Editor),
		Face:   render.FontFace(a.cfg.Editor),
		Size:   unit.Sp(a.cfg.Editor.FontSize),
	}

	icon := appIcon()
	var ops op.Ops
	for {
		switch e := win.Event().(type) {
		case gioapp.ViewEvent:
			a.view = e
			setWindowIcon(e, icon)
		case gioapp.DestroyEvent:
			a.quit()
			return e.Err
		case gioapp.FrameEvent:
			gtx := gioapp.NewContext(&ops, e)
			a.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

// layout handles input for this frame, starts/resizes Nvim as needed, and
// paints the current uistate snapshot. Applying redraw batches happens on a
// separate goroutine (see pumpRedraw) so that a burst of Nvim output
// doesn't have to wait for a frame to be scheduled before it's processed.
func (a *App) layout(gtx layout.Context) {
	size := gtx.Constraints.Max
	if a.fonts.Metrics.CellWidth == 0 {
		a.fonts.Metrics = render.Measure(gtx, a.fonts.Shaper, a.fonts.Face, a.fonts.Size)
	}

	a.handleInput(gtx)
	a.syncSize(size)

	snap := a.state.Snapshot()
	if snap.Title != a.title {
		a.title = snap.Title
		if a.title != "" {
			a.win.Option(gioapp.Title(a.title))
			setWindowTitle(a.view, a.title)
		}
	}

	render.Frame(gtx, a.fonts, snap)
}

// InputFilters returns the event filters describing everything the editor
// window wants to receive. It is exported so tests can drive Gio's real
// filter-matching logic against the exact filters production uses.
//
// The Optional modifier set is load-bearing: Gio discards any key event
// whose modifiers fall outside a filter's Required|Optional set, so
// omitting it silently drops ":" (Shift-;), "<C-w>", and every other
// modified key before it ever reaches Nvim.
func InputFilters(tag event.Tag) []event.Filter {
	bigScroll := pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}
	return []event.Filter{
		key.FocusFilter{Target: tag},
		key.Filter{Focus: tag, Optional: anyModifier},
		key.Filter{Focus: tag, Name: key.NameTab, Optional: anyModifier},
		pointer.Filter{
			Target:  tag,
			Kinds:   pointer.Press | pointer.Release | pointer.Drag | pointer.Scroll,
			ScrollX: bigScroll,
			ScrollY: bigScroll,
		},
	}
}

// handleInput registers the whole window as one input target and forwards
// whatever key/pointer events arrive to Nvim.
func (a *App) handleInput(gtx layout.Context) {
	area := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	event.Op(gtx.Ops, rootTag)
	area.Pop()

	// Tell Gio this tag is an active text-input client so the platform
	// layer (NSTextInputClient on macOS) routes IME and voice dictation
	// through EditEvent instead of swallowing it.
	key.InputHintOp{Tag: rootTag, Hint: key.HintAny}.Add(gtx.Ops)

	if !a.focused {
		gtx.Execute(key.FocusCmd{Tag: rootTag})
	}

	filters := InputFilters(rootTag)
	for {
		e, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		switch ev := e.(type) {
		case key.FocusEvent:
			a.focused = ev.Focus
		case key.Event:
			a.onKey(ev)
		case key.EditEvent:
			a.onEdit(ev)
		case pointer.Event:
			a.onPointer(ev)
		}
	}
}

func (a *App) onKey(e key.Event) {
	if a.proc == nil {
		return
	}
	if s := input.EncodeKey(e); s != "" {
		a.proc.Input(s)
	}
}

// onEdit forwards text produced by an input method (macOS Dictation, CJK
// IME, Emoji picker, etc.) to Nvim. EditEvent.Text may contain multiple
// characters; Nvim's nvim_input accepts them as a UTF-8 byte string.
func (a *App) onEdit(e key.EditEvent) {
	if a.proc == nil || e.Text == "" {
		return
	}
	a.proc.Input(e.Text)
}

func (a *App) onPointer(e pointer.Event) {
	if a.proc == nil || a.fonts.Metrics.CellWidth == 0 {
		return
	}
	col := int(e.Position.X) / a.fonts.Metrics.CellWidth
	row := int(e.Position.Y) / a.fonts.Metrics.CellHeight
	mods := input.ModifierPrefix(e.Modifiers)

	if e.Kind == pointer.Scroll {
		if action, ok := input.ScrollDirection(e); ok {
			a.proc.InputMouse("wheel", action, mods, 1, row, col)
		}
		return
	}

	button := input.MouseButtonFor(e)
	action := input.MouseAction(e.Kind)
	if button == "" || action == "" {
		return
	}
	a.proc.InputMouse(button, action, mods, 1, row, col)
}

// syncSize computes the grid size implied by the window's pixel size and
// either starts Nvim (first frame) or asks it to resize (later frames).
func (a *App) syncSize(size image.Point) {
	cw, ch := a.fonts.Metrics.CellWidth, a.fonts.Metrics.CellHeight
	if cw == 0 || ch == 0 {
		return
	}
	cols, rows := size.X/cw, size.Y/ch
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}

	if a.proc == nil {
		a.cols, a.rows = cols, rows
		a.startNvim()
		return
	}
	if cols != a.cols || rows != a.rows {
		a.cols, a.rows = cols, rows
		a.proc.Resize(cols, rows)
	}
}

// startNvim spawns the backend process once the first frame has told us how
// big the initial grid should be, and starts the goroutine that pumps its
// redraw events into our state model.
func (a *App) startNvim() {
	proc, err := nvimproc.Spawn(a.cfg.Nvim.Command, a.cfg.Nvim.ExtraArgs, a.files, a.cols, a.rows)
	if err != nil {
		// Nothing meaningful to render without Nvim; surfacing to stderr
		// is enough for the MVP (see IMPLEMENTATION_PLAN.md for a real
		// error-toast widget in a later phase).
		panic(err)
	}
	a.proc = proc
	go a.pumpRedraw()
}

// pumpRedraw applies every `redraw` batch Nvim sends to the state model and
// asks Gio for a new frame whenever a batch ends with `flush`. It runs for
// the lifetime of the Nvim process, independent of Gio's own frame timing:
// this is what lets Nvim-driven updates (e.g. a background job printing to
// a buffer) show up without any user input to otherwise trigger a frame.
func (a *App) pumpRedraw() {
	for batch := range a.proc.Redraw {
		if a.state.Apply(batch) {
			a.win.Invalidate()
		}
	}
}

// quit asks Nvim to exit (honoring unsaved-changes prompts) and waits a
// short, bounded time for it to do so before letting the window close
// anyway. See Process.RequestQuit for the documented limitation here.
func (a *App) quit() {
	if a.proc == nil {
		return
	}
	a.proc.RequestQuit()
	select {
	case <-a.proc.Exited:
	case <-time.After(500 * time.Millisecond):
	}
}
