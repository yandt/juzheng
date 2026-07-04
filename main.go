package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/icons"
)

// Wails 用 Go 的 embed 包把前端文件嵌入二进制。
//
//go:embed all:frontend/dist
var assets embed.FS

// flow 事件的数据类型，前端据此获得强类型 API。
func init() {
	application.RegisterEvent[FlowUpdate]("flow:update")
	application.RegisterEvent[map[string]any]("proxy:started")
	application.RegisterEvent[map[string]any]("proxy:stopped")
	application.RegisterEvent[map[string]any]("singbox:started")
	application.RegisterEvent[map[string]any]("singbox:stopped")
}

func main() {
	mitmService := &MitmProxyService{}
	singboxService := NewSingBoxService()

	app := application.New(application.Options{
		Name:        "juzheng",
		Description: "Claude Code 流量监控器 — 实时查看并分析 CC 发往服务器的请求",
		Services: []application.Service{
			application.NewService(mitmService),
			application.NewService(singboxService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		// Accessory 策略：应用不在 Dock 显示，只在菜单栏驻留。
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
	})

	// 注入 app 引用，供 service 发射事件用。
	mitmService.SetApp(app)
	singboxService.SetApp(app)

	// 主窗口：启动隐藏，点托盘图标时弹出。
	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Juzheng — CC 流量监控",
		Name:   "main",
		Width:  1100,
		Height: 720,
		Hidden: true, // 启动不显示，由托盘触发
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		URL: "/",
	})

	// 拦截窗口关闭：只隐藏，不销毁，保证常驻。
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		win.Hide()
		e.Cancel()
	})

	// 系统托盘：菜单栏常驻图标 + 右键菜单。
	tray := app.SystemTray.New()
	tray.SetTooltip("Juzheng — CC 流量监控")
	tray.SetTemplateIcon(icons.SystrayMacTemplate) // macOS 模板图标，自动适配深/浅色

	// 右键菜单。
	menu := app.NewMenu()
	menu.Add("显示窗口").OnClick(func(ctx *application.Context) {
		win.Show()
		win.Focus()
	})
	menu.Add("隐藏窗口").OnClick(func(ctx *application.Context) {
		win.Hide()
	})
	menu.AddSeparator()
	menu.Add("退出 Juzheng").OnClick(func(ctx *application.Context) {
		app.Quit()
	})
	tray.SetMenu(menu)

	// 点托盘图标自动显/隐窗口（Wails 内置，自动定位到图标附近）。
	tray.AttachWindow(win).WindowOffset(5)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
