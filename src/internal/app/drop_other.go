//go:build !darwin && !windows && !linux

package editorapp

// Platforms without a native drop target of their own.
func installDropTarget(_ any) {}
