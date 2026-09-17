//go:build !darwin && !windows

package editorapp

// Linux has no drop target to install: Gio (v0.10.2) raises transfer
// events for clipboard reads only and never decodes an X11/Wayland file
// drop, and neither protocol can be handled behind Gio's back because the
// drop hand-shake is delivered to whichever connection owns the window.
// Dropping a file on the Linux build therefore does nothing until Gio
// grows drag-and-drop support; onDrop is already wired to text/uri-list
// and will start working the moment it does.
func installDropTarget(_ any) {}
