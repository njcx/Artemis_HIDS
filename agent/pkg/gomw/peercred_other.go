//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd

package gomw

import "net"

// peerPid 在不支持的平台上无法获取对端 PID，退化为跳过校验
func peerPid(conn net.Conn) (int, bool) {
	return 0, false
}
