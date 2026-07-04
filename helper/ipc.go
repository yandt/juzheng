// helper 与主 app 之间的 IPC 协议（JSON over unix socket）。
// 主 app 是客户端，helper 是服务端。协议为单行 JSON 请求 + 单行 JSON 响应。

package main

// IPC 请求。
type IPCRequest struct {
	Action  string `json:"action"`            // "start" | "stop" | "status" | "ping"
	Config  string `json:"config,omitempty"`  // 仅 start 时：sing-box JSON 配置内容
}

// IPC 响应。
type IPCResponse struct {
	OK      bool   `json:"ok"`
	Running bool   `json:"running"`           // status/start/stop 后返回当前运行状态
	Message string `json:"message,omitempty"` // 错误或提示信息
	PID     int    `json:"pid,omitempty"`     // helper 进程 pid
}

// SocketPath 是 helper 监听的 unix socket 路径。
// 放 /var/run 需 root（helper 是 root，可写；主 app 可读连接）。
// 为让非 root 主 app 能连接，helper 启动后会对 socket chmod 0666。
const SocketPath = "/var/run/juzheng-helper.sock"
