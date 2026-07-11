//go:build darwin

package main

import (
	_ "embed"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// macOS 托盘图标：黑色剪影 Template，由系统按菜单栏明暗自适应上色（单色）。
// 用「形状」而非「颜色」区分状态，绕开 Wails(alpha2.115) Template 会单色化彩色图的限制：
//   - 运行中（active）：完整戴帽剪影（有帽翅）
//   - 未运行（idle） ：去掉帽翅的剪影
// 两态均走 SetTemplateIcon，isTemplateIcon 始终一致，无彩色被抹问题。
//
//go:embed build/systray/systray-mac-idle@2x.png
var trayIdleIcon []byte

//go:embed build/systray/systray-mac-active@2x.png
var trayActiveIcon []byte

// updateTrayIcon 按运行状态切换 macOS 托盘图标（均为 Template 单色，形状区分）。
func updateTrayIcon(tray *application.SystemTray, running bool) {
	if running {
		tray.SetTemplateIcon(trayActiveIcon)
	} else {
		tray.SetTemplateIcon(trayIdleIcon)
	}
}
