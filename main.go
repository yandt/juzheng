package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	wailsevents "github.com/wailsapp/wails/v3/pkg/events"
	"github.com/zhanghui/juzheng/internal/events"
)

// setDockVisible 运行时显隐 Dock 图标 —— 平台相关，实现见 dock_darwin.go / dock_other.go。
// macOS：Regular/Accessory 激活策略切换；其他平台：无 Dock 概念，空实现。

// 托盘图标与状态切换 updateTrayIcon 平台相关（见 tray_darwin.go / tray_other.go）：
// macOS 用黑色 Template 剪影（系统自适应深浅）；Windows/Linux 用着色图标（灰=idle/绿=active），
// 保证深/浅任务栏都清晰可见。

// Wails 用 Go 的 embed 包把前端文件嵌入二进制。
//
//go:embed all:frontend/dist
var assets embed.FS

// flow 事件的数据类型，前端据此获得强类型 API。
// 事件名引用 internal/events 常量（消除散落的字符串字面量）。
func init() {
	application.RegisterEvent[FlowUpdate](events.FlowUpdate)
	application.RegisterEvent[map[string]any](events.ProxyStarted)
	application.RegisterEvent[map[string]any](events.ProxyStopped)
	application.RegisterEvent[map[string]any](events.SingboxStarted)
	application.RegisterEvent[map[string]any](events.SingboxStopped)
	application.RegisterEvent[MonitorHit](events.MonitorHit)
}

func main() {
	// appEmitter 适配器：把 *application.App 适配成 events.EventEmitter 接口。
	// 这样 L1 模块只依赖接口，不依赖 Wails —— 落实"下层不调上层"。
	// 用占位 App 先构造 service（emitter 内部 lazy 捕获 app），app 创建后回填。
	emitter := &appEmitter{}

	mitmService := NewMitmProxyService(emitter)
	singboxService := NewSingBoxService(emitter)
	subscriptionService := NewSubscriptionService()
	sysProxyService := NewSysProxyService()
	appInfoService := NewAppService()

	// mainWindow 在下方创建后回填；单实例二次启动回调用它唤起已有窗口。
	var mainWindow *application.WebviewWindow
	// showMainWindow：显示主窗口（恢复 Dock 图标 + 取消最小化 + 显示聚焦）。
	// win 创建后赋真正实现；这里先占位，供 options 里的回调提前引用。
	showMainWindow := func() {}

	app := application.New(application.Options{
		Name:        "juzheng",
		Description: "网络流量监控器 — 实时监控并分析网络请求",
		// 单实例：同一时刻只允许一个 juzheng 运行。第二个实例启动即退出，
		// 并通知第一个实例唤起（显示 + 聚焦）主窗口，避免多开常驻进程。
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.zhanghui.juzheng",
			OnSecondInstanceLaunch: func(_ application.SecondInstanceData) {
				if mainWindow != nil {
					showMainWindow()
				}
			},
		},
		Services: []application.Service{
			application.NewService(mitmService),
			application.NewService(singboxService),
			application.NewService(subscriptionService),
			application.NewService(sysProxyService),
			application.NewService(appInfoService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		// Regular 策略：应用作为标准窗口应用，在 Dock 显示图标。
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyRegular,
			// 关闭最后一个窗口时不退出 app（服务继续运行，靠 Dock/托盘唤起）
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	// 回填 app 到 emitter（service 已构造完成，emitter 现在能转发事件到 app）。
	// 替代旧的 SetApp 后门 —— service 不再暴露 SetApp 给前端 binding。
	emitter.app = app

	// 主窗口：启动即显示（标准应用形态，进 Dock）。
	// 标题栏：沉浸式（无可见标题栏），但保留窗口装饰 ——
	// FullSizeContent 让内容顶满，HideTitle 去标题文字，AppearsTransparent 让标题栏透明。
	// 双击顶部透明标题栏区域仍可最大化（macOS 原生手势保留）。
	// 前端需在顶部留出可拖拽区域（CSS app-region:drag，见 AppSidebar/顶栏）。
	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Juzheng — 网络流量监控",
		Name:   "main",
		Width:  1100,
		Height: 720,
		// 固定窗口尺寸：禁止缩放/放大（红黄绿的最大化/拖拽边缘均不改变大小）。
		DisableResize: true,
		MinWidth:      1100,
		MinHeight:     720,
		MaxWidth:      1100,
		MaxHeight:     720,
		// 无边框：平台相关。Windows 下去掉系统默认标题栏（沉浸式，靠前端顶栏拖拽 +
		// 托盘控制窗口）；macOS 保持 false —— 用下方 MacWindow 的透明标题栏（仍显示红黄绿按钮）。
		Frameless: framelessWindow,
		// 调试：启动时打开 WebKit Inspector（F12 等效）。发布前改 false。
		OpenInspectorOnStartup: true,
		DevToolsEnabled:        true,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 38, // 透明标题栏区域高度（可拖拽 + 双击最大化）
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar: application.MacTitleBar{
				AppearsTransparent: true,
				HideTitle:          true,
				FullSizeContent:    true,
			},
		},
		URL: "/",
	})
	mainWindow = win // 回填给单实例二次启动回调

	// 显示主窗口：恢复 Dock 图标 → 取消最小化 → 显示并聚焦。
	showMainWindow = func() {
		setDockVisible(true)
		if win.IsMinimised() {
			win.UnMinimise()
		}
		win.Show()
		win.Focus()
	}

	// 拦截窗口关闭（红按钮）：只隐藏窗口 + 隐藏 Dock 图标，不退出 app
	// （服务/MITM/代理内核继续运行）。此时仅能从托盘菜单唤起窗口。
	win.RegisterHook(wailsevents.Common.WindowClosing, func(e *application.WindowEvent) {
		win.Hide()
		setDockVisible(false)
		e.Cancel()
	})

	// 拦截最小化（黄按钮）：macOS 隐藏窗口（靠 Dock/托盘唤起）；Windows/Linux 走系统默认
	// 最小化到任务栏（hideOnMinimise=false，不拦截）。平台差异见 window_darwin/other.go。
	win.RegisterHook(wailsevents.Common.WindowMinimise, func(e *application.WindowEvent) {
		if hideOnMinimise {
			win.Hide()
		}
	})

	// 系统托盘：保留菜单栏图标，关窗后用于唤起窗口、退出 app。
	// （主入口现在是 Dock，托盘作为辅助。）
	tray := app.SystemTray.New()
	tray.SetTooltip("Juzheng — 网络流量监控")
	// 初始为待机态（平台相关图标）。
	updateTrayIcon(tray, false)

	// 右键菜单。
	menu := app.NewMenu()
	menu.Add("显示窗口").OnClick(func(ctx *application.Context) {
		showMainWindow()
	})
	menu.Add("隐藏窗口").OnClick(func(ctx *application.Context) {
		win.Hide()
	})
	menu.AddSeparator()
	menu.Add("退出 Juzheng").OnClick(func(ctx *application.Context) {
		app.Quit()
	})
	tray.SetMenu(menu)

	// 托盘图标随服务状态切换：任一服务（proxy / singbox）在运行 → 激活态（黑剪影+绿点）；
	// 全部停止 → 回到待机态（纯黑 Template 剪影）。
	// 注意：SetTemplateIcon 调用后框架内部 isTemplateIcon 标志会被置 true 且不随 SetIcon 复位，
	// 任一服务运行 → 激活态，全部停止 → 待机态。图标与切换方式平台相关（见 tray_*.go）。
	proxyRunning := false
	singboxRunning := false
	updateTray := func() {
		updateTrayIcon(tray, proxyRunning || singboxRunning)
	}
	app.Event.On(events.ProxyStarted, func(*application.CustomEvent) { proxyRunning = true; updateTray() })
	app.Event.On(events.ProxyStopped, func(*application.CustomEvent) { proxyRunning = false; updateTray() })
	app.Event.On(events.SingboxStarted, func(*application.CustomEvent) { singboxRunning = true; updateTray() })
	app.Event.On(events.SingboxStopped, func(*application.CustomEvent) { singboxRunning = false; updateTray() })

	// 点托盘图标也切换显/隐（不锚定窗口位置，让窗口停在标准位置）。
	tray.OnClick(func() {
		if win.IsVisible() {
			win.Hide()
		} else {
			showMainWindow()
		}
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// appEmitter 把 *application.App 适配成 events.EventEmitter 接口。
// L1 模块（mitmcore/singboxservice）依赖此接口而非 *application.App，
// 实现"下层不调上层"的解耦。app 在 service 构造时可能还未创建，
// 用指针延迟回填（emitter.app = app）。
type appEmitter struct {
	app *application.App
}

func (e *appEmitter) Emit(name string, data any) bool {
	if e.app == nil {
		return false
	}
	return e.app.Event.Emit(name, data)
}
