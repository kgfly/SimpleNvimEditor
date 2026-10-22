//go:build linux

package editorapp

import (
	"net"
	"os"
	"strconv"
)

const slotsSupported = true

// claimSlot binds an abstract Unix socket, which has no file and which the
// kernel frees when the socket is closed or the process ends. The namespace
// is shared by all users.
func claimSlot(slot string) (release func(), ok bool) {
	name := "@simplenvim." + strconv.Itoa(os.Getuid()) + ".slot." + slot
	c, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: name, Net: "unixgram"})
	if err != nil {
		return nil, false
	}
	return func() { _ = c.Close() }, true
}
