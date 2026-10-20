package editorapp

import "strconv"

// maxSlot is the highest instance number shown on the icon; later
// instances of a color group use the group's plain icon.
const maxSlot = 9

// claimIcon numbers this instance within its color group by claiming the
// lowest free slot, and returns the icon to show and the slot number: color
// for slot 1, color_<n> otherwise, and color with slot 0 when no slot could
// be claimed. The claim lasts until the process ends, however it ends.
func claimIcon(color string) (icon string, slot int) {
	for n := 1; n <= maxSlot; n++ {
		if claimSlot(color + "." + strconv.Itoa(n)) {
			if n == 1 {
				return color, n
			}
			return color + "_" + strconv.Itoa(n), n
		}
	}
	return color, 0
}
