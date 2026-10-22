package editorapp

import (
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
