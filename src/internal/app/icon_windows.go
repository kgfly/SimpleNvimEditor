//go:build windows

package editorapp

import (
	"syscall"
	"unsafe"

	gioapp "gioui.org/app"
)

var (
	user32  = syscall.NewLazyDLL("user32.dll")
	shell32 = syscall.NewLazyDLL("shell32.dll")

	procCreateIconFromResourceEx = user32.NewProc("CreateIconFromResourceEx")
	procSendNotifyMessageW       = user32.NewProc("SendNotifyMessageW")
	procSetProcessAppUserModelID = shell32.NewProc("SetCurrentProcessExplicitAppUserModelID")
)

const (
	wmSetIcon      = 0x0080
	iconSmall      = 0
	iconBig        = 1
	iconResVer     = 0x00030000
	lrDefaultColor = 0
)

// setWindowIcon sets the title-bar and taskbar icon of the window.
func setWindowIcon(view any, color string) {
	ev, ok := view.(gioapp.Win32ViewEvent)
	if !ok || ev.HWND == 0 {
		return
	}
	png := iconPNG(color)
	// Windows Vista+ accepts PNG data as an icon resource. The handle is
	// never destroyed: the window keeps using it.
	h, _, _ := procCreateIconFromResourceEx.Call(
		uintptr(unsafe.Pointer(&png[0])), uintptr(len(png)), 1, iconResVer, 0, 0, lrDefaultColor)
	if h == 0 {
		return
	}
	// SendNotifyMessage does not wait on the window thread, which may be
	// blocked waiting for this goroutine to finish a frame.
	procSendNotifyMessageW.Call(ev.HWND, wmSetIcon, iconBig, h)
	procSendNotifyMessageW.Call(ev.HWND, wmSetIcon, iconSmall, h)
}

// setProcessAppID puts this process in its own taskbar group, which also
// stops a pinned shortcut's icon from replacing the window's.
func setProcessAppID(color string) {
	id, err := syscall.UTF16PtrFromString("SimpleNvimEditor." + color)
	if err != nil {
		return
	}
	procSetProcessAppUserModelID.Call(uintptr(unsafe.Pointer(id)))
}
