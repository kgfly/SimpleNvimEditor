//go:build darwin

package editorapp

/*
#cgo LDFLAGS: -framework CoreFoundation

#include <stdlib.h>

// Declaration only; defined in slot_darwin.c.
int snv_claim_slot(const char *name);
*/
import "C"

import "unsafe"

// claimSlot registers a named message port in this login session, which
// launchd frees when the process ends.
func claimSlot(slot string) bool {
	name := C.CString("io.github.kgfly.simplenvimeditor.slot." + slot)
	defer C.free(unsafe.Pointer(name))
	return C.snv_claim_slot(name) != 0
}
