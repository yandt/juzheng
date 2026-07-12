// Package paths 是 juzheng 应用的文件路径中枢（L0 基础设施层）。
//
// 集中管理应用数据目录下所有子路径，纯函数、无状态、无外部依赖。
// 任何模块需要读写应用数据文件，都应通过本包获取路径，避免散落的 os.UserConfigDir() 调用。
//
// 应用数据根目录遵循各平台规范（os.UserConfigDir + "Juzheng"）：
//   - macOS   ~/Library/Application Support/Juzheng
//   - Windows %AppData%\Juzheng（即 C:\Users\<user>\AppData\Roaming\Juzheng）
//   - Linux   $XDG_CONFIG_HOME/Juzheng（默认 ~/.config/Juzheng）
//
// 注意：本包只服务于用户态 app 进程。helper（root/SYSTEM）不依赖本包，其配置经 IPC 传入，
// 绝不能让 helper 走 UserConfigDir（那是特权账户的目录，会读错位置）。
package paths

import (
	"os"
	"path/filepath"
)

// dirName 是应用数据目录名（各平台配置根下的子目录）。
const dirName = "Juzheng"

// JuzhengDir 返回应用根目录（os.UserConfigDir()/Juzheng，不存在不创建，由调用方按需 MkdirAll）。
func JuzhengDir() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, dirName), nil
}

// LegacyJuzhengDir 返回旧版应用根目录 ~/.juzheng（v0.2.84 及更早），仅供一次性数据迁移使用。
func LegacyJuzhengDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".juzheng"), nil
}

// SchemesDir 多订阅目录 <数据目录>/schemes（物理目录名保留 schemes，避免老数据丢失）。
func SchemesDir() (string, error) {
	d, err := JuzhengDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "schemes"), nil
}

// SchemeFile 某个订阅的完整路径 <数据目录>/schemes/<name>.json。
func SchemeFile(name string) (string, error) {
	d, err := SchemesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, name+".json"), nil
}

// SchemeInfoFile 订阅的伙伴信息文件（机场用量/到期）<数据目录>/schemes/<name>.info.json。
func SchemeInfoFile(name string) (string, error) {
	d, err := SchemesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, name+".info.json"), nil
}

// MetaPath 元数据文件 <数据目录>/meta.json。
func MetaPath() (string, error) {
	d, err := JuzhengDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "meta.json"), nil
}

// MonitorPath 监控规则文件 <数据目录>/monitor.json（内容层监控/拦截/改写/报警规则，全局一份）。
func MonitorPath() (string, error) {
	d, err := JuzhengDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "monitor.json"), nil
}

// SingboxConfigPath 当前活动订阅的运行副本 <数据目录>/singbox.json（helper 读这个）。
func SingboxConfigPath() (string, error) {
	d, err := JuzhengDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "singbox.json"), nil
}

// ProfilePath 系统 Profile 文件 <数据目录>/profile.json（系统脚手架：TUN/DNS/MITM/route 骨架等，全局一份）。
func ProfilePath() (string, error) {
	d, err := JuzhengDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "profile.json"), nil
}

// CaDir MITM 的 CA 证书目录 <数据目录>/ca。
func CaDir() (string, error) {
	d, err := JuzhengDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "ca"), nil
}
