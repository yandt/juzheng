// Package iproto 定义 juzheng-helper 与主 app 之间的 IPC 协议（L0 基础设施层）。
//
// 主 app（非 root）是客户端，helper（root 守护进程）是服务端。
// 协议为 JSON over unix socket：单行 JSON 请求 + 单行 JSON 响应。
//
// 本包同时被主 app（internal/helperclient）和 helper（helper/main.go）导入，
// 消除此前 helper/ipc.go 与 singbox_types.go 的镜像重复定义。
package iproto

import "os"

// 环境变量名：覆盖默认 socket 路径 / 指定 helper 只接受的调用方 UID。
// 集中在 L0，helper 与 client 双端共用，避免此前"helper 支持覆盖、client 写死"的不一致。
const (
	EnvSocketPath = "JUZHENG_HELPER_SOCKET" // 覆盖 socket 路径（开发/测试）
	EnvAllowedUID = "JUZHENG_ALLOWED_UID"   // [unix] helper 只接受此 UID（+root）的连接
	EnvAllowedSID = "JUZHENG_ALLOWED_SID"   // [windows] 命名管道 ACL 只允许此用户 SID（+SYSTEM/Admin）连接
)

// MaxConfigBytes 是单次 start 请求允许的配置大小上限（防止畸形/恶意超大配置撑爆内存）。
const MaxConfigBytes = 16 * 1024 * 1024

// IPC 动作常量（IPCRequest.Action 取值）。
const (
	ActionStart  = "start"  // 启动 sing-box 内核（需带 Config）
	ActionStop   = "stop"   // 停止内核
	ActionReload = "reload" // 用新配置热重载内核（Close 旧实例 + 用新配置起新实例，需带 Config）
	ActionStatus = "status" // 查询运行状态
	ActionPing   = "ping"   // 心跳探活
)

// IPCRequest 是主 app → helper 的请求。
type IPCRequest struct {
	Action string `json:"action"`            // 见 Action* 常量
	Config string `json:"config,omitempty"`  // 仅 start 时：sing-box JSON 配置内容
}

// IPCResponse 是 helper → 主 app 的响应。
type IPCResponse struct {
	OK      bool   `json:"ok"`
	Running bool   `json:"running"`           // status/start/stop 后返回当前运行状态
	Message string `json:"message,omitempty"` // 错误或提示信息
	PID     int    `json:"pid,omitempty"`     // helper 进程 pid
}

// SocketPath 是 helper 监听的默认端点，平台相关（见 transport_unix.go / transport_windows.go）：
//   - unix:    /var/run/juzheng-helper.sock（放 /var/run 需 root，helper 是 root 可写）
//   - windows: \\.\pipe\juzheng-helper（命名管道，ACL 限定安装用户 + SYSTEM）
// 两端（helper 监听 / client 拨号）都经 ResolveSocketPath 取值，保证一致。

// ResolveSocketPath 返回实际使用的端点：EnvSocketPath 覆盖优先，否则用平台默认 SocketPath。
func ResolveSocketPath() string {
	if p := os.Getenv(EnvSocketPath); p != "" {
		return p
	}
	return SocketPath
}
