//go:build linux

package editorapp

import (
	"net"
	"os"
	"strconv"
)

// heldSlots keeps the claimed sockets open for the life of the process.
var heldSlots []*net.UnixConn

// claimSlot binds an abstract Unix socket, which has no file and which the
// kernel frees when the process ends. The namespace is shared by all users.
func claimSlot(slot string) bool {
	name := "@simplenvim." + strconv.Itoa(os.Getuid()) + ".slot." + slot
	c, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: name, Net: "unixgram"})
	if err != nil {
		return false
	}
	heldSlots = append(heldSlots, c)
	return true
}
