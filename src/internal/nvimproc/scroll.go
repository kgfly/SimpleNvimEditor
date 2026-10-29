package nvimproc

// scrollLua scrolls window `win` so its 0-based topline is `top` (any value
// past the end means "show the last line at the bottom"), without changing
// which window has focus. The cursor stays put if it is still on screen and
// is otherwise pulled just inside the new view, because Nvim would scroll
// straight back to a cursor left off screen.
//
// A terminal in Terminal mode is pinned to its output, so it is taken to
// Normal mode first. That switch only completes after this call returns, so
// the scroll waits for the ModeChanged event; a scroll issued before then
// is lost.
const scrollLua = `
local win, top = ...
if not vim.api.nvim_win_is_valid(win) then return end
local function apply()
  if not vim.api.nvim_win_is_valid(win) then return end
  vim.api.nvim_win_call(win, function()
    local count = vim.api.nvim_buf_line_count(0)
    local height = vim.api.nvim_win_get_height(0)
    local first = math.max(1, math.min(top + 1, count))
    if first + height > count then
      vim.cmd('keepjumps normal! Gzb')
      return
    end
    local cur = vim.api.nvim_win_get_cursor(0)
    local so = math.min(vim.wo.scrolloff, math.floor((height - 1) / 2))
    local lo = first == 1 and 1 or first + so
    local hi = first + height - 1 - so
    vim.fn.winrestview({ topline = first, lnum = math.max(lo, math.min(cur[1], hi)), col = cur[2] })
  end)
end
if win == vim.api.nvim_get_current_win() and vim.api.nvim_get_mode().mode == 't' then
  vim.api.nvim_create_autocmd('ModeChanged', {
    pattern = 't:*',
    once = true,
    callback = function() vim.schedule(apply) end,
  })
  vim.cmd.stopinsert()
else
  apply()
end
`

// ScrollWindow scrolls Nvim window win (a handle) so that its 0-based
// topline is topline; a value past the end scrolls to the bottom.
//
// A scrollbar drag produces a new position every frame, and only the
// latest matters, so a request for a window that already has one waiting
// replaces it rather than queueing behind it (as Resize does). It runs on
// slow because, like any Lua call, Nvim defers it at a prompt.
func (p *Process) ScrollWindow(win, topline int) {
	p.scrollMu.Lock()
	defer p.scrollMu.Unlock()

	if p.pendingScroll == nil {
		p.pendingScroll = make(map[int]int)
	}
	_, queued := p.pendingScroll[win]
	p.pendingScroll[win] = topline
	if queued {
		return
	}
	p.slow.push(func() {
		p.scrollMu.Lock()
		top := p.pendingScroll[win]
		delete(p.pendingScroll, win)
		p.scrollMu.Unlock()
		p.applyScroll(win, top)
	})
}

// applyScroll performs the actual scroll round trip. It is a field-backed
// indirection purely so tests can observe coalescing without a live Nvim.
func (p *Process) applyScroll(win, topline int) {
	if p.scrollFn != nil {
		p.scrollFn(win, topline)
		return
	}
	_ = p.Nvim.ExecLua(scrollLua, nil, win, topline)
}
