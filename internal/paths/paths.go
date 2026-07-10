// Package paths 是 juzheng 应用的文件路径中枢（L0 基础设施层）。
//
// 集中管理 ~/.juzheng 下所有子路径，纯函数、无状态、无外部依赖。
// 任何模块需要读写应用数据文件，都应通过本包获取路径，避免散落的 os.UserConfigDir() 调用。
package paths

import (
	"os"
	"path/filepath"
)

// JuzhengDir 返回应用根目录 ~/.juzheng（不存在不创建，由调用方按需 MkdirAll）。
func JuzhengDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".juzheng"), nil
}

// SchemesDir 多订阅目录 ~/.juzheng/schemes（物理目录名保留 schemes，避免老数据丢失）。
func SchemesDir() (string, error) {
	d, err := JuzhengDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "schemes"), nil
}

// SchemeFile 某个订阅的完整路径 ~/.juzheng/schemes/<name>.json。
func SchemeFile(name string) (string, error) {
	d, err := SchemesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, name+".json"), nil
}

// SchemeInfoFile 订阅的伙伴信息文件（机场用量/到期）~/.juzheng/schemes/<name>.info.json。
func SchemeInfoFile(name string) (string, error) {
	d, err := SchemesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, name+".info.json"), nil
}

// MetaPath 元数据文件 ~/.juzheng/meta.json。
func MetaPath() (string, error) {
	d, err := JuzhengDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "meta.json"), nil
}

// MonitorPath 监控规则文件 ~/.juzheng/monitor.json（内容层监控/拦截/改写/报警规则，全局一份）。
func MonitorPath() (string, error) {
	d, err := JuzhengDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "monitor.json"), nil
}

// SingboxConfigPath 当前活动订阅的运行副本 ~/.juzheng/singbox.json（helper 读这个）。
func SingboxConfigPath() (string, error) {
	d, err := JuzhengDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "singbox.json"), nil
}

// ProfilePath 系统 Profile 文件 ~/.juzheng/profile.json（系统脚手架：TUN/DNS/MITM/route 骨架等，全局一份）。
func ProfilePath() (string, error) {
	d, err := JuzhengDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "profile.json"), nil
}

// CaDir MITM 的 CA 证书目录 ~/.juzheng/ca。
func CaDir() (string, error) {
	d, err := JuzhengDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "ca"), nil
}
