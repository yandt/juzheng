// Package appinfo 提供应用与系统元信息（L1 系统层）。
//
// 版本号规则（语义化版本，三段式）：
//   - 主版本（Major）：整个系统颠覆式改变才递增
//   - 次版本（Minor）：功能增加或重大改变时递增（小版本归零）
//   - 修订版（Patch）：小范围改动/bugfix 时递增
//
// 不依赖 Wails。替换实现（如改成从 ldflags 注入版本）只需改本包，不影响上层 service。
package appinfo

import (
	"os"
	"runtime"
	"strings"
)

// 版本号组件（主.次.修，三段式，每次编译后递增）。
// 约定：小范围改动/bugfix → Patch +1；功能增加/重大改变 → Minor +1 且 Patch 归零；
// 系统颠覆式改变 → Major +1 且 Minor/Patch 归零。
const (
	VersionMajor = "0" // 系统颠覆式改变（未到 1.0 正式版）
	VersionMinor = "2" // 功能增加/重大改变（系统分层重构 + 模块化）
	VersionPatch = "74" // Windows 启动预检:启用 TUN 但缺 wintun.dll 时给可操作提示(下载链接+放置路径),不擅自打包 DLL(preflight_windows/other)。含 73:helper UAC 自提权安装:helper.exe 兼作安装器(--install/--uninstall),app 经 PowerShell RunAs 提权拉起;传当前用户 SID 收紧命名管道 ACL(EnvAllowedSID)。含 72:窗口/图标/托盘修复:page-header 右侧为三按钮让位(设置键可点);Logo 区高度对齐 page-header;exe 嵌图标+manifest(rsrc syso);托盘图标改可见着色(灰/绿,tray_darwin/other)。含 71:自绘三按钮(最小化/最大化禁用/关闭)移入界面右上角;最小化走任务栏(hideOnMinimise 平台相关),关闭隐藏到托盘。含 70:Frameless 无边框;69:Windows 移植地基:隔离 main.go cgo(dock_darwin/dock_other);iproto/helper 命名管道传输(go-winio);helper 可作 Windows 服务(svc.Run);sysproxy/helperclient/mitmcore证书/paths 拆 _darwin/_windows;Windows 侧实现就位(注册表代理/SCM 服务/certutil)。四路交叉编译通过,macOS 零回归。含 68:SSE 合并/重连归并/流量监控刷新
)

// Info 是应用与系统元信息（供前端「关于」/版本号显示）。
type Info struct {
	AppVersion string `json:"appVersion"` // 完整版本号 主.次.修
	GoVersion  string `json:"goVersion"`  // Go 工具链版本
	OS         string `json:"os"`         // 操作系统 darwin/windows/linux
	Arch       string `json:"arch"`       // CPU 架构
	AppName    string `json:"appName"`    // 应用名
	IsDev      bool   `json:"isDev"`      // 是否调试模式（wails3 dev）
}

// FullVersion 返回完整版本号：主.次.修。
func FullVersion() string {
	return VersionMajor + "." + VersionMinor + "." + VersionPatch
}

// Get 返回完整的应用与系统信息。
func Get() Info {
	return Info{
		AppVersion: FullVersion(),
		GoVersion:  runtime.Version(),
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		AppName:    "Juzheng",
		IsDev:      isDevBuild(),
	}
}

// isDevBuild 判断当前是否为开发构建（wails3 dev）。
func isDevBuild() bool {
	if os.Getenv("WAILS_DEV") != "" {
		return true
	}
	if exe, err := os.Executable(); err == nil {
		return strings.Contains(exe, ".dev.app") || strings.Contains(exe, ".dev")
	}
	return false
}
