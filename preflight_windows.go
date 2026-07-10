//go:build windows

// preflight_windows.go：启动 sing-box 前的 Windows 侧预检。
//
// TUN（虚拟网卡）在 Windows 依赖 wintun.dll（WireGuard 官方发布的闭源驱动，不随本项目分发）。
// sing-box 运行时按 DLL 搜索路径加载它（helper.exe 所在目录 / System32 / PATH）。缺失时给出
// 可操作提示，避免用户只看到内核启动的底层报错。

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// preflightStart 在把配置发给 helper 前检查：若启用 TUN 但找不到 wintun.dll，返回可操作错误。
func preflightStart(cfg string) error {
	if !configUsesTun(cfg) {
		return nil
	}
	if wintunAvailable() {
		return nil
	}
	dir := filepath.Join(programDataDir(), "Juzheng")
	return fmt.Errorf(
		"已启用 TUN（虚拟网卡）但缺少 wintun.dll。请从 https://www.wintun.net 下载与本机架构匹配"+
			"（amd64）的 wintun.dll，放到 %s\\wintun.dll（需管理员），或放入 System32，然后重试。",
		dir)
}

func programDataDir() string {
	if pd := os.Getenv("ProgramData"); pd != "" {
		return pd
	}
	return `C:\ProgramData`
}

// configUsesTun 判断配置是否含 TUN 入站。
func configUsesTun(cfg string) bool {
	var doc struct {
		Inbounds []struct {
			Type string `json:"type"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal([]byte(cfg), &doc); err != nil {
		return strings.Contains(cfg, `"tun"`) // 解析失败兜底：宽松匹配
	}
	for _, in := range doc.Inbounds {
		if strings.EqualFold(in.Type, "tun") {
			return true
		}
	}
	return false
}

// wintunAvailable 在 DLL 搜索路径（helper 目录 / System32 / PATH）中查找 wintun.dll。
func wintunAvailable() bool {
	dirs := []string{filepath.Join(programDataDir(), "Juzheng")}
	if w := os.Getenv("WINDIR"); w != "" {
		dirs = append(dirs, filepath.Join(w, "System32"))
	}
	dirs = append(dirs, strings.Split(os.Getenv("PATH"), ";")...)
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(d, "wintun.dll")); err == nil {
			return true
		}
	}
	return false
}
