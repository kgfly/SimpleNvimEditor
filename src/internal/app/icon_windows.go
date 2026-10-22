//go:build windows

package editorapp

import (
	"runtime"
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
	procSHGetPropertyStoreForWin = shell32.NewProc("SHGetPropertyStoreForWindow")

	ole32              = syscall.NewLazyDLL("ole32.dll")
	procCoInitializeEx = ole32.NewProc("CoInitializeEx")
	procCoUninitialize = ole32.NewProc("CoUninitialize")
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

// appUserModelID is the taskbar group of icon.
func appUserModelID(icon string) string { return "SimpleNvimEditor." + icon }

// setProcessAppID puts this process in its own taskbar group, which also
// stops a pinned shortcut's icon from replacing the window's.
func setProcessAppID(color string) {
	id, err := syscall.UTF16PtrFromString(appUserModelID(color))
	if err != nil {
		return
	}
	procSetProcessAppUserModelID.Call(uintptr(unsafe.Pointer(id)))
}

type guid struct {
	data1        uint32
	data2, data3 uint16
	data4        [8]byte
}

type propertyKey struct {
	fmtid guid
	pid   uint32
}

// propVariant is a PROPVARIANT holding at most a pointer.
type propVariant struct {
	vt       uint16
	reserved [3]uint16
	val      uintptr
	_        uintptr
}

const vtLPWSTR = 31

var (
	iidIPropertyStore  = guid{0x886D8EEB, 0x8CF2, 0x4446, [8]byte{0x8D, 0x02, 0xCD, 0xBA, 0x1D, 0xBD, 0xCF, 0x99}}
	pkeyAppUserModelID = propertyKey{guid{0x9F4C2855, 0x9F79, 0x4B39, [8]byte{0xA8, 0xD0, 0xE1, 0xD4, 0x2D, 0xE1, 0xD5, 0xF3}}, 5}
)

// comObject is a COM interface pointer: its first word is the vtable.
type comObject struct{ vtbl *[8]uintptr }

// IPropertyStore vtable slots.
const (
	propStoreRelease  = 2
	propStoreSetValue = 6
	propStoreCommit   = 7
)

// setWindowAppID moves the window to icon's taskbar group. The process
// group set at startup no longer fits once :Simplenvimslot renumbers the
// window, and a later editor taking the old number would otherwise share its
// group. The default icon has no group of its own, so it clears the window's.
// It must run on the window thread.
func setWindowAppID(view any, icon string) {
	ev, ok := view.(gioapp.Win32ViewEvent)
	if !ok || ev.HWND == 0 {
		return
	}
	// S_OK and S_FALSE need a matching uninitialize; RPC_E_CHANGED_MODE
	// means COM is already set up on this thread, which is just as good.
	if hr, _, _ := procCoInitializeEx.Call(0, 2 /* COINIT_APARTMENTTHREADED */); hr == 0 || hr == 1 {
		defer procCoUninitialize.Call()
	}
	var store *comObject
	hr, _, _ := procSHGetPropertyStoreForWin.Call(ev.HWND,
		uintptr(unsafe.Pointer(&iidIPropertyStore)), uintptr(unsafe.Pointer(&store)))
	if hr != 0 || store == nil {
		return
	}
	vtbl, this := store.vtbl, uintptr(unsafe.Pointer(store))
	defer syscall.SyscallN(vtbl[propStoreRelease], this)

	var v propVariant
	var id *uint16
	if icon != defaultIconColor {
		var err error
		if id, err = syscall.UTF16PtrFromString(appUserModelID(icon)); err != nil {
			return
		}
		v.vt, v.val = vtLPWSTR, uintptr(unsafe.Pointer(id))
	}
	syscall.SyscallN(vtbl[propStoreSetValue], this,
		uintptr(unsafe.Pointer(&pkeyAppUserModelID)), uintptr(unsafe.Pointer(&v)))
	syscall.SyscallN(vtbl[propStoreCommit], this)
	runtime.KeepAlive(id)
}
