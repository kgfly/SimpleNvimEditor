//go:build windows

package editorapp

import (
	"syscall"
	"unsafe"
)

var procCreateMutexW = syscall.NewLazyDLL("kernel32.dll").NewProc("CreateMutexW")

const errorAlreadyExists syscall.Errno = 183

// claimSlot creates a named mutex in this login session. The handle is never
// closed, so Windows frees the name when the process ends.
func claimSlot(slot string) bool {
	name, err := syscall.UTF16PtrFromString(`Local\SimpleNvimEditor.slot.` + slot)
	if err != nil {
		return false
	}
	h, _, err := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return false
	}
	if err == errorAlreadyExists {
		syscall.CloseHandle(syscall.Handle(h))
		return false
	}
	return true
}
