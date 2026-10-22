//go:build windows

package editorapp

import (
	"syscall"
	"unsafe"
)

var procCreateMutexW = syscall.NewLazyDLL("kernel32.dll").NewProc("CreateMutexW")

const errorAlreadyExists syscall.Errno = 183

const slotsSupported = true

// claimSlot creates a named mutex in this login session. Windows frees the
// name when the handle is closed or the process ends.
func claimSlot(slot string) (release func(), ok bool) {
	name, err := syscall.UTF16PtrFromString(`Local\SimpleNvimEditor.slot.` + slot)
	if err != nil {
		return nil, false
	}
	h, _, err := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return nil, false
	}
	if err == errorAlreadyExists {
		syscall.CloseHandle(syscall.Handle(h))
		return nil, false
	}
	return func() { _ = syscall.CloseHandle(syscall.Handle(h)) }, true
}
