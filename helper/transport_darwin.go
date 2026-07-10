//go:build singbox && darwin

// transport_darwin.go：helper 在 macOS 的监听端点 —— unix domain socket + UID 权限收紧。

package main

import (
	"log"
	"net"
	"os"
)

// listen 在 unix socket 上监听，并按白名单 UID 收紧权限。
// 先清理可能残留的旧 socket 文件，再 net.Listen，最后 chown/chmod。
func listen(sockPath string, allowedUID int) (net.Listener, error) {
	if err := os.RemoveAll(sockPath); err != nil {
		log.Printf("清理旧 socket 警告: %v", err)
	}
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return nil, err
	}
	applySocketPerms(sockPath, allowedUID)
	return ln, nil
}

// applySocketPerms 收紧 socket 权限：配置了白名单 UID 时 chown 给它并设为 0600
// （仅该用户与 root 可连接）；未配置时回退为 0666（开发方便）。
func applySocketPerms(sockPath string, allowedUID int) {
	if allowedUID >= 0 {
		if err := os.Chown(sockPath, allowedUID, -1); err != nil {
			log.Printf("chown socket 警告: %v", err)
		}
		if err := os.Chmod(sockPath, 0o600); err != nil {
			log.Printf("chmod socket 警告: %v", err)
		}
		return
	}
	if err := os.Chmod(sockPath, 0o666); err != nil {
		log.Printf("chmod socket 警告: %v", err)
	}
}
