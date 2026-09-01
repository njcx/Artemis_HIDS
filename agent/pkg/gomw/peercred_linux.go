//go:build linux

package gomw

import (
	"net"
	"syscall"
)

// peerPid 通过 SO_PEERCRED 获取 Unix socket 对端的真实 PID
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
	var ucredErr error
	err = raw.Control(func(fd uintptr) {
		cred, err := syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
		if err != nil {
			ucredErr = err
			return
		}
		pid = int(cred.Pid)
	})
	if err != nil || ucredErr != nil {
		return 0, false
	}

	return pid, true
}
