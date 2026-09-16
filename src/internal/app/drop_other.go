//go:build !darwin && !windows

package editorapp

// Linux file drops arrive through Gio's transfer events. Other non-desktop
// targets have no native drop target to install.
func installDropTarget(_ any) {}
