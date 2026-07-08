package main

import "github.com/zhanghui/juzheng/internal/sysproxy"

// sysproxy_service.go：独立的系统代理 Wails service（L3 薄壳）。
//
// 从 SingBoxService 拆出，委托 internal/sysproxy（L1）。
// 前端通过 sysproxyservice binding 调用。

// SysProxyService 是系统代理管理 Wails service。
type SysProxyService struct{}

// SystemProxyState 别名转发到 internal/sysproxy（保持 Wails binding 兼容）。
type SystemProxyState = sysproxy.State

func (s *SysProxyService) GetSystemProxy() SystemProxyState {
	return sysproxy.Get()
}

func (s *SysProxyService) SetSystemProxy(state SystemProxyState) error {
	return sysproxy.Set(state)
}

func NewSysProxyService() *SysProxyService { return &SysProxyService{} }
