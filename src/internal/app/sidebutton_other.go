//go:build !darwin

package editorapp

// Other Gio backends report back/forward as ButtonQuaternary/ButtonQuinary.
func installSideButtons(_ any) {}
