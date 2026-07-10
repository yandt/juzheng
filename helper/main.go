//go:build singbox

// juzheng-helper: 以 root 运行的特权守护进程，承载 sing-box 内核（TUN 需 root）。
// 由 launchd 管理（/Library/LaunchDaemons），主 app 通过 unix socket 与之通信。
//
// 启动后：
//   1. 监听 socket（路径由 iproto.ResolveSocketPath 决定）
//   2. 按 UID 白名单收紧 socket 权限（仅允许安装用户 + root 连接）
//   3. 循环接受连接，读取 JSON 请求，执行 start/stop/status/ping
//
// 编译需 singbox build tag 及 sing-box 全套 tags（见 Taskfile helper 任务）。

package main

import (
	"bufio"
	"context"
	stdjson "encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	singjson "github.com/sagernet/sing/common/json"
	"github.com/zhanghui/juzheng/internal/iproto"
)

// readTimeout 是单次 IPC 请求的读取超时，防止挂起的客户端泄漏 goroutine。
const readTimeout = 10 * time.Second

// helperState 持有 sing-box 实例，受 mu 保护。
type helperState struct {
	mu       sync.Mutex
	instance *box.Box
	cancel   context.CancelFunc

	// allowedUID 是被允许连接的非 root UID。-1 表示未配置（开发回退：不做 UID 限制）。
	allowedUID int
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// 提权动作（Windows：--install/--uninstall，由主 app 经 UAC 拉起）。处理完即退出。
	// macOS 恒为 false（安装走 launchd）。
	if handleAdminArgs() {
		return
	}

	log.Printf("juzheng-helper 启动 (pid=%d, uid=%d)", os.Getpid(), os.Getuid())

	sockPath := iproto.ResolveSocketPath()

	state := &helperState{allowedUID: -1}
	if v := os.Getenv(iproto.EnvAllowedUID); v != "" {
		if uid, err := strconv.Atoi(v); err == nil && uid >= 0 {
			state.allowedUID = uid
			log.Printf("UID 白名单：仅允许 UID %d 与 root 连接", uid)
		} else {
			log.Printf("警告：%s=%q 无法解析为 UID，回退为不限制", iproto.EnvAllowedUID, v)
		}
	} else {
		log.Printf("警告：未设置 %s，socket 不做 UID 限制（仅建议开发环境）", iproto.EnvAllowedUID)
	}

	// 监听端点：平台相关（unix socket + 权限收紧 / 命名管道 + SDDL），见 transport_*.go。
	ln, err := listen(sockPath, state.allowedUID)
	if err != nil {
		log.Fatalf("监听 %s 失败: %v", sockPath, err)
	}
	log.Printf("监听端点: %s", sockPath)

	// 驱动服务：平台相关（macOS/console 走信号；Windows 走 SCM 服务或 console），见 run_*.go。
	// 驱动内部调用 serveLoop 接受连接，退出前停内核、清理端点。
	runServer(state, ln, sockPath)
}

// serveLoop 循环接受连接直到 ctx 取消；处理完在途连接后停内核、清理端点。
// 被各平台驱动（run_darwin.go / run_windows.go）复用。
func (s *helperState) serveLoop(ctx context.Context, ln net.Listener, sockPath string) {
	var wg sync.WaitGroup
	go func() {
		<-ctx.Done()
		log.Printf("停止接受新连接")
		ln.Close() // 使 Accept 返回错误，跳出主循环
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				break // ctx 取消导致的 Accept 失败：正常退出
			}
			log.Printf("accept 错误: %v", err)
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.handle(conn)
		}()
	}

	wg.Wait()    // 等所有在途连接处理完
	s.stop()     // 停止内核
	os.Remove(sockPath)
	log.Printf("juzheng-helper 已优雅退出")
}

// handle 处理一个 IPC 连接：校验调用方 → 读一行 JSON 请求 → 返回一行 JSON 响应。
func (s *helperState) handle(conn net.Conn) {
	defer conn.Close()

	// 1. 校验调用方 UID（root 守护进程必须确认对端身份，防止任意本地进程控制内核）。
	if s.allowedUID >= 0 {
		uid, err := peerUID(conn)
		if err != nil {
			log.Printf("获取对端凭证失败，拒绝连接: %v", err)
			s.writeResp(conn, IPCResponse{OK: false, Message: "无法校验调用方身份"})
			return
		}
		if int(uid) != s.allowedUID && uid != 0 {
			log.Printf("拒绝来自 UID %d 的连接（仅允许 %d 与 root）", uid, s.allowedUID)
			s.writeResp(conn, IPCResponse{OK: false, Message: "调用方无权限"})
			return
		}
	}

	// 2. 读请求：设超时防挂起；用 LimitReader 限制单请求最大字节，防超大 payload 撑爆内存。
	conn.SetReadDeadline(time.Now().Add(readTimeout))
	reader := bufio.NewReader(io.LimitReader(conn, iproto.MaxConfigBytes+4096))
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
	log.Printf("IPC 请求: action=%s", req.Action)

	switch req.Action {
	case iproto.ActionPing:
		s.writeResp(conn, IPCResponse{OK: true, Running: s.isRunning(), PID: os.Getpid()})
	case iproto.ActionStart:
		s.writeResp(conn, s.start(req.Config))
	case iproto.ActionReload:
		s.writeResp(conn, s.reload(req.Config))
	case iproto.ActionStop:
		s.writeResp(conn, s.stopResp())
	case iproto.ActionStatus:
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
	if len(configStr) > iproto.MaxConfigBytes {
		return IPCResponse{OK: false, Message: "配置过大"}
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
		instance.Close() // 启动失败也要释放已分配资源
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

// reload 用新配置热重载内核：先关掉旧实例，再用新配置起新实例（进程内完成，不重启 helper）。
// 供路由规则 / 抓包域名 / DNS 等结构性配置改动后即时生效——sing-box 只在启动时加载配置，
// box.Box 无原地重载，故走 Close+New+Start。未运行时不自动拉起（返回未运行）。
func (s *helperState) reload(configStr string) IPCResponse {
	s.mu.Lock()
	notRunning := s.instance == nil
	s.mu.Unlock()
	if notRunning {
		return IPCResponse{OK: false, Running: false, Message: "内核未运行，无需重载"}
	}
	s.stop()
	return s.start(configStr)
}

// stop 停止内核。加锁取出实例后在锁外关闭，避免 Close 阻塞持锁。
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
	data, err := stdjson.Marshal(resp)
	if err != nil {
		// 序列化响应本身失败：回一个最简合法 JSON，避免客户端收到畸形数据卡死解析。
		fmt.Fprint(conn, "{\"ok\":false,\"message\":\"序列化响应失败\"}\n")
		return
	}
	fmt.Fprintf(conn, "%s\n", data)
}
