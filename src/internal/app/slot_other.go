//go:build !windows && !linux && !darwin

package editorapp

const slotsSupported = false

// claimSlot has no process-lifetime named object here, so no numbering.
func claimSlot(_ string) (func(), bool) { return nil, false }
