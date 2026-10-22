//go:build darwin

package editorapp

/*
#cgo LDFLAGS: -framework CoreFoundation

#include <stdlib.h>

// Declarations only; defined in slot_darwin.c.
void *snv_claim_slot(const char *name);
void snv_release_slot(void *port);
*/
import "C"

import "unsafe"

const slotsSupported = true

// claimSlot registers a named message port in this login session, which
// launchd frees when the port is released or the process ends.
//
// CoreFoundation hands back the existing port for a name this process
// already holds, so callers must never claim a slot they own.
func claimSlot(slot string) (release func(), ok bool) {
	name := C.CString("io.github.kgfly.simplenvimeditor.slot." + slot)
	defer C.free(unsafe.Pointer(name))
	port := C.snv_claim_slot(name)
	if port == nil {
		return nil, false
	}
	return func() { C.snv_release_slot(port) }, true
}
