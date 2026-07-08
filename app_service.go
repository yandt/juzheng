package main

import "github.com/zhanghui/juzheng/internal/appinfo"

// app_service.go 已迁移至 internal/appinfo（L1 系统层）。
//
// 本文件保留 Wails service 薄壳：
// - AppInfo 类型别名（Wails binding 生成依赖）
// - AppService struct + GetAppInfo 方法（Wails binding 生成依赖）
// 内部完全委托给 appinfo.Get()。

// AppInfo 是应用与系统信息（Wails binding 生成依赖此导出类型）。
// 通过别名暴露 appinfo.Info，保持 JSON 字段名不变（appVersion/goVersion/os/arch/appName/isDev）。
type AppInfo = appinfo.Info

// AppService 提供应用/系统元信息。作为 Wails Service 注册。
type AppService struct{}

// GetAppInfo 返回应用与系统信息（委托 appinfo.Get）。
func (s *AppService) GetAppInfo() AppInfo {
	return appinfo.Get()
}

func NewAppService() *AppService { return &AppService{} }
