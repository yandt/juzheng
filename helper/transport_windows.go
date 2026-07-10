//go:build singbox && windows

// transport_windows.go：helper 在 Windows 的监听端点 —— 命名管道 + SDDL 访问控制。
//
// 与 macOS 的 unix socket + LOCAL_PEERCRED 对应：Windows 下调用方的准入由命名管道
// 的 ACL（SDDL）在连接建立时强制，因此无需逐连接再查对端 UID。

package main

import (
	"net"
	"os"

	winio "github.com/Microsoft/go-winio"
	"github.com/zhanghui/juzheng/internal/iproto"
)

// listen 在命名管道上监听。SDDL 收紧可连接方，allowedUID 在 Windows 不适用（准入靠 ACL）。
func listen(pipePath string, _ int) (net.Listener, error) {
	cfg := &winio.PipeConfig{
		SecurityDescriptor: pipeSDDL(),
	}
	return winio.ListenPipe(pipePath, cfg)
}

// pipeSDDL 构造命名管道的安全描述符：
//   - SYSTEM(SY) 与 Administrators(BA) 全权（helper 以服务身份运行）
//   - 安装用户：若服务安装时通过 JUZHENG_ALLOWED_SID 注入其 SID，则仅授予该用户读写
//   - 回退：授予 Authenticated Users(AU) 读写（TODO：收紧为安装用户 SID，达到与 macOS UID 白名单同等隔离）
func pipeSDDL() string {
	base := "D:(A;;GA;;;SY)(A;;GA;;;BA)"
	if sid := os.Getenv(iproto.EnvAllowedSID); sid != "" {
		return base + "(A;;GRGW;;;" + sid + ")"
	}
	return base + "(A;;GRGW;;;AU)"
}

// peerUID：Windows 下准入已由管道 SDDL 强制，无需逐连接查 UID。
// 返回 0（视为 root/已授权），与 helper/main.go 中 uid==0 放行逻辑一致。
func peerUID(_ net.Conn) (uint32, error) {
	return 0, nil
}
