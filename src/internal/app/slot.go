package editorapp

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
)

// maxSlot is the highest instance number shown on the icon; later
// instances of a color group use the group's plain icon.
const maxSlot = 99

// heldSlots maps each slot this process owns to the function releasing it.
var (
	heldSlotsMu sync.Mutex
	heldSlots   = map[string]func(){}
)

func slotKey(color string, n int) string { return color + "." + strconv.Itoa(n) }

// slotIcon returns the icon for slot n of color: color for slot 1 or no
// slot, color_<n> otherwise.
func slotIcon(color string, n int) string {
	if n < 2 || n > maxSlot {
		return color
	}
	return color + "_" + strconv.Itoa(n)
}

// takeSlot claims slot n of color for this process, reporting whether it
// was free. It must not be called for a slot this process already holds.
func takeSlot(color string, n int) bool {
	key := slotKey(color, n)
	release, ok := claimSlot(key)
	if !ok {
		return false
	}
	heldSlotsMu.Lock()
	heldSlots[key] = release
	heldSlotsMu.Unlock()
	return true
}

// releaseSlot frees slot n of color if this process holds it.
func releaseSlot(color string, n int) {
	key := slotKey(color, n)
	heldSlotsMu.Lock()
	release := heldSlots[key]
	delete(heldSlots, key)
	heldSlotsMu.Unlock()
	if release != nil {
		release()
	}
}

// claimIcon numbers this instance within its color group by claiming the
// lowest free slot, and returns the icon to show and the slot number, 0
// when no slot could be claimed. The claim lasts until it is released or
// the process ends, however it ends.
func claimIcon(color string) (icon string, slot int) {
	for n := 1; n <= maxSlot; n++ {
		if takeSlot(color, n) {
			return slotIcon(color, n), n
		}
	}
	return color, 0
}

// claimRequestedSlot takes slot n of color for an editor started with
// --slot n; numbered says a SIMPLENVIM_BG_<COLOR> variable chose color.
func claimRequestedSlot(color string, numbered bool, n int) error {
	switch {
	case !slotsSupported:
		return errors.New("--slot: icon numbers aren't supported on this OS")
	case !numbered:
		return errors.New("--slot: this editor isn't numbered; start it with SIMPLENVIM_BG_<COLOR> set")
	case n < 1 || n > maxSlot:
		return fmt.Errorf("--slot: the number must be 1 to %d", maxSlot)
	case !takeSlot(color, n):
		return fmt.Errorf("--slot: %s %d is in use", color, n)
	}
	return nil
}

// CheckSlot reports why an editor started now with this environment and
// --slot n would fail, or nil if it would get slot n.
func CheckSlot(n int) error {
	color, numbered := startupIcon(os.Environ())
	if err := claimRequestedSlot(color, numbered, n); err != nil {
		return err
	}
	releaseSlot(color, n)
	return nil
}
