//go:build windows

package iproto

import (
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

// SocketPath：Windows 命名管道名。安装用户与 SYSTEM 可连接（ACL 在 helper 监听侧用 SDDL 收紧）。
const SocketPath = `\\.\pipe\juzheng-helper`

// Dial 连接 helper（客户端）：命名管道拨号，timeout 控制等待管道就绪的时长。
func Dial(timeout time.Duration) (net.Conn, error) {
	return winio.DialPipe(ResolveSocketPath(), &timeout)
}
