//go:build singbox

// wintun_windows_amd64.go：内嵌 amd64 版 wintun.dll（TUN 虚拟网卡驱动，WireGuard 官方签名，
// 允许再分发）。helper 安装时写到 %ProgramData%\Juzheng，供 sing-box 运行 TUN 时加载，
// 免去用户手动下载。文件名 _windows_amd64 后缀已自动约束为 windows+amd64。

package main

import (
	_ "embed"
	"os"
	"path/filepath"
)

//go:embed wintun/amd64/wintun.dll
var wintunDLL []byte

// writeWintun 把内嵌的 wintun.dll（与本 helper 架构匹配）写入 dir。
func writeWintun(dir string) error {
	return os.WriteFile(filepath.Join(dir, "wintun.dll"), wintunDLL, 0o644)
}
