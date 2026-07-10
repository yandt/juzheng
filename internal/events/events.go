// Package events 集中管理 juzheng 应用的事件名常量与事件发射抽象（L0 基础设施层）。
//
// 设计目的：
//  1. 消除散落在 main.go/proxy_service.go/singbox_service.go 的事件名字符串字面量；
//  2. 提供 EventEmitter 接口，让 L1 业务模块依赖接口而非 *application.App，
//     从而解除对 Wails 的耦合（L1 不允许调上层），并消除 SetApp 后门。
//
// 下层（L1）通过 EventEmitter 接口上抛事件；上层（L3 Wails service / main）实现该接口，
// 把 Emit 转发到 application.App.Event.Emit。
package events

// 事件名常量。所有 Emit/On 调用必须引用这些常量，禁止散落字符串字面量。
const (
	// FlowUpdate 增量推送一次 HTTP 交互（request/response/sse-*），数据为 FlowUpdate 结构。
	FlowUpdate = "flow:update"

	// ProxyStarted / ProxyStopped MITM 代理启停，数据 map[string]any{addr, upstream}。
	ProxyStarted = "proxy:started"
	ProxyStopped = "proxy:stopped"

	// SingboxStarted / SingboxStopped sing-box 内核启停，数据 map[string]any{}。
	SingboxStarted = "singbox:started"
	SingboxStopped = "singbox:stopped"

	// MonitorHit 监控规则命中，数据为 mitmcore.MonitorHit（含 flowId/规则/动作/命中字段等）。
	MonitorHit = "monitor:hit"
)

// EventEmitter 是事件发射抽象。
// L1 业务模块依赖此接口（而非 *application.App），实现"下层不调上层"的解耦。
//
// 实现方（L3 service 或 main.go）负责把 Emit 转发到 Wails 的 application.App.Event.Emit。
// 替换实现（如改用 channel、测试 mock）无需改动 L1 模块 —— 这就是"可插拔"。
type EventEmitter interface {
	// Emit 发射一个事件。name 取本包的事件名常量；data 为负载数据。
	// 返回是否有监听者接收（与 application.App.Event.Emit 语义一致）。
	Emit(name string, data any) bool
}

// NoopEmitter 是空实现，用于不需要发射事件的场景（如测试、或暂未接入事件的 L1 模块）。
type NoopEmitter struct{}

func (NoopEmitter) Emit(string, any) bool { return false }
