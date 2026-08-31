//go:build darwin || freebsd || netbsd || openbsd

package gomw

import (
	"net"
	"syscall"
)

// sys/un.h: SOL_LOCAL=0, LOCAL_PEERPID=2 (not exposed by Go's syscall package on BSD/Darwin)
const (
	solLocal     = 0
	localPeerPid = 2
)

// peerPid 通过 LOCAL_PEERPID 获取 Unix socket 对端的真实 PID
func peerPid(conn net.Conn) (int, bool) {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, false
	}

	raw, err := uc.SyscallConn()
	if err != nil {
		return 0, false
	}

	var pid int
	var sockErr error
	err = raw.Control(func(fd uintptr) {
		v, err := syscall.GetsockoptInt(int(fd), solLocal, localPeerPid)
		if err != nil {
			sockErr = err
			return
		}
		pid = v
	})
	if err != nil || sockErr != nil {
		return 0, false
	}

	return pid, true
}
