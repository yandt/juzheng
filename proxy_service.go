package main

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// MitmProxyService 是 Wails Service，封装 go-mitmproxy 代理。
// 公开方法会被 Wails 自动绑定到前端；通过 app.Event 推送捕获的流量。
type MitmProxyService struct {
	app     *application.App
	options ServiceOptions

	mu     sync.Mutex
	proxy  *proxy.Proxy
	running bool
	cancel  context.CancelFunc
}

// ServiceOptions 是前端可调的代理运行参数。
type ServiceOptions struct {
	Addr        string `json:"addr"`        // 监听地址，默认 :9080
	Upstream    string `json:"upstream"`    // 上游代理（链式到 Clash），可空
	SslInsecure bool   `json:"sslInsecure"` // 是否跳过上游 TLS 校验
	CaRootPath  string `json:"caRootPath"`  // CA 证书目录
}

// 默认配置。
const (
	defaultAddr     = ":9080"
	defaultUpstream = ""
)

// ServiceStartup 由 Wails 在应用启动时调用。
func (s *MitmProxyService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	// 初始化默认配置；CA 存到用户配置目录下 juzheng/ca
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}
	s.options = ServiceOptions{
		Addr:        defaultAddr,
		Upstream:    defaultUpstream,
		SslInsecure: true,
		CaRootPath:  filepath.Join(configDir, "juzheng", "ca"),
	}
	// 不自动启动代理，等前端调用 Start()，避免没人信任证书时空跑。
	return nil
}

// ServiceShutdown 由 Wails 在应用退出时调用，优雅关闭代理。
func (s *MitmProxyService) ServiceShutdown() error {
	if err := s.Stop(); err != nil {
		log.Printf("关闭代理失败: %v", err)
		return err
	}
	return nil
}

// SetApp 在 main.go 中注入 application.App，供发射事件用。
// （Wails v3 的 service 没有直接的 app 引用，手动注入最可靠。）
func (s *MitmProxyService) SetApp(app *application.App) {
	s.app = app
}

// GetOptions 返回当前配置，供前端显示。
func (s *MitmProxyService) GetOptions() ServiceOptions {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.options
}

// SetOptions 由前端调用更新配置（仅在代理停止时可改）。
func (s *MitmProxyService) SetOptions(opts ServiceOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return fmt.Errorf("代理运行中，请先停止")
	}
	if opts.Addr != "" {
		s.options.Addr = opts.Addr
	}
	s.options.Upstream = opts.Upstream
	s.options.SslInsecure = opts.SslInsecure
	if opts.CaRootPath != "" {
		s.options.CaRootPath = opts.CaRootPath
	}
	return nil
}

// IsRunning 返回代理是否在运行。
func (s *MitmProxyService) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Start 启动代理。返回监听地址。
func (s *MitmProxyService) Start() (string, error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return "", fmt.Errorf("代理已在运行")
	}
	opts := s.options
	s.mu.Unlock()

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

	p, err := proxy.NewProxy(proxyOpts)
	if err != nil {
		return "", fmt.Errorf("创建代理失败: %w", err)
	}
	p.AddAddon(&captureAddon{app: s.app})

	ctx, cancel := context.WithCancel(context.Background())

	s.mu.Lock()
	s.proxy = p
	s.running = true
	s.cancel = cancel
	addr := opts.Addr
	s.mu.Unlock()

	// 在独立 goroutine 里跑代理；Start() 阻塞，所以用 cancel 来停。
	go func() {
		runErr := serveProxy(ctx, p)
		s.mu.Lock()
		s.running = false
		s.proxy = nil
		s.cancel = nil
		s.mu.Unlock()
		if runErr != nil && ctx.Err() == nil {
			log.Printf("代理异常退出: %v", runErr)
		}
		if s.app != nil {
			s.app.Event.Emit("proxy:stopped", map[string]any{"addr": addr})
		}
	}()

	if s.app != nil {
		s.app.Event.Emit("proxy:started", map[string]any{"addr": addr, "upstream": opts.Upstream})
	}
	return addr, nil
}

// serveProxy 在 ctx 取消时优雅关闭代理。
func serveProxy(ctx context.Context, p *proxy.Proxy) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Start()
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		// Shutdown 会停止监听并关闭现有连接。
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return p.Shutdown(shutdownCtx)
	}
}

// Stop 停止代理。
func (s *MitmProxyService) Stop() error {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	return nil
}

// GetCACertPath 返回 CA 证书文件路径，供前端引导用户信任。
func (s *MitmProxyService) GetCACertPath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return filepath.Join(s.options.CaRootPath, "mitmproxy-ca-cert.cer")
}

// CertStatus 反映 CA 证书的就绪与信任状态。
type CertStatus struct {
	Exists       bool   `json:"exists"`       // 证书文件是否已生成
	Path         string `json:"path"`         // 证书文件路径
	Trusted      bool   `json:"trusted"`      // 是否已加入系统钥匙串并信任
	InstallCmd   string `json:"installCmd"`   // 信任命令（供用户复制）
	Fingerprint  string `json:"fingerprint"`  // SHA256 指纹（核对用）
}

// GetCertStatus 返回证书状态。代理首次 Start 后证书才会生成。
func (s *MitmProxyService) GetCertStatus() CertStatus {
	path := s.GetCACertPath()
	st := CertStatus{Path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		return st
	}
	st.Exists = true
	if cert, err := x509.ParseCertificate(data); err == nil {
		st.Fingerprint = fmt.Sprintf("%X", sha256Sum(cert.Raw))
	}
	st.InstallCmd = fmt.Sprintf(`sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain "%s"`, path)
	st.Trusted = isCertTrusted(path)
	return st
}

// InstallCert 通过 osascript 提权调用 security add-trusted-cert。
// 会弹出系统密码框，用户输入后完成信任。返回输出/错误信息。
func (s *MitmProxyService) InstallCert() (string, error) {
	path := s.GetCACertPath()
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("证书尚未生成，请先启动一次代理: %w", err)
	}
	script := fmt.Sprintf(`do shell script "security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain %s" with administrator privileges`, path)
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	return string(out), err
}

// sha256Sum 计算 SHA256 并返回字节（避免在多处置复逻辑）。
func sha256Sum(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// isCertTrusted 通过 security verify-cert 检查证书是否已被信任。
func isCertTrusted(path string) bool {
	err := exec.Command("security", "verify-cert", "-c", path).Run()
	return err == nil
}

// captureAddon 实现 proxy.Addon，把流量转成 CapturedFlow 并发射事件。
type captureAddon struct {
	proxy.BaseAddon
	app *application.App

	mu    sync.Mutex
	flows map[string]*CapturedFlow // id -> flow（在请求/响应/SSE 间传递）
}

func (a *captureAddon) ensureMap() {
	if a.flows == nil {
		a.flows = make(map[string]*CapturedFlow)
	}
}

// headerToMap 把 http.Header 转成 map（多值用逗号合并），便于前端处理。
func headerToMap(h http.Header) map[string]string {
	m := make(map[string]string, len(h))
	for k, vs := range h {
		// 合并多值
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
	// 解码 body（处理 gzip/br）
	if body, err := f.Request.DecodedBody(); err == nil {
		cf.ReqBody = string(body)
	} else {
		cf.ReqBody = string(f.Request.Body)
	}
	a.flows[cf.ID] = cf
	a.mu.Unlock()

	if a.app != nil {
		a.app.Event.Emit("flow:update", FlowUpdate{ID: cf.ID, Type: "request", Flow: cf})
	}
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

	if a.app != nil {
		a.app.Event.Emit("flow:update", FlowUpdate{ID: cf.ID, Type: "response", Flow: &clone})
	}
}

// --- SSE hooks ---

func (a *captureAddon) SSEStart(f *proxy.Flow) {
	a.mu.Lock()
	if cf, ok := a.flows[f.Id.String()]; ok {
		cf.IsSSE = true
	}
	a.mu.Unlock()
	if a.app != nil {
		a.app.Event.Emit("flow:update", FlowUpdate{ID: f.Id.String(), Type: "sse-start"})
	}
}

func (a *captureAddon) SSEMessage(f *proxy.Flow) {
	events := f.SSE.Events
	if len(events) == 0 {
		return
	}
	ev := events[len(events)-1]
	dto := SSEEventDTO{Event: ev.Event, ID: ev.ID, Data: string(ev.Data)}

	a.mu.Lock()
	if cf, ok := a.flows[f.Id.String()]; ok {
		cf.SSEEvents = append(cf.SSEEvents, dto)
	}
	a.mu.Unlock()

	if a.app != nil {
		a.app.Event.Emit("flow:update", FlowUpdate{ID: f.Id.String(), Type: "sse-message", SSEEvent: &dto})
	}
}

func (a *captureAddon) SSEEnd(f *proxy.Flow) {
	a.mu.Lock()
	var count int
	if cf, ok := a.flows[f.Id.String()]; ok {
		count = len(cf.SSEEvents)
		now := time.Now()
		cf.EndTime = &now
		cf.DurationMs = now.Sub(cf.StartTime).Milliseconds()
	}
	a.mu.Unlock()
	if a.app != nil {
		a.app.Event.Emit("flow:update", FlowUpdate{ID: f.Id.String(), Type: "sse-end"})
	}
	_ = count
}
