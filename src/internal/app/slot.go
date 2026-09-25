package editorapp

import "strconv"

// maxSlot is the highest instance number shown on the icon; later
// instances of a color group use the group's plain icon.
const maxSlot = 9

// claimIcon numbers this instance within its color group by claiming the
// lowest free slot, and returns the icon to show: color for slot 1 (or when
// all slots are taken), color_<n> otherwise. The claim lasts until the
// process ends, however it ends.
func claimIcon(color string) string {
	for n := 1; n <= maxSlot; n++ {
		if claimSlot(color + "." + strconv.Itoa(n)) {
			if n == 1 {
				return color
			}
			return color + "_" + strconv.Itoa(n)
		}
	}
	return color
}
