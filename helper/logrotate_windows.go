//go:build singbox && windows

package main

import (
	"os"
	"path/filepath"
)

// logFilePath 是 Windows 下 helper 的日志路径：%ProgramData%\Juzheng\juzheng-helper.log
// （与 helper.exe / wintun.dll 同目录，复用 installDirWin）。
func logFilePath() string { return filepath.Join(installDirWin(), "juzheng-helper.log") }

// redirectStdio 把 sing-box 的日志出口换成 w。
//
// 与 macOS 不同，这里不做 fd 层面的 dup2：helper 作为 Windows 服务运行时没有控制台，
// fd 1/2 本就无效，没有「launchd 持有日志文件 fd」这类需要在 fd 层面夺回的情况。
// sing-box 在 box.New 时读取 os.Stderr 变量（见 sing-box log.New），故只需替换该变量，
// 且必须在任何 box.New 之前完成 —— setupLogging 在 main 早期调用即满足。
func redirectStdio(w *os.File) error {
	os.Stdout = w
	os.Stderr = w
	return nil
}
