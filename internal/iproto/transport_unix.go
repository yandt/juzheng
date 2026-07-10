//go:build !windows

package iproto

import (
	"net"
	"time"
)

// SocketPath：unix domain socket 路径。放 /var/run 需 root 写入（helper 以 root 运行）。
const SocketPath = "/var/run/juzheng-helper.sock"

// Dial 连接 helper（客户端）：unix socket 拨号。
func Dial(timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", ResolveSocketPath(), timeout)
}
