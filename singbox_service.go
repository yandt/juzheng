// SingBoxService：sing-box 内核控制器 Wails service（L3 薄壳）。
//
// 职责（拆分后）：sing-box 内核启停 + 配置读写 + helper 生命周期管理。
// 订阅管理已拆到 SubscriptionService；系统代理已拆到 SysProxyService。
//
// 通过 EventEmitter 接口上抛 singbox:started/stopped 事件（不依赖 *application.App）。

package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/zhanghui/juzheng/internal/events"
	"github.com/zhanghui/juzheng/internal/helperclient"
	"github.com/zhanghui/juzheng/internal/singboxcfg"
)

// SingBoxService 是 sing-box 内核控制器 Wails service。
type SingBoxService struct {
	emitter events.EventEmitter // 事件发射器（构造注入，替代 SetApp）
	options SingBoxServiceOptions

	mu      sync.Mutex
	running bool
}

// ServiceStartup 由 Wails 在应用启动时调用。
func (s *SingBoxService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	// 首次运行初始化（schemes 目录 / 默认订阅 / meta.json / singbox.json）委托 L1。
	if err := singboxcfg.EnsureDefaults(); err != nil {
		log.Printf("初始化应用数据目录失败: %v", err)
	}
	return nil
}

// ServiceShutdown 由 Wails 在应用退出时调用。
func (s *SingBoxService) ServiceShutdown() error {
	return s.Stop()
}

// GetOptions 返回当前配置。
func (s *SingBoxService) GetOptions() SingBoxServiceOptions {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.options
}

// IsRunning 返回 sing-box 内核是否在运行（通过询问 helper）。
func (s *SingBoxService) IsRunning() bool {
	resp, err := helperclient.Call("status", "")
	if err != nil {
		return false
	}
	s.mu.Lock()
	s.running = resp.Running
	r := s.running
	s.mu.Unlock()
	return r
}

// Start 让 helper 启动 sing-box 内核。
// 读取配置文件后，在内存中转为 sing-box 兼容格式（移除 _comment 等非标准字段），再传给 helper。
func (s *SingBoxService) Start() (string, error) {
	cfg, err := singboxcfg.ReadSingboxConfig()
	if err != nil {
		return "", fmt.Errorf("读取配置失败: %w", err)
	}
	// 内存转换：清理 sing-box 不认的字段（_comment 等），不修改原配置文件。
	cfg = singboxcfg.Sanitize(cfg)
	resp, err := helperclient.Call("start", cfg)
	if err != nil {
		return "", fmt.Errorf("连接 helper 失败（是否已安装并运行？）: %w", err)
	}
	if !resp.OK {
		return "", fmt.Errorf("%s", resp.Message)
	}
	s.mu.Lock()
	s.running = resp.Running
	s.mu.Unlock()
	s.emitter.Emit(events.SingboxStarted, map[string]any{})
	return "started", nil
}

// Stop 让 helper 停止 sing-box 内核。
// helper 不可达时本地状态置为已停止，但仍返回错误让调用方感知（此前吞掉错误返回 nil，
// 导致前端误以为停止成功）。
func (s *SingBoxService) Stop() error {
	resp, err := helperclient.Call("stop", "")
	if err != nil {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		s.emitter.Emit(events.SingboxStopped, map[string]any{})
		return fmt.Errorf("停止 sing-box 失败（helper 不可达）: %w", err)
	}
	s.mu.Lock()
	s.running = resp.Running
	s.mu.Unlock()
	s.emitter.Emit(events.SingboxStopped, map[string]any{})
	return nil
}

// ReloadConfig 热重载运行中的内核：读当前配置 → 清洗 → 让 helper 用新配置重建 box 实例。
// 供路由规则 / 抓包域名 / DNS 等结构性改动保存后即时生效（sing-box 只在启动时加载配置，
// 节点切换/代理模式走 clash_api 可热改，但路由/DNS 不行，故走实例级重载）。
// 内核未运行时为 no-op（返回 nil）。不发 started/stopped 事件，避免托盘图标闪烁。
func (s *SingBoxService) ReloadConfig() error {
	s.mu.Lock()
	running := s.running
	s.mu.Unlock()
	if !running {
		return nil
	}
	cfg, err := singboxcfg.ReadSingboxConfig()
	if err != nil {
		return fmt.Errorf("读取配置失败: %w", err)
	}
	cfg = singboxcfg.Sanitize(cfg)
	resp, err := helperclient.Call("reload", cfg)
	if err != nil {
		return fmt.Errorf("连接 helper 失败: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("%s", resp.Message)
	}
	s.mu.Lock()
	s.running = resp.Running
	s.mu.Unlock()
	return nil
}

// GetConfig 读取配置文件内容。
func (s *SingBoxService) GetConfig() (string, error) {
	return singboxcfg.ReadSingboxConfig()
}

// SanitizeConfig 把给定配置转为最终供 sing-box 运行的形态：与 Start() 实际发给 helper 的
// 内容完全一致（移除 _comment / legacy 字段 / block-dns 特殊 outbound 转 action / geo 规则 / cache_file）。
// 供「源码」页展示"最终运行配置"，解析失败时原样返回。
func (s *SingBoxService) SanitizeConfig(content string) string {
	return singboxcfg.Sanitize(content)
}

// SetConfig 写入运行配置，并把其中的系统部分（TUN/DNS/MITM/route 骨架等）
// 同步到全局系统 Profile —— 这样切换到其它订阅时这些系统设置依然生效（Phase 2）。
func (s *SingBoxService) SetConfig(content string) error {
	if err := singboxcfg.WriteSingboxConfig(content); err != nil {
		return err
	}
	if err := singboxcfg.UpdateProfileFromFull(content); err != nil {
		log.Printf("同步系统 Profile 失败: %v", err)
	}
	return nil
}

// ===== helper 生命周期管理 =====

// HelperStatus 别名转发到 internal/helperclient（保持 Wails binding 兼容）。
type HelperStatus = helperclient.Status

func (s *SingBoxService) GetHelperStatus() HelperStatus {
	return helperclient.GetStatus()
}

func (s *SingBoxService) InstallHelper() (string, error) {
	return helperclient.Install()
}

func (s *SingBoxService) UninstallHelper() (string, error) {
	return helperclient.Uninstall()
}

// NewSingBoxService 构造函数（emitter 注入，消除 SetApp）。
func NewSingBoxService(emitter events.EventEmitter) *SingBoxService {
	if emitter == nil {
		emitter = events.NoopEmitter{}
	}
	return &SingBoxService{emitter: emitter}
}
