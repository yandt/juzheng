// Package sysproxy 管理系统代理设置（L1 系统层）。
//
// 职责：查询/启用/关闭系统代理（HTTP/HTTPS/SOCKS）、设置绕过列表。
// 平台实现分离：
//   - sysproxy_darwin.go：networksetup + scutil，改全局设置用 osascript 提权
//   - sysproxy_windows.go：注册表 Internet Settings + WinINet 刷新
//
// 本文件只放跨平台共享的类型与默认值；Get / Set 由各平台文件实现。
// 依赖：仅 stdlib。不依赖 Wails、不依赖事件系统。
package sysproxy

// DefaultMixedPort 默认 mixed inbound 端口（与配置模板 mixed-back 一致）。
const DefaultMixedPort = 9788

// State 反映系统代理当前状态（也用作设置项）。
type State struct {
	Enabled     bool     `json:"enabled"`     // 是否已设为系统代理
	Server      string   `json:"server"`      // 代理服务器（如 127.0.0.1）
	Port        int      `json:"port"`        // 代理端口
	EnableHTTP  bool     `json:"enableHttp"`  // 启用 HTTP 代理
	EnableHTTPS bool     `json:"enableHttps"` // 启用 HTTPS 代理
	EnableSOCKS bool     `json:"enableSocks"` // 启用 SOCKS 代理
	Bypass      []string `json:"bypass"`      // 绕过代理的域名/IP 列表
}

// DefaultState 返回默认系统代理设置。
func DefaultState() State {
	return State{
		Server:      "127.0.0.1",
		Port:        DefaultMixedPort,
		EnableHTTP:  true,
		EnableHTTPS: true,
		EnableSOCKS: true,
		Bypass:      []string{"127.0.0.1", "192.168.0.0/16", "10.0.0.0/8", "172.16.0.0/12", "localhost", "*.local"},
	}
}
