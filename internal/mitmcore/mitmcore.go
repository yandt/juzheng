// Package mitmcore 封装 go-mitmproxy HTTPS 抓包代理（L1 系统层）。
//
// 职责：
//   - go-mitmproxy 代理的启动/停止（独立 goroutine + context 控制）
//   - 流量捕获（captureAddon）：请求/响应/SSE 转 CapturedFlow
//   - CA 证书管理：路径查询/状态（生成/指纹/信任）/一键信任（osascript 提权）
//
// 依赖：L0 events（EventEmitter 接口 + 事件常量）、L0 paths（CA 目录）。
// 不依赖 Wails 的 *application.App —— 通过 EventEmitter 接口上抛事件，
// 实现"下层不调上层"的解耦，便于测试和替换。
package mitmcore

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
	"github.com/zhanghui/juzheng/internal/events"
	"github.com/zhanghui/juzheng/internal/paths"
)

// Options 是代理运行参数。
type Options struct {
	Addr        string `json:"addr"`        // 监听地址，默认 :9080
	Upstream    string `json:"upstream"`    // 上游代理（链式到 sing-box），可空
	SslInsecure bool   `json:"sslInsecure"` // 是否跳过上游 TLS 校验
	CaRootPath  string `json:"caRootPath"`  // CA 证书目录
}

// 默认配置。
const (
	DefaultAddr     = ":9080"
	DefaultUpstream = ""
)

// DefaultOptions 返回默认配置（CA 目录自动指向 ~/.juzheng/ca）。
func DefaultOptions() Options {
	caDir, _ := paths.CaDir()
	return Options{
		Addr:        DefaultAddr,
		Upstream:    DefaultUpstream,
		SslInsecure: true,
		CaRootPath:  caDir,
	}
}

// ===== 流量数据结构（与原 flow_types.go JSON 兼容）=====

// CapturedFlow 是一次 HTTP 交互的快照。
type CapturedFlow struct {
	ID          string             `json:"id"`
	Method      string             `json:"method"`
	URL         string             `json:"url"`
	Host        string             `json:"host"`
	Path        string             `json:"path"`
	ReqHeaders  map[string]string  `json:"reqHeaders"`
	ReqBody     string             `json:"reqBody"`
	StatusCode  int                `json:"statusCode"`
	RespHeaders map[string]string  `json:"respHeaders"`
	RespBody    string             `json:"respBody"`
	IsSSE       bool               `json:"isSSE"`
	SSEEvents   []SSEEventDTO      `json:"sseEvents"`
	StartTime   time.Time          `json:"startTime"`
	EndTime     *time.Time         `json:"endTime,omitempty"`
	DurationMs  int64              `json:"durationMs"`
}

// SSEEventDTO 是单个 Server-Sent Event 的传输结构。
type SSEEventDTO struct {
	Event string `json:"event"`
	ID    string `json:"id"`
	Data  string `json:"data"`
}

// FlowUpdate 是增量更新事件负载。
type FlowUpdate struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"` // request | response | sse-start | sse-message | sse-end
	Flow     *CapturedFlow  `json:"flow,omitempty"`
	SSEEvent *SSEEventDTO   `json:"sseEvent,omitempty"`
}

// CertStatus 反映 CA 证书的就绪与信任状态。
type CertStatus struct {
	Exists      bool   `json:"exists"`
	Path        string `json:"path"`
	Trusted     bool   `json:"trusted"`
	InstallCmd  string `json:"installCmd"`
	Fingerprint string `json:"fingerprint"`
}

// ===== 代理控制器 =====

// Proxy 是 mitm 代理控制器（替代原 MitmProxyService 的领域逻辑部分）。
// 通过 EventEmitter 接口上抛事件，不依赖 Wails。
type Proxy struct {
	emitter events.EventEmitter // 事件发射器（接口注入）

	mu      sync.Mutex
	options Options
	proxy   *proxy.Proxy
	running bool
	cancel  context.CancelFunc

	monitor *monitorEngine // 内容层监控规则引擎（运行中可热更新）
	capture *captureAddon  // 流量捕获 addon（持引用以便前端补拉已抓流量）
}

// New 创建代理控制器。emitter 用于上抛 flow:update / proxy:started/stopped 事件。
func New(emitter events.EventEmitter) *Proxy {
	if emitter == nil {
		emitter = events.NoopEmitter{}
	}
	return &Proxy{
		emitter: emitter,
		options: DefaultOptions(),
		monitor: newMonitorEngine(),
	}
}

// SetMonitorRules 热更新内容层监控规则（运行中即时生效，无需重启代理）。
func (p *Proxy) SetMonitorRules(rules []MonitorRule) {
	p.monitor.SetRules(rules)
}

// Flows 返回当前已抓流量（供前端打开页面时补拉，弥补漏收的实时事件）。
func (p *Proxy) Flows() []CapturedFlow {
	p.mu.Lock()
	c := p.capture
	p.mu.Unlock()
	if c == nil {
		return []CapturedFlow{}
	}
	return c.List()
}

// SetEmitter 运行时替换事件发射器（用于 service 注入 app 后回调）。
func (p *Proxy) SetEmitter(emitter events.EventEmitter) {
	if emitter != nil {
		p.emitter = emitter
	}
}

// Options 返回当前配置。
func (p *Proxy) Options() Options {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.options
}

// SetOptions 更新配置（仅在代理停止时可改）。
func (p *Proxy) SetOptions(opts Options) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return fmt.Errorf("代理运行中，请先停止")
	}
	if opts.Addr != "" {
		p.options.Addr = opts.Addr
	}
	p.options.Upstream = opts.Upstream
	p.options.SslInsecure = opts.SslInsecure
	if opts.CaRootPath != "" {
		p.options.CaRootPath = opts.CaRootPath
	}
	return nil
}

// IsRunning 返回代理是否在运行。
func (p *Proxy) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// Start 启动代理，返回监听地址。
func (p *Proxy) Start() (string, error) {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return "", fmt.Errorf("代理已在运行")
	}
	opts := p.options
	emitter := p.emitter
	p.mu.Unlock()

	if err := os.MkdirAll(opts.CaRootPath, 0o755); err != nil {
		return "", fmt.Errorf("创建 CA 目录失败: %w", err)
	}

	proxyOpts := &proxy.Options{
		Addr:        opts.Addr,
		SslInsecure: opts.SslInsecure,
		CaRootPath:  opts.CaRootPath,
	}
	if opts.Upstream != "" {
		proxyOpts.Upstream = opts.Upstream
	}

	pr, err := proxy.NewProxy(proxyOpts)
	if err != nil {
		return "", fmt.Errorf("创建代理失败: %w", err)
	}
	capture := &captureAddon{emitter: emitter}
	pr.AddAddon(capture)
	// 监控 addon 挂在捕获之后：先记录原始流量，再评估监控规则并执行动作。
	pr.AddAddon(&monitorAddon{engine: p.monitor, emitter: emitter})

	ctx, cancel := context.WithCancel(context.Background())

	p.mu.Lock()
	p.proxy = pr
	p.capture = capture
	p.running = true
	p.cancel = cancel
	addr := opts.Addr
	p.mu.Unlock()

	go func() {
		runErr := serve(ctx, pr)
		p.mu.Lock()
		p.running = false
		p.proxy = nil
		p.cancel = nil
		p.mu.Unlock()
		if runErr != nil && ctx.Err() == nil {
			log.Printf("代理异常退出: %v", runErr)
		}
		emitter.Emit(events.ProxyStopped, map[string]any{"addr": addr})
	}()

	emitter.Emit(events.ProxyStarted, map[string]any{"addr": addr, "upstream": opts.Upstream})
	return addr, nil
}

// serve 在 ctx 取消时优雅关闭代理。
func serve(ctx context.Context, p *proxy.Proxy) error {
	errCh := make(chan error, 1)
	go func() { errCh <- p.Start() }()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return p.Shutdown(shutdownCtx)
	}
}

// Stop 停止代理。
func (p *Proxy) Stop() error {
	p.mu.Lock()
	cancel := p.cancel
	p.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	return nil
}

// ===== CA 证书管理 =====

// CaCertPath 返回 CA 证书文件路径。
func (p *Proxy) CaCertPath() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return filepath.Join(p.options.CaRootPath, "mitmproxy-ca-cert.cer")
}

// GetCertStatus 返回证书状态（代理首次 Start 后证书才会生成）。
func (p *Proxy) GetCertStatus() CertStatus {
	path := p.CaCertPath()
	st := CertStatus{Path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		return st
	}
	st.Exists = true
	if cert, err := x509.ParseCertificate(data); err == nil {
		st.Fingerprint = fmt.Sprintf("%X", sha256sum(cert.Raw))
	}
	st.InstallCmd = certInstallCmd(path) // 平台相关：见 cert_darwin.go / cert_windows.go
	st.Trusted = isCertTrusted(path)
	return st
}

// InstallCert 将 CA 证书装入系统信任存储（平台相关，见 cert_darwin.go / cert_windows.go）。
// 证书需先由代理生成（首次 Start 后）。
func (p *Proxy) InstallCert() (string, error) {
	path := p.CaCertPath()
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("证书尚未生成，请先启动一次代理: %w", err)
	}
	return installCertTrust(path)
}

func sha256sum(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// ===== captureAddon（流量捕获，通过 EventEmitter 上抛）=====

// defaultMaxFlows 后端保留的已抓流量上限（超出淘汰最旧），防止长时间抓包无限增长。
// 与前端 useMitm 的 MAX_FLOWS 对齐。
const defaultMaxFlows = 1000

type captureAddon struct {
	proxy.BaseAddon
	emitter events.EventEmitter

	mu    sync.Mutex
	flows map[string]*CapturedFlow
	order []string // 按到达顺序记录 flow id，用于上限淘汰 + 有序返回（供前端补拉）
	max   int      // flows 上限，<=0 用 defaultMaxFlows
}

func (a *captureAddon) ensureMap() {
	if a.flows == nil {
		a.flows = make(map[string]*CapturedFlow)
	}
}

// List 返回已抓流量（按到达顺序，旧→新）。供前端打开页面时补拉，避免漏掉实时事件的流量。
func (a *captureAddon) List() []CapturedFlow {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]CapturedFlow, 0, len(a.order))
	for _, id := range a.order {
		if cf, ok := a.flows[id]; ok {
			out = append(out, *cf)
		}
	}
	return out
}

func headerToMap(h http.Header) map[string]string {
	m := make(map[string]string, len(h))
	for k, vs := range h {
		combined := ""
		for i, v := range vs {
			if i > 0 {
				combined += ", "
			}
			combined += v
		}
		m[k] = combined
	}
	return m
}

func (a *captureAddon) Request(f *proxy.Flow) {
	a.mu.Lock()
	a.ensureMap()
	cf := &CapturedFlow{
		ID:         f.Id.String(),
		Method:     f.Request.Method,
		URL:        f.Request.URL.String(),
		Host:       f.Request.URL.Host,
		Path:       f.Request.URL.Path,
		ReqHeaders: headerToMap(f.Request.Header),
		StartTime:  f.StartTime,
	}
	if body, err := f.Request.DecodedBody(); err == nil {
		cf.ReqBody = string(body)
	} else {
		cf.ReqBody = string(f.Request.Body)
	}
	a.flows[cf.ID] = cf
	a.order = append(a.order, cf.ID)
	max := a.max
	if max <= 0 {
		max = defaultMaxFlows
	}
	for len(a.order) > max { // 环形淘汰最旧，与前端上限对齐
		old := a.order[0]
		a.order = a.order[1:]
		delete(a.flows, old)
	}
	a.mu.Unlock()

	a.emitter.Emit(events.FlowUpdate, FlowUpdate{ID: cf.ID, Type: "request", Flow: cf})
}

func (a *captureAddon) Response(f *proxy.Flow) {
	a.mu.Lock()
	a.ensureMap()
	cf, ok := a.flows[f.Id.String()]
	if !ok {
		cf = &CapturedFlow{ID: f.Id.String(), Method: f.Request.Method, URL: f.Request.URL.String()}
		a.flows[cf.ID] = cf
	}
	cf.StatusCode = f.Response.StatusCode
	cf.RespHeaders = headerToMap(f.Response.Header)
	if body, err := f.Response.DecodedBody(); err == nil {
		cf.RespBody = string(body)
	} else {
		cf.RespBody = string(f.Response.Body)
	}
	now := time.Now()
	cf.EndTime = &now
	cf.DurationMs = now.Sub(cf.StartTime).Milliseconds()
	clone := *cf
	a.mu.Unlock()

	a.emitter.Emit(events.FlowUpdate, FlowUpdate{ID: cf.ID, Type: "response", Flow: &clone})
}

func (a *captureAddon) SSEStart(f *proxy.Flow) {
	a.mu.Lock()
	if cf, ok := a.flows[f.Id.String()]; ok {
		cf.IsSSE = true
	}
	a.mu.Unlock()
	a.emitter.Emit(events.FlowUpdate, FlowUpdate{ID: f.Id.String(), Type: "sse-start"})
}

func (a *captureAddon) SSEMessage(f *proxy.Flow) {
	sseEvents := f.SSE.Events
	if len(sseEvents) == 0 {
		return
	}
	ev := sseEvents[len(sseEvents)-1]
	dto := SSEEventDTO{Event: ev.Event, ID: ev.ID, Data: string(ev.Data)}

	a.mu.Lock()
	if cf, ok := a.flows[f.Id.String()]; ok {
		cf.SSEEvents = append(cf.SSEEvents, dto)
	}
	a.mu.Unlock()

	a.emitter.Emit(events.FlowUpdate, FlowUpdate{ID: f.Id.String(), Type: "sse-message", SSEEvent: &dto})
}

func (a *captureAddon) SSEEnd(f *proxy.Flow) {
	a.mu.Lock()
	if cf, ok := a.flows[f.Id.String()]; ok {
		now := time.Now()
		cf.EndTime = &now
		cf.DurationMs = now.Sub(cf.StartTime).Milliseconds()
	}
	a.mu.Unlock()
	a.emitter.Emit(events.FlowUpdate, FlowUpdate{ID: f.Id.String(), Type: "sse-end"})
}
