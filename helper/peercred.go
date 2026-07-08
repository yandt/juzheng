//go:build singbox && darwin

// peercred.go：读取 unix socket 对端进程凭证（macOS LOCAL_PEERCRED）。
// helper 以 root 运行，据此校验调用方 UID，只接受安装用户（+root）的连接。

package main

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// peerUID 返回 unix socket 对端进程的有效 UID。
func peerUID(conn net.Conn) (uint32, error) {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, fmt.Errorf("非 unix 连接，无法获取对端凭证")
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return 0, err
	}
	var cred *unix.Xucred
	var credErr error
	if err := raw.Control(func(fd uintptr) {
		cred, credErr = unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
	}); err != nil {
		return 0, err
	}
	if credErr != nil {
		return 0, credErr
	}
	return cred.Uid, nil
}
