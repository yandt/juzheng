//go:build !darwin

package main

import (
	_ "embed"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Windows/Linux 托盘图标：无 Template 概念，用着色图标保证深/浅任务栏都清晰可见。
// idle 中性灰(#A0A4AB)，active 品牌绿(#4ADE80)。由 gentray 从黑色剪影重着色生成。
//
//go:embed build/systray/systray-win-idle.png
var trayIdleIcon []byte

//go:embed build/systray/systray-win-active.png
var trayActiveIcon []byte

// updateTrayIcon 按运行状态切换托盘图标（两态都用 SetIcon，非 Template）。
func updateTrayIcon(tray *application.SystemTray, running bool) {
	if running {
		tray.SetIcon(trayActiveIcon)
	} else {
		tray.SetIcon(trayIdleIcon)
	}
}
