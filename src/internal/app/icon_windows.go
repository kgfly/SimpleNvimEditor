//go:build windows

package editorapp

import (
	"sync"
	"syscall"
	"unsafe"

	gioapp "gioui.org/app"
)

var (
	user32  = syscall.NewLazyDLL("user32.dll")
	shell32 = syscall.NewLazyDLL("shell32.dll")

	procCreateIconFromResourceEx = user32.NewProc("CreateIconFromResourceEx")
	procSendNotifyMessageW       = user32.NewProc("SendNotifyMessageW")
	procSetClassLongPtrW         = user32.NewProc("SetClassLongPtrW")
	procSetProcessAppUserModelID = shell32.NewProc("SetCurrentProcessExplicitAppUserModelID")
)

const (
	wmSetIcon      = 0x0080
	iconSmall      = 0
	iconBig        = 1
	iconResVer     = 0x00030000
	lrDefaultColor = 0

	gclpHIcon   = -14
	gclpHIconSm = -34
)

// setWindowIcon sets the title-bar and taskbar icon of the window. It runs
// on Gio's client goroutine, so everything it does has to be asynchronous
// with respect to the window thread; anything that isn't belongs in
// installClassIcon instead.
func setWindowIcon(view any, color string) {
	ev, ok := view.(gioapp.Win32ViewEvent)
	if !ok || ev.HWND == 0 {
		return
	}
	h := iconHandle(color)
	if h == 0 {
		return
	}
	// SendNotifyMessage does not wait on the window thread, which may be
	// blocked waiting for this goroutine to finish a frame.
	procSendNotifyMessageW.Call(ev.HWND, wmSetIcon, iconBig, h)
	procSendNotifyMessageW.Call(ev.HWND, wmSetIcon, iconSmall, h)
}

// installClassIcon swaps the icons of the window class, which the taskbar
// falls back to when WM_GETICON times out on a window thread busy with a
// frame; the class belongs to this process alone.
//
// It must run on the window thread -- runWindow hands it to Window.Run --
// because SetClassLongPtr updates the frame synchronously and waits for the
// window thread to do it. Called from the client goroutine it deadlocks:
// the window thread is parked handing that very ViewEvent to the client,
// which is now blocked inside SetClassLongPtr, so neither can make
// progress. The window never paints and Windows reports it as
// "Not Responding".
func installClassIcon(view any, color string) {
	ev, ok := view.(gioapp.Win32ViewEvent)
	if !ok || ev.HWND == 0 {
		return
	}
	h := iconHandle(color)
	if h == 0 {
		return
	}
	procSetClassLongPtrW.Call(ev.HWND, gclpOffset(gclpHIcon), h)
	procSetClassLongPtrW.Call(ev.HWND, gclpOffset(gclpHIconSm), h)
}

// gclpOffset converts a GCLP_* class offset to the uintptr
// SetClassLongPtr wants. Going through a variable is not decoration: a
// negative constant cannot be converted to uintptr.
func gclpOffset(index int) uintptr {
	return uintptr(index)
}

// iconHandle returns the HICON for color, creating it at most once. Windows
// Vista+ accepts PNG data as an icon resource. The handle is never
// destroyed: the window and its class keep using it for the life of the
// process.
var (
	iconHandleMu sync.Mutex
	iconHandleOf string
	iconHandleH  uintptr
)

func iconHandle(color string) uintptr {
	iconHandleMu.Lock()
	defer iconHandleMu.Unlock()
	if iconHandleH != 0 && iconHandleOf == color {
		return iconHandleH
	}
	png := iconPNG(color)
	h, _, _ := procCreateIconFromResourceEx.Call(
		uintptr(unsafe.Pointer(&png[0])), uintptr(len(png)), 1, iconResVer, 0, 0, lrDefaultColor)
	if h == 0 {
		return 0
	}
	iconHandleOf, iconHandleH = color, h
	return h
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
