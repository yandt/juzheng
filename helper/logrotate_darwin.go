//go:build singbox && darwin

package main

import (
	"os"
	"syscall"
)

// logFilePath 是 macOS 下 helper 的日志路径。
// 与 plist 里 StandardOutPath/StandardErrorPath 保持同一路径：dup2 之后 launchd 原来指向该
// 文件的 fd 已被换成管道，文件所有权归 lumberjack，故无需改 plist —— 存量安装重启 helper
// 即获得轮转。
func logFilePath() string { return "/var/log/juzheng-helper.log" }

// redirectStdio 把进程的 fd 1/2 换成 w（管道写端）。
//
// 用 dup2 而不是仅给 os.Stdout/os.Stderr 变量赋值：赋值只能影响之后读取该变量的 Go 代码，
// 而 dup2 是在文件描述符层面替换，任何持有原 fd 的写入（含 launchd 的重定向、以及可能的
// 非 Go 代码）都会一并流入管道。Go 的 os.Stdout/os.Stderr 本身就包着 fd 1/2，故无需再改变量。
func redirectStdio(w *os.File) error {
	if err := syscall.Dup2(int(w.Fd()), int(os.Stdout.Fd())); err != nil {
		return err
	}
	return syscall.Dup2(int(w.Fd()), int(os.Stderr.Fd()))
}
