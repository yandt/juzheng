// sing-box service 共享类型（无 build tag）。

package main

// SingBoxServiceOptions 是前端可调的 sing-box 运行参数。
type SingBoxServiceOptions struct {
	ConfigPath string `json:"configPath"` // sing-box JSON 配置路径
}

// IPCRequest 是主 app 与 helper 之间的协议请求（与 helper/ipc.go 镜像）。
type IPCRequest struct {
	Action string `json:"action"`          // "start" | "stop" | "status" | "ping"
	Config string `json:"config,omitempty"` // 仅 start 时：sing-box JSON 配置内容
}

// IPCResponse 是 helper 返回的协议响应。
type IPCResponse struct {
	OK      bool   `json:"ok"`
	Running bool   `json:"running"`
	Message string `json:"message,omitempty"`
	PID     int    `json:"pid,omitempty"`
}

