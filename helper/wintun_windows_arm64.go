//go:build singbox

// wintun_windows_arm64.go：内嵌 arm64 版 wintun.dll（见 amd64 版说明）。
// 文件名 _windows_arm64 后缀已自动约束为 windows+arm64。

package main

import (
	_ "embed"
	"os"
	"path/filepath"
)

//go:embed wintun/arm64/wintun.dll
var wintunDLL []byte

// writeWintun 把内嵌的 wintun.dll（与本 helper 架构匹配）写入 dir。
func writeWintun(dir string) error {
	return os.WriteFile(filepath.Join(dir, "wintun.dll"), wintunDLL, 0o644)
}
