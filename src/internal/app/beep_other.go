//go:build !darwin

package editorapp

// Only AppKit beeps on unhandled key bindings.
func installBeepSuppressor(_ any) {}
