//go:build darwin

package main

import (
	_ "embed"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// macOS 托盘图标：黑色剪影，作为 Template 由系统按菜单栏深浅自适应上色。
// idle 用纯黑 Template（自适应）；active 用带绿点的非 Template 图（Template 会被强制单色，显不出绿）。
//
//go:embed build/systray/systray-idle@2x.png
var trayIdleIcon []byte

//go:embed build/systray/systray-active@2x.png
var trayActiveIcon []byte

// updateTrayIcon 按运行状态切换 macOS 托盘图标。
func updateTrayIcon(tray *application.SystemTray, running bool) {
	if running {
		tray.SetIcon(trayActiveIcon)
	} else {
		tray.SetTemplateIcon(trayIdleIcon)
	}
}
