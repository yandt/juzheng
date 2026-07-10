// Package helperclient 是 juzheng-helper 守护进程的客户端（L1 系统层）。
//
// 职责：
//   - 通过 IPC（unix socket / 命名管道）向 helper 发送请求（启动/停止/查询 sing-box 内核）
//   - helper 二进制的部署/卸载/状态查询（平台相关：launchd / Windows 服务）
//   - 内嵌 helper 二进制 + 平台部署资源
//
// 平台实现分离：
//   - helperclient_darwin.go：launchd + osascript 提权，内嵌 macOS helper + plist
//   - helperclient_windows.go：Windows 服务(SCM) + UAC 提权，内嵌 helper.exe
//
// 依赖：L0 iproto（IPC 协议）。不依赖 Wails、不依赖事件系统。
package helperclient

import (
	"bufio"
	stdjson "encoding/json"
	"time"

	"github.com/zhanghui/juzheng/internal/iproto"
)

// Status 反映 helper 守护进程的安装与运行状态。
type Status struct {
	Installed  bool   `json:"installed"`  // 是否已部署（plist / 服务已注册）
	Running    bool   `json:"running"`    // helper 进程是否在运行
	Loaded     bool   `json:"loaded"`     // 服务管理器是否已加载（launchd / SCM）
	PID        int    `json:"pid"`        // helper pid
	InstallCmd string `json:"installCmd"` // 安装命令（供复制）
}

// Call 向 helper 发送一个 IPC 请求并返回响应。
// action 见 iproto.Action* 常量；config 仅 start 时需要。传输由 iproto.Dial 按平台选择。
func Call(action, config string) (*iproto.IPCResponse, error) {
	conn, err := iproto.Dial(2 * time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	req := iproto.IPCRequest{Action: action, Config: config}
	data, err := stdjson.Marshal(req)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write(append(data, '\n')); err != nil {
		return nil, err
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return nil, err
	}
	var resp iproto.IPCResponse
	if err := stdjson.Unmarshal([]byte(line), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
