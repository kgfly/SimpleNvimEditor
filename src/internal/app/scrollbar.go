package editorapp

import (
	"image"

	"gioui.org/io/pointer"
	"gioui.org/unit"

	"github.com/kgfly/SimpleNvimEditor/internal/input"
	"github.com/kgfly/SimpleNvimEditor/internal/render"
	"github.com/kgfly/SimpleNvimEditor/internal/uistate"
)

// scrollDrag is a left-button press on a scrollbar that hasn't been
// released yet. bar is the geometry at press time: mapping the pointer
// through it keeps the thumb under the pointer even while Nvim's view,
// and so the current frame's bar, lags behind.
type scrollDrag struct {
	bar render.Scrollbar
	// grab is how far below the thumb's top edge the pointer holds it.
	grab int
}

// scrollbars returns the bars to draw and hit-test, or nil when the
// editor.scrollbar setting is off.
func (a *App) scrollbars(snap uistate.Snapshot) []render.Scrollbar {
	if !a.cfg.Editor.Scrollbar {
		return nil
	}
	return render.Scrollbars(snap, a.fonts.Metrics, a.metric.Dp(unit.Dp(a.cfg.Editor.ScrollbarWidth)))
}

// rowViewMethod is the notification rowViewLua sends.
const rowViewMethod = "simplenvim_rows"

// rowViewLua makes Nvim report each split window's scroll state in screen
// rows ([win, total, top] triples) whenever it may have changed. win_viewport
// only counts buffer lines, so it can't place a bar for a view part-way
// through a wrapped line, such as one line longer than the window. Buffers
// over max_bytes are left to win_viewport: counting their rows on every
// scroll would cost too much.
const rowViewLua = `
local chan = ...
local max_bytes = 1000000
local function th(win, o) return vim.api.nvim_win_text_height(win, o).all end
local sent, queued = nil, false
local function report()
  local out = {}
  for _, win in ipairs(vim.api.nvim_tabpage_list_wins(0)) do
    local buf = vim.api.nvim_win_get_buf(win)
    if vim.api.nvim_win_get_config(win).relative == ''
        and vim.api.nvim_buf_get_offset(buf, vim.api.nvim_buf_line_count(buf)) <= max_bytes then
      local view = vim.api.nvim_win_call(win, vim.fn.winsaveview)
      local top = view.topline > 1 and th(win, { end_row = view.topline - 2 }) or 0
      if view.skipcol > 0 then
        top = top + th(win, { start_row = view.topline - 1, end_row = view.topline - 1, end_vcol = view.skipcol })
      end
      out[#out + 1] = { win, th(win, {}), top }
    end
  end
  local key = vim.inspect(out)
  if key ~= sent then
    sent = key
    vim.rpcnotify(chan, '` + rowViewMethod + `', out)
  end
end
local function schedule()
  if queued then return end
  queued = true
  vim.schedule(function()
    queued = false
    pcall(report)
  end)
end
local events = { 'WinScrolled', 'WinResized', 'WinNew', 'WinClosed', 'BufWinEnter', 'TabEnter',
  'TextChanged', 'TextChangedI', 'VimResized', 'OptionSet' }
if vim.fn.exists('##TextChangedT') == 1 then
  events[#events + 1] = 'TextChangedT'
end
vim.api.nvim_create_autocmd(events, {
  group = vim.api.nvim_create_augroup('SimpleNvimRowView', {}),
  callback = schedule,
})
schedule()
`

// installRowViews keeps the state's row views current while the scrollbar
// setting is on.
func (a *App) installRowViews() {
	if !a.cfg.Editor.Scrollbar {
		return
	}
	_ = a.proc.HandleRequest(rowViewMethod, func(rows interface{}) {
		a.state.SetRowViews(rows)
		a.invalidate()
	})
	a.proc.ExecLua(rowViewLua)
}

// onScrollbarPointer handles the left button on a scrollbar and reports
// whether it consumed e. Pressing the thumb starts a drag; pressing the
// track elsewhere jumps so the thumb is centred on the pointer, and can be
// dragged on from there. Neither moves focus to the scrolled window. Nvim
// never sees these events, so it doesn't also click the text underneath.
func (a *App) onScrollbarPointer(e pointer.Event, snap uistate.Snapshot) bool {
	pos := image.Pt(int(e.Position.X), int(e.Position.Y))
	switch e.Kind {
	case pointer.Press:
		if input.MouseButtonFor(e) != "left" {
			return false
		}
		for _, b := range a.scrollbars(snap) {
			if !pos.In(b.Track) {
				continue
			}
			d := &scrollDrag{bar: b, grab: pos.Y - b.Thumb.Min.Y}
			if !pos.In(b.Thumb) {
				d.grab = b.Thumb.Dy() / 2
				a.proc.ScrollWindow(b.Win, b.ToplineAt(pos.Y-d.grab), b.Rows)
			}
			a.scrollDrag = d
			return true
		}
	case pointer.Drag:
		if d := a.scrollDrag; d != nil {
			a.proc.ScrollWindow(d.bar.Win, d.bar.ToplineAt(pos.Y-d.grab), d.bar.Rows)
			return true
		}
	case pointer.Release, pointer.Cancel:
		if a.scrollDrag != nil {
			a.scrollDrag = nil
			return true
		}
	}
	return false
}
