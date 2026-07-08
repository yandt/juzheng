// sing-box service 共享类型（无 build tag）。
//
// IPCRequest/IPCResponse/SocketPath 已迁移至 internal/iproto（L0）。
// AppMeta/CardConfig/卡片常量 已迁移至 internal/singboxcfg（L1）。
// SubscriptionMeta/SubscriptionInfo 已迁移至 subscription_service.go（L3）。
// 本文件仅保留 IPC 协议别名（helper 二进制共用）+ 卡片常量转发。

package main

import (
	"github.com/zhanghui/juzheng/internal/iproto"
	"github.com/zhanghui/juzheng/internal/mitmcore"
	"github.com/zhanghui/juzheng/internal/singboxcfg"
	"github.com/zhanghui/juzheng/internal/subscriptions"
)

// SingBoxServiceOptions 是前端可调的 sing-box 运行参数。
type SingBoxServiceOptions struct {
	ConfigPath string `json:"configPath"` // sing-box JSON 配置路径
}

// IPCRequest / IPCResponse 转发到 internal/iproto，保持旧调用点兼容。
type IPCRequest = iproto.IPCRequest
type IPCResponse = iproto.IPCResponse

const SocketPath = iproto.SocketPath

// AppMeta / CardConfig 转发到 internal/singboxcfg，保持 Wails binding 兼容。
type AppMeta = singboxcfg.AppMeta
type CardConfig = singboxcfg.CardConfig

// 卡片标识常量转发（前端 HomePage 据此渲染）。
const (
	CardCurrentSubscription = singboxcfg.CardCurrentSubscription
	CardProxyGroups         = singboxcfg.CardProxyGroups
	CardQuickToggles        = singboxcfg.CardQuickToggles
	CardTraffic             = singboxcfg.CardTraffic
)

// 以下类型别名确保 Wails binding 生成时把它们收入 models.ts（前端类型检查依赖）。
// 这些类型在 L1 内部包定义，若主包不暴露别名，binding 生成器不会为它们生成前端类型。
// CardConfig 已在上面定义（singboxcfg.CardConfig），这里只补 mitmcore/subscriptions 的类型。
type CapturedFlow = mitmcore.CapturedFlow
type SSEEventDTO = mitmcore.SSEEventDTO
type SubscriptionInfo = subscriptions.Info
