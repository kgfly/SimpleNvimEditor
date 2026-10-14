//go:build !darwin

package editorapp

// Only macOS has an application menu; elsewhere `simplenvim --version`
// reports the version.
func installAppMenu(_ string) {}
