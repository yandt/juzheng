package main

import (
	"context"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/zhanghui/juzheng/internal/events"
	"github.com/zhanghui/juzheng/internal/mitmcore"
)

// proxy_service.go 已迁移至 internal/mitmcore（L1 系统层）。
//
// 本文件保留 MitmProxyService Wails service 薄壳：
// - 委托 mitmcore.Proxy 完成所有领域逻辑
// - SetApp 改为注入 EventEmitter 接口（替代直接传 *application.App）
// 阶段三将彻底移除 SetApp，改用构造函数注入。

// MitmProxyService 是 Wails Service，封装 go-mitmproxy 代理。
// 公开方法会被 Wails 自动绑定到前端。
type MitmProxyService struct {
	core *mitmcore.Proxy
}

// ServiceOptions 别名转发到 internal/mitmcore（保持 Wails binding 兼容）。
type ServiceOptions = mitmcore.Options

// CertStatus 别名转发到 internal/mitmcore（保持 Wails binding 兼容）。
type CertStatus = mitmcore.CertStatus

// ServiceStartup 由 Wails 在应用启动时调用。
func (s *MitmProxyService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	// core 已在 NewMitmProxyService 构造时创建；不自动启动代理，等前端调用 Start()。
	return nil
}

// ServiceShutdown 由 Wails 在应用退出时调用，优雅关闭代理。
func (s *MitmProxyService) ServiceShutdown() error {
	if err := s.core.Stop(); err != nil {
		log.Printf("关闭代理失败: %v", err)
		return err
	}
	return nil
}

// SetApp 已移除 —— 改用构造函数注入 EventEmitter（NewMitmProxyService(emitter)）。
// 这样 L1（mitmcore）只依赖接口，消除 SetApp 后门（前端不再能调 SetApp）。

// GetOptions 返回当前配置。
func (s *MitmProxyService) GetOptions() ServiceOptions {
	return s.core.Options()
}

// SetOptions 由前端调用更新配置（仅在代理停止时可改）。
func (s *MitmProxyService) SetOptions(opts ServiceOptions) error {
	return s.core.SetOptions(opts)
}

// IsRunning 返回代理是否在运行。
func (s *MitmProxyService) IsRunning() bool {
	return s.core.IsRunning()
}

// Start 启动代理，返回监听地址。
func (s *MitmProxyService) Start() (string, error) {
	return s.core.Start()
}

// Stop 停止代理。
func (s *MitmProxyService) Stop() error {
	return s.core.Stop()
}

// GetCACertPath 返回 CA 证书文件路径。
func (s *MitmProxyService) GetCACertPath() string {
	return s.core.CaCertPath()
}

// GetCertStatus 返回证书状态。
func (s *MitmProxyService) GetCertStatus() CertStatus {
	return s.core.GetCertStatus()
}

// InstallCert 通过 osascript 提权信任证书。
func (s *MitmProxyService) InstallCert() (string, error) {
	return s.core.InstallCert()
}

// NewMitmProxyService 构造函数（emitter 注入，消除 SetApp）。
func NewMitmProxyService(emitter events.EventEmitter) *MitmProxyService {
	return &MitmProxyService{core: mitmcore.New(emitter)}
}

// appEmitter 适配器已移至 main.go（main 负责 app → EventEmitter 适配）。
