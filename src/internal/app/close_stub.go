//go:build !darwin && (!windows || !cgo)

package editorapp

// Platforms with no way to cancel a close request.
//
// X11 delivers WM_DELETE_WINDOW, and Wayland xdg_toplevel.close, to the
// connection that created the window -- Gio's -- and Gio turns both into an
// immediate shutdown. Neither can be intercepted from outside the toolkit
// (an X11 ClientMessage sent with an empty event mask reaches only the
// window's creator, which is why the XDND code in drop_x11.c needs a proxy
// window and why no such escape hatch exists here).
//
// Unsaved work is still protected: when Nvim does not exit within the grace
// period in App.quit it is showing a "save changes?" prompt, and Run puts
// the session back on screen in a fresh window so the prompt can be
// answered.
func installCloseHandler(_ any) {}

func allowClose() {}
