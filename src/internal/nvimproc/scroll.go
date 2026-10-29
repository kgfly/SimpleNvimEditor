package nvimproc

// scrollLua scrolls window `win` so its 0-based topline is `top` (any value
// past the end means "show the last line at the bottom"), without changing
// which window has focus. With `rows` set, `top` counts screen rows instead,
// so the view can start part-way through a wrapped line. The cursor stays
// put if it is still on screen and is otherwise pulled just inside the new
// view, because Nvim would scroll straight back to a cursor left off screen.
//
// A terminal in Terminal mode is pinned to its output, so it is taken to
// Normal mode first. That switch only completes after this call returns, so
// the scroll waits for the ModeChanged event; a scroll issued before then
// is lost.
const scrollLua = `
local win, top, rows = ...
if not vim.api.nvim_win_is_valid(win) then return end
local function th(o) return vim.api.nvim_win_text_height(0, o).all end
-- above is the screen rows taken by the lines before line l.
local function above(l) return l > 1 and th({ end_row = l - 2 }) or 0 end
-- at returns the line and the start virtual column of screen row r.
local function at(r, count)
  local lo, hi = 1, count
  while lo < hi do
    local mid = math.floor((lo + hi + 1) / 2)
    if above(mid) <= r then lo = mid else hi = mid - 1 end
  end
  local k = r - above(lo)
  local v0, v1 = 0, vim.fn.virtcol({ lo, '$' })
  while v0 < v1 do
    local mid = math.floor((v0 + v1 + 1) / 2)
    if th({ start_row = lo - 1, end_row = lo - 1, end_vcol = mid }) <= k then v0 = mid else v1 = mid - 1 end
  end
  return lo, v0
end
local function apply_rows()
  local count = vim.api.nvim_buf_line_count(0)
  local height = vim.api.nvim_win_get_height(0)
  local total = th({})
  local r = math.max(0, math.min(top, total - height))
  local l, v = at(r, count)
  local so = math.min(vim.wo.scrolloff, math.floor((height - 1) / 2))
  local lo = r == 0 and 0 or r + so
  local hi = r + height - 1 - so
  if r >= total - height then hi = total - 1 end
  local cur = vim.api.nvim_win_get_cursor(0)
  local crow = above(cur[1]) + th({ start_row = cur[1] - 1, end_row = cur[1] - 1, end_vcol = vim.fn.virtcol('.') }) - 1
  local cl, cc = cur[1], cur[2]
  if crow < lo or crow > hi then
    local cv
    cl, cv = at(math.max(lo, math.min(crow, hi)), count)
    cc = math.max(0, vim.fn.virtcol2col(0, cl, cv + 1) - 1)
  end
  -- Nvim scrolls a cursor sitting exactly on skipcol back to the bottom
  -- row, so step it onto the next character.
  if cl == l and v > 0 and cc == vim.fn.virtcol2col(0, l, v + 1) - 1 then
    local line = vim.api.nvim_buf_get_lines(0, l - 1, l, true)[1]
    cc = math.min(#line - 1, cc + vim.str_utf_end(line, cc + 1) + 1)
  end
  vim.fn.winrestview({ topline = l, skipcol = v, lnum = cl, col = cc })
end
local function apply()
  if not vim.api.nvim_win_is_valid(win) then return end
  vim.api.nvim_win_call(win, function()
    if rows then
      apply_rows()
      return
    end
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
// topline is topline, or with rows set, so that topline screen rows are
// above it; a value past the end scrolls to the bottom.
//
// A scrollbar drag produces a new position every frame, and only the
// latest matters, so a request for a window that already has one waiting
// replaces it rather than queueing behind it (as Resize does). It runs on
// slow because, like any Lua call, Nvim defers it at a prompt.
func (p *Process) ScrollWindow(win, topline int, rows bool) {
	p.scrollMu.Lock()
	defer p.scrollMu.Unlock()

	if p.pendingScroll == nil {
		p.pendingScroll = make(map[int]scrollTarget)
	}
	_, queued := p.pendingScroll[win]
	p.pendingScroll[win] = scrollTarget{topline, rows}
	if queued {
		return
	}
	p.slow.push(func() {
		p.scrollMu.Lock()
		t := p.pendingScroll[win]
		delete(p.pendingScroll, win)
		p.scrollMu.Unlock()
		p.applyScroll(win, t.top, t.rows)
	})
}

type scrollTarget struct {
	top  int
	rows bool
}

// applyScroll performs the actual scroll round trip. It is a field-backed
// indirection purely so tests can observe coalescing without a live Nvim.
func (p *Process) applyScroll(win, topline int, rows bool) {
	if p.scrollFn != nil {
		p.scrollFn(win, topline, rows)
		return
	}
	_ = p.Nvim.ExecLua(scrollLua, nil, win, topline, rows)
}
