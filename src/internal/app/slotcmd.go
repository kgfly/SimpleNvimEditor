package editorapp

import (
	"strconv"
	"strings"
	"sync"
)

// slotCmd is the Nvim command that renumbers this editor's icon.
const slotCmd = "Simplenvimslot"

// RPC methods behind :Simplenvimslot; see slotCmdLua.
const (
	slotCmdMethod     = "simplenvim_slot"
	slotCmdFreeMethod = "simplenvim_slot_free"
)

// slotCmdLua defines :Simplenvimslot in Nvim: `:Simplenvimslot 4` takes
// slot 4 of its color group, `:Simplenvimslot` shows the current one, and
// Tab completes the free ones. It runs with this client's channel ID as its
// argument, and leaves alone a command of that name the user already defined.
const slotCmdLua = `
local chan = ...
local function define()
  if vim.fn.exists(':` + slotCmd + `') == 2 then
    return
  end
  vim.api.nvim_create_user_command('` + slotCmd + `', function(o)
    local ok, r = pcall(vim.rpcrequest, chan, '` + slotCmdMethod + `', o.args)
    if not ok then
      r = { err = tostring(r) }
    end
    if r.err then
      vim.notify(r.err, vim.log.levels.ERROR)
    else
      vim.notify(r.msg)
    end
  end, {
    nargs = '?',
    desc = 'Renumber this SimpleNvimEditor icon',
    complete = function()
      local ok, r = pcall(vim.rpcrequest, chan, '` + slotCmdFreeMethod + `')
      return ok and r or {}
    end,
  })
end
if vim.v.vim_did_enter == 1 then
  define()
else
  vim.api.nvim_create_autocmd('VimEnter', { once = true, callback = define })
end
`

// slotState is this editor's place in its color group. The :Simplenvimslot
// handlers change it from RPC goroutines and layout applies the change to
// the window.
type slotState struct {
	mu sync.Mutex
	// color is the group, or "" when this editor isn't numbered.
	color string
	// n is the slot held, or 0 for none (the 100th and later editors).
	n int
	// changed says n has changed since layout last applied it.
	changed bool
}

func (s *slotState) init(color string, n int) {
	s.mu.Lock()
	s.color, s.n = color, n
	s.mu.Unlock()
}

// renumber runs `:Simplenvimslot arg`: an empty arg reports the current
// slot, a number from 1 to maxSlot moves to that slot if it is free. It
// returns the message for the user and whether it is an error.
func (s *slotState) renumber(arg string) (msg string, failed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !slotsSupported {
		return slotCmd + ": icon numbers aren't supported on this OS", true
	}
	if s.color == "" {
		return slotCmd + ": this editor isn't numbered; start it with SIMPLENVIM_BG_<COLOR> set", true
	}
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return slotCmd + ": " + s.describe(s.n) + " (free: " + compactRanges(s.freeLocked()) + ")", false
	}
	n, err := strconv.Atoi(arg)
	if err != nil || n < 1 || n > maxSlot {
		return slotCmd + ": the number must be 1 to " + strconv.Itoa(maxSlot), true
	}
	if n == s.n {
		return slotCmd + ": already " + s.describe(n), false
	}
	if !takeSlot(s.color, n) {
		return slotCmd + ": " + s.describe(n) + " is in use", true
	}
	releaseSlot(s.color, s.n)
	s.n = n
	s.changed = true
	return slotCmd + ": " + s.describe(n), false
}

func (s *slotState) describe(n int) string {
	if n == 0 {
		return s.color + " without a number"
	}
	return s.color + " " + strconv.Itoa(n)
}

// free lists the slots of this editor's group that no editor holds.
func (s *slotState) free() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	free := []string{}
	if !slotsSupported || s.color == "" {
		return free
	}
	for _, n := range s.freeLocked() {
		free = append(free, strconv.Itoa(n))
	}
	return free
}

// freeLocked probes each slot by claiming and releasing it. The editor's
// own slot is skipped: claiming it again would not fail (see claimSlot).
func (s *slotState) freeLocked() []int {
	var free []int
	for n := 1; n <= maxSlot; n++ {
		if n != s.n && takeSlot(s.color, n) {
			releaseSlot(s.color, n)
			free = append(free, n)
		}
	}
	return free
}

// compactRanges joins ascending numbers with runs of three or more shortened,
// e.g. "1 2 4-99", so a long free list still fits on the message line.
func compactRanges(ns []int) string {
	var parts []string
	for i := 0; i < len(ns); {
		j := i
		for j+1 < len(ns) && ns[j+1] == ns[j]+1 {
			j++
		}
		switch {
		case j-i >= 2:
			parts = append(parts, strconv.Itoa(ns[i])+"-"+strconv.Itoa(ns[j]))
		case j > i:
			parts = append(parts, strconv.Itoa(ns[i]), strconv.Itoa(ns[j]))
		default:
			parts = append(parts, strconv.Itoa(ns[i]))
		}
		i = j + 1
	}
	return strings.Join(parts, " ")
}

// takeChange returns the group and the new slot if :Simplenvimslot moved
// this editor since the last call.
func (s *slotState) takeChange() (color string, n int, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.changed {
		return "", 0, false
	}
	s.changed = false
	return s.color, s.n, true
}

// installSlotCmd serves the :Simplenvimslot requests and defines the
// command in Nvim.
func (a *App) installSlotCmd() {
	_ = a.proc.HandleRequest(slotCmdMethod, func(arg string) (map[string]string, error) {
		msg, failed := a.slot.renumber(arg)
		if failed {
			return map[string]string{"err": msg}, nil
		}
		a.invalidate()
		return map[string]string{"msg": msg}, nil
	})
	_ = a.proc.HandleRequest(slotCmdFreeMethod, func() ([]string, error) {
		return a.slot.free(), nil
	})
	a.proc.ExecLua(slotCmdLua)
}

// applySlot shows a slot taken with :Simplenvimslot in the icon and title.
func (a *App) applySlot() {
	color, n, ok := a.slot.takeChange()
	if !ok {
		return
	}
	a.iconColor = slotIcon(color, n)
	if a.cfg.Editor.TitlePrefix {
		a.titlePrefix = titlePrefix(color, n)
	}
	setWindowIcon(a.view, a.iconColor)
	retagWindow(a.win, a.view, a.iconColor)
}
