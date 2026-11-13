package spare

import (
	"fmt"
	"golang.org/x/sys/unix"
	"net"
)

func CheckPeer(c *net.UnixConn, uid uint32) error {
	raw, err := c.SyscallConn()
	if err != nil {
		return err
	}
	var peerErr error
	err = raw.Control(func(fd uintptr) {
		cred, e := unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
		if e != nil {
			peerErr = e
		} else if cred.Uid != uid {
			peerErr = fmt.Errorf("unexpected spare peer uid %d", cred.Uid)
		}
	})
	if err != nil {
		return err
	}
	return peerErr
}
