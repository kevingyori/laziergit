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
		cred, e := unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		if e != nil {
			peerErr = e
		} else if cred.Uid != uid {
			peerErr = fmt.Errorf("spare peer uid %d, expected %d", cred.Uid, uid)
		}
	})
	if err != nil {
		return err
	}
	return peerErr
}
