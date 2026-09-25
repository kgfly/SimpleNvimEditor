//go:build !windows && !linux && !darwin

package editorapp

// claimSlot has no process-lifetime named object here, so no numbering.
func claimSlot(_ string) bool { return false }
