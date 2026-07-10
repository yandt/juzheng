//go:build singbox && darwin

// run_darwin.go：macOS 下 helper 的运行驱动 —— 信号触发优雅退出。

package main

import (
	"context"
	"net"
	"os/signal"
	"syscall"
)

// handleAdminArgs：macOS 的 helper 安装走 launchd（helperclient osascript），helper 自身无提权动作参数。
func handleAdminArgs() bool { return false }

// runServer 以 console/daemon 方式运行：SIGINT/SIGTERM 触发优雅退出。
// launchd 停止服务时向进程发 SIGTERM，据此清理内核与 socket。
func runServer(s *helperState, ln net.Listener, sockPath string) {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	s.serveLoop(ctx, ln, sockPath)
}
