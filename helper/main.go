//go:build singbox

// juzheng-helper: 以 root 运行的特权守护进程，承载 sing-box 内核（TUN 需 root）。
// 由 launchd 管理（/Library/LaunchDaemons），主 app 通过 unix socket 与之通信。
//
// 启动后：
//   1. 监听 SocketPath
//   2. chmod 0666 让非 root 的主 app 能连接
//   3. 循环接受连接，读取 JSON 请求，执行 start/stop/status/ping
//
// 编译需 singbox build tag 及 sing-box 全套 tags（见 Taskfile helper 任务）。

package main

import (
	"bufio"
	"context"
	stdjson "encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	singjson "github.com/sagernet/sing/common/json"
)

// helperState 持有 sing-box 实例，受 mu 保护。
type helperState struct {
	mu       sync.Mutex
	instance *box.Box
	cancel   context.CancelFunc
}

func main() {
	// helper 以 root 由 launchd 启动，日志写 syslog；这里也写 stderr 供 launchd 重定向。
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("juzheng-helper 启动 (pid=%d, uid=%d)", os.Getpid(), os.Getuid())

	// socket 路径：生产（root）用 /var/run，开发测试可用 JUZHENG_HELPER_SOCKET 环境变量覆盖。
	sockPath := SocketPath
	if env := os.Getenv("JUZHENG_HELPER_SOCKET"); env != "" {
		sockPath = env
	}

	state := &helperState{}

	// 清理可能残留的旧 socket。
	if err := os.RemoveAll(sockPath); err != nil {
		log.Printf("清理旧 socket 警告: %v", err)
	}

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		log.Fatalf("监听 %s 失败: %v", sockPath, err)
	}
	// 关键：让非 root 的主 app 能连接。
	if err := os.Chmod(sockPath, 0o666); err != nil {
		log.Printf("chmod socket 警告: %v", err)
	}
	log.Printf("监听 socket: %s", sockPath)

	// 信号处理：优雅退出。
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go func() {
		<-ctx.Done()
		log.Printf("收到退出信号，关闭")
		state.stop()
		ln.Close()
		os.Remove(sockPath)
		os.Exit(0)
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("accept 错误: %v", err)
			continue
		}
		go state.handle(conn)
	}
}

// handle 处理一个 IPC 连接：读一行 JSON 请求，返回一行 JSON 响应。
func (s *helperState) handle(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		s.writeResp(conn, IPCResponse{OK: false, Message: "读取请求失败: " + err.Error()})
		return
	}
	var req IPCRequest
	if err := stdjson.Unmarshal([]byte(line), &req); err != nil {
		s.writeResp(conn, IPCResponse{OK: false, Message: "解析 JSON 失败: " + err.Error()})
		return
	}
	switch req.Action {
	case "ping":
		s.writeResp(conn, IPCResponse{OK: true, Running: s.isRunning(), PID: os.Getpid()})
	case "start":
		s.writeResp(conn, s.start(req.Config))
	case "stop":
		s.writeResp(conn, s.stopResp())
	case "status":
		s.writeResp(conn, IPCResponse{OK: true, Running: s.isRunning(), PID: os.Getpid()})
	default:
		s.writeResp(conn, IPCResponse{OK: false, Message: "未知 action: " + req.Action})
	}
}

// start 启动 sing-box 内核。
func (s *helperState) start(configStr string) IPCResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.instance != nil {
		return IPCResponse{OK: true, Running: true, Message: "已在运行", PID: os.Getpid()}
	}
	if configStr == "" {
		return IPCResponse{OK: false, Message: "配置为空"}
	}

	ctx, cancel := context.WithCancel(context.Background())
	ctx = include.Context(ctx)

	options, err := singjson.UnmarshalExtendedContext[option.Options](ctx, []byte(configStr))
	if err != nil {
		cancel()
		return IPCResponse{OK: false, Message: "配置解析失败: " + E.Cause(err).Error()}
	}

	instance, err := box.New(box.Options{
		Context: ctx,
		Options: options,
	})
	if err != nil {
		cancel()
		return IPCResponse{OK: false, Message: "创建 sing-box 实例失败: " + err.Error()}
	}
	if err := instance.Start(); err != nil {
		cancel()
		return IPCResponse{OK: false, Message: "启动 sing-box 失败: " + err.Error()}
	}

	s.instance = instance
	s.cancel = cancel
	log.Printf("sing-box 已启动")
	return IPCResponse{OK: true, Running: true, PID: os.Getpid()}
}

// stopResp 停止内核，返回响应。
func (s *helperState) stopResp() IPCResponse {
	s.stop()
	return IPCResponse{OK: true, Running: false, PID: os.Getpid()}
}

// stop 停止内核（无锁版，供信号处理调用前已持锁或独立调用）。
func (s *helperState) stop() {
	s.mu.Lock()
	instance := s.instance
	cancel := s.cancel
	s.instance = nil
	s.cancel = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if instance != nil {
		if err := instance.Close(); err != nil {
			log.Printf("sing-box 关闭出错: %v", err)
		}
		log.Printf("sing-box 已停止")
	}
}

func (s *helperState) isRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.instance != nil
}

func (s *helperState) writeResp(conn net.Conn, resp IPCResponse) {
	data, _ := stdjson.Marshal(resp)
	fmt.Fprintf(conn, "%s\n", data)
}
