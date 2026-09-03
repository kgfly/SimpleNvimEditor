//go:build !darwin

package editorapp

// InstallOpenFileHandler is a no-op away from macOS.
//
// Linux and Windows pass the filename as an ordinary command-line
// argument (via the .desktop Exec line and the shell "Edit with" verb
// respectively), so it arrives through cli.Parse like any other argv entry
// and needs no out-of-band delivery.
func InstallOpenFileHandler() {}
