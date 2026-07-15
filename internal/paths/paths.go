// Package paths 是 juzheng 应用的文件路径中枢（L0 基础设施层）。
//
// 集中管理应用数据目录下所有子路径，纯函数、无状态、无外部依赖。
// 任何模块需要读写应用数据文件，都应通过本包获取路径，避免散落的 os.UserHomeDir() 调用。
//
// 应用数据根目录为 ~/.juzheng，全平台统一（Windows 即 %USERPROFILE%\.juzheng）：
// 点目录是开发/网络工具的成熟惯例（~/.ssh、~/.aws、~/.docker、~/.kube），
// 且路径全平台一致，排查问题时不必记忆三个平台三个位置。
//
// 历史：v0.2.85~v0.2.91 曾改用 os.UserConfigDir()/Juzheng（mac ~/Library/Application Support 等），
// 但该"规范化"收益甚微，反而引入一次性迁移逻辑；其守卫把"新目录已存在"等同于"已迁移"，
// 被更早期版本遗留的同名残留目录骗过，导致真实配置静默读不到。v0.2.92 起改回并删除迁移代码。
//
// 注意：本包只服务于用户态 app 进程。helper（root/SYSTEM）不依赖本包，其配置经 IPC 传入，
// 绝不能让 helper 走本包（那会解析到特权账户的 home，读错位置）。
package paths

import (
	"os"
	"path/filepath"
)

// dirName 是应用数据目录名（用户 home 下的点目录）。
const dirName = ".juzheng"

// JuzhengDir 返回应用根目录（~/.juzheng，不存在不创建，由调用方按需 MkdirAll）。
func JuzhengDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, dirName), nil
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
