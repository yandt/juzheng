package main

// flow_types.go：MITM 流量类型别名。
//
// CapturedFlow / SSEEventDTO / FlowUpdate 已迁移至 internal/mitmcore（L1）。
// 类型别名现统一在 singbox_types.go 管理（确保 Wails binding 生成收入 models.ts）。
// 本文件保留 FlowUpdate 别名（mitm 专用，避免 singbox_types 过臃肿）。

import "github.com/zhanghui/juzheng/internal/mitmcore"

type FlowUpdate = mitmcore.FlowUpdate
