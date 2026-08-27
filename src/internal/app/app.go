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
	"gioui.org/f32"
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
	cfg      config.Config
	nvimArgs []string
	options  Options

	win   *gioapp.Window
	fonts render.Fonts
	state *uistate.State
	proc  *nvimproc.Process

	cols, rows int
	focused    bool
	title      string
	view       any

	// ime mirrors just enough document state to keep the platform's
	// input method (macOS Dictation, CJK IMEs, emoji picker) alive; see
	// ime.go.
	ime imeShadow

	// mods reconstructs the held modifier set from key events, because
	// Gio's Windows backend reports it incorrectly; see
	// input.ModifierTracker.
	mods input.ModifierTracker

	// policy decides what Alt means; see input.Policy.
	policy input.Policy
}

// Options controls how the editor window starts.
type Options struct {
	Maximized bool
}

// New creates an App that starts Nvim with the given arguments (may be empty).
func New(cfg config.Config, nvimArgs []string, options Options) *App {
	return &App{
		cfg:      cfg,
		nvimArgs: append([]string(nil), nvimArgs...),
		options:  options,
		state:    uistate.New(),
		ime:      newIMEShadow(),
		policy:   cfg.Editor.InputPolicy(),
	}
}

// Run drives win's event loop until the window is closed. It blocks the
// calling goroutine, matching Gio's own convention (see gioui.org/app doc).
func (a *App) Run(win *gioapp.Window) error {
	a.win = win
	windowOptions := []gioapp.Option{gioapp.Size(unit.Dp(1000), unit.Dp(650))}
	if a.options.Maximized {
		windowOptions = append(windowOptions, gioapp.Maximized.Option())
	}
	win.Option(windowOptions...)

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
			// A focus change ends any composition the old handler
			// owned, so start the next one from a clean slate.
			a.ime.reset()
			// Modifier releases go to whoever has focus, so the tail
			// of an Alt-Tab never reaches us. Forget what we thought
			// was held rather than treat the next plain keypress as a
			// chord.
			a.mods.Reset()
		case key.Event:
			a.onKey(ev)
		case key.EditEvent:
			a.onEdit(ev)
		case key.SnippetEvent:
			// The platform is telling us which slice of the document
			// it wants to see; our shadow is small enough to always
			// report in full, so there is nothing to recompute.
		case key.SelectionEvent:
			a.ime.setSelection(key.Range(ev))
		case key.CompositionEvent:
			a.ime.setComposing(key.Range(ev))
		case pointer.Event:
			a.onPointer(ev)
		}
	}

	// Report our (shadow) document state back to the platform every
	// frame. Without this the IME sees a document that never changes,
	// concludes the app ignored its edit, and cancels the composition —
	// which is exactly what made voice dictation appear to do nothing.
	a.ime.trimIfIdle()
	a.ime.sync(gtx, rootTag, a.caret(), image.Rectangle{})
}

// caret reports where Nvim's cursor is in window pixels, so macOS can place
// the dictation/candidate popup next to the text being edited rather than
// in the window's top-left corner.
func (a *App) caret() key.Caret {
	m := a.fonts.Metrics
	if m.CellWidth == 0 {
		return key.Caret{}
	}
	snap := a.state.Snapshot()
	origin, ok := render.GridOrigin(snap, snap.Cursor.GridID, m)
	if !ok {
		return key.Caret{}
	}
	x := float32(origin.X + snap.Cursor.Col*m.CellWidth)
	baseline := float32(origin.Y + snap.Cursor.Row*m.CellHeight + m.Baseline)
	return key.Caret{
		Pos:     f32.Pt(x, baseline),
		Ascent:  float32(m.Baseline),
		Descent: float32(m.CellHeight - m.Baseline),
	}
}

func (a *App) onKey(e key.Event) {
	// Track modifiers even before Nvim exists, so the held set can never
	// drift out of sync with the keyboard.
	e = a.mods.Key(e)
	if a.proc == nil {
		return
	}
	// Printable keys arrive twice on macOS: once as an EditEvent (via
	// NSTextInputClient's insertText:, which is also how Dictation and
	// IMEs deliver text) and once as this key.Event. X11 does the same.
	// Sending both would type every character twice, so text is handled
	// exclusively in onEdit and only non-text keys are encoded here.
	if a.policy.IsTextKey(e) {
		return
	}
	if s := input.EncodeKey(e); s != "" {
		a.proc.Input(s)
	}
}

// onEdit forwards text produced by an input method (macOS Dictation, CJK
// IME, emoji picker) or an ordinary printable keypress to Nvim.
//
// An EditEvent means "replace Range with Text". While a composition is in
// flight the input method revises its guess in place, so each update
// replaces the range it marked previously ("hello" -> "hello wor" ->
// "hello world"); an ordinary keypress carries an empty range and simply
// inserts. Nvim has no notion of provisional text, so a replacement is
// expressed by backspacing over the marked runes and typing the new
// version — in a single nvim_input call, so a redraw can never observe the
// half-deleted state.
//
// The rune count comes from Range rather than from tracking composition
// state, because the platform emits the EditEvent *before* the
// CompositionEvent that marks the range: on the first preedit update we
// would not yet know a composition had started.
func (a *App) onEdit(e key.EditEvent) {
	if a.proc == nil {
		return
	}

	// macOS composes Option-chords into text before it reports the key:
	// gio_onKeys calls interpretKeyEvents (which fires this EditEvent)
	// and only then emits the key.Event. So when Alt is Meta, Option-a
	// would insert "å" here AND fire <A-a> from onKey — the very
	// double-input that IsTextKey exists to prevent, just in the other
	// direction. The key path owns these chords, so drop the text.
	if a.altOwnsKeyPath() {
		return
	}

	stale := a.ime.replace(e.Range, e.Text)

	if keys := input.Backspaces(stale) + input.EncodeText(e.Text); keys != "" {
		a.proc.Input(keys)
	}
}

// altOwnsKeyPath reports whether an Alt-chord is currently being handled as
// a Meta keystroke, in which case any text the platform composed from it is
// a duplicate and must be discarded.
//
// A composition in flight is exempt: a CJK IME may legitimately hold Alt
// while committing, and the text it delivers is a real commit rather than
// an Option-glyph.
func (a *App) altOwnsKeyPath() bool {
	return a.policy.MetaHeld(a.mods.Held()) && !a.ime.composing()
}

func (a *App) onPointer(e pointer.Event) {
	if a.proc == nil || a.fonts.Metrics.CellWidth == 0 {
		return
	}
	col := int(e.Position.X) / a.fonts.Metrics.CellWidth
	row := int(e.Position.Y) / a.fonts.Metrics.CellHeight
	mods := input.ModifierPrefix(a.mods.Modifiers(e.Modifiers))

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
	proc, err := nvimproc.Spawn(a.cfg.Nvim.Command, a.cfg.Nvim.ExtraArgs, a.nvimArgs, a.cols, a.rows)
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
