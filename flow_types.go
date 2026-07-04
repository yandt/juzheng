package main

import "time"

// CapturedFlow 是一次 HTTP 交互的快照，会通过事件推给前端。
// 字段全大写导出，Wails 会据此生成 TypeScript models。
type CapturedFlow struct {
	ID         string         `json:"id"`
	Method     string         `json:"method"`
	URL        string         `json:"url"`
	Host       string         `json:"host"`
	Path       string         `json:"path"`
	ReqHeaders map[string]string `json:"reqHeaders"`
	ReqBody    string         `json:"reqBody"`
	StatusCode int            `json:"statusCode"`
	RespHeaders map[string]string `json:"respHeaders"`
	RespBody   string         `json:"respBody"`
	IsSSE      bool           `json:"isSSE"`
	SSEEvents  []SSEEventDTO  `json:"sseEvents"`
	StartTime  time.Time      `json:"startTime"`
	EndTime    *time.Time     `json:"endTime,omitempty"`
	DurationMs int64          `json:"durationMs"`
}

// SSEEventDTO 是单个 Server-Sent Event 的传输结构。
type SSEEventDTO struct {
	Event string `json:"event"`
	ID    string `json:"id"`
	Data  string `json:"data"`
}

// FlowUpdate 是增量更新事件，避免每次都推整个 flow。
type FlowUpdate struct {
	ID    string `json:"id"`
	Type  string `json:"type"`  // "request" | "response" | "sse-start" | "sse-message" | "sse-end"
	Flow  *CapturedFlow `json:"flow,omitempty"`
	SSEEvent *SSEEventDTO `json:"sseEvent,omitempty"` // 仅 sse-message 时填充
}
