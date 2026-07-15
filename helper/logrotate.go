//go:build singbox

// helper 日志轮转：把本进程的全部日志（helper 自身的 log.Printf + sing-box 内核日志）
// 收敛到一个带轮转的文件，避免无限增长。
//
// 为什么需要：sing-box 在 info 级别会为每条连接打日志，而 launchd(mac)/服务(win) 只是把
// stdout/stderr 重定向到一个文件，没有任何轮转 —— 实测跑几天就到 1.4 GB。
//
// 日志是怎么被接管的：
//   1. helper 自身：log.SetOutput(lumberjack)。
//   2. sing-box 内核：它在 box.New 时读取 os.Stderr（见 sing-box log.New：Output 为空且
//      DefaultWriter 为 nil 时用 os.Stderr）。故必须在任何 box.New 之前完成重定向。
//      具体做法见 redirectStdio 的平台实现。
//
// 注意：轮转由 lumberjack 负责，它会 rename + reopen 同一路径，因此这个路径不能同时被
// launchd 持有写入 —— redirectStdio 在 unix 上用 dup2 把 fd 1/2 换成管道，正是为此：
// launchd 原来指向日志文件的 fd 被替换掉，文件的所有权交给 lumberjack，无需改 plist
// （存量安装重启 helper 即生效）。
package main

import (
	"io"
	"log"
	"os"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

// 轮转策略：单文件 16MB，保留 3 个历史，最长 7 天，历史压缩。
// 上限约 16MB + 3 个压缩历史，正常在几十 MB 量级（原本无上限）。
const (
	logMaxSizeMB  = 16
	logMaxBackups = 3
	logMaxAgeDays = 7
)

// setupLogging 装配日志轮转。必须在 main 早期、任何 box.New 之前调用。
// 失败时不阻断启动（日志坏了也不该让内核起不来），仅退回原始 stderr 并打一行提示。
func setupLogging() {
	lj := &lumberjack.Logger{
		Filename:   logFilePath(),
		MaxSize:    logMaxSizeMB,
		MaxBackups: logMaxBackups,
		MaxAge:     logMaxAgeDays,
		Compress:   true,
	}

	// helper 自身的 log.Printf。
	log.SetOutput(lj)

	// 内核日志：把 stdout/stderr 接到管道，goroutine 持续搬运到 lumberjack。
	r, w, err := os.Pipe()
	if err != nil {
		log.Printf("日志轮转：创建管道失败，内核日志仍走原 stderr: %v", err)
		return
	}
	if err := redirectStdio(w); err != nil {
		log.Printf("日志轮转：重定向 stdout/stderr 失败，内核日志仍走原 stderr: %v", err)
		_ = r.Close()
		_ = w.Close()
		return
	}
	go func() {
		// 随进程存活；进程退出时管道关闭，Copy 自然返回。
		_, _ = io.Copy(lj, r)
	}()
	log.Printf("日志轮转已启用: %s (单文件 %dMB, 保留 %d 个, %d 天)",
		lj.Filename, logMaxSizeMB, logMaxBackups, logMaxAgeDays)
}
