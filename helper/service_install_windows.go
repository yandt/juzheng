//go:build singbox && windows

// service_install_windows.go：helper 兼作自己的 Windows 服务安装器（在被 UAC 提权拉起时执行）。
//
// 主 app（普通权限）通过 UAC 提权运行 `juzheng-helper.exe --install --allow-sid <SID>`，
// 本进程（管理员）把自身复制到 %ProgramData%\Juzheng，注册为自启动服务（以 LocalSystem 运行，
// TUN 需 SYSTEM），并把 --allow-sid 作为服务参数写入——服务每次启动都据此收紧命名管道 ACL。

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// installDirWin 返回 helper.exe 部署目录 %ProgramData%\Juzheng。
func installDirWin() string {
	pd := os.Getenv("ProgramData")
	if pd == "" {
		pd = `C:\ProgramData`
	}
	return filepath.Join(pd, "Juzheng")
}

// installedExePath 返回服务指向的 helper.exe 最终路径。
func installedExePath() string { return filepath.Join(installDirWin(), "juzheng-helper.exe") }

// runInstall 部署自身并注册+启动服务。allowSID 非空时作为服务参数注入（收紧管道 ACL）。
func runInstall(allowSID string) error {
	dir := installDirWin()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建目录 %s: %w", dir, err)
	}
	dst := installedExePath()
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if !strings.EqualFold(self, dst) {
		if err := copyFile(self, dst); err != nil {
			return fmt.Errorf("复制 helper 到 %s: %w", dst, err)
		}
	}

	// 写入内嵌的 wintun.dll（与 helper 架构匹配）到 helper 同目录，供 sing-box 运行 TUN 时加载。
	// best-effort：失败不阻断安装，仅影响 TUN 模式，普通代理不受影响。
	if err := writeWintun(dir); err != nil {
		log.Printf("写入 wintun.dll 失败（TUN 模式将不可用）: %v", err)
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("连接服务管理器: %w", err)
	}
	defer m.Disconnect()

	// 已存在则先停止并删除，用新二进制/参数重建。
	if s, err := m.OpenService(serviceName); err == nil {
		_, _ = s.Control(svc.Stop)
		time.Sleep(500 * time.Millisecond)
		_ = s.Delete()
		s.Close()
		time.Sleep(500 * time.Millisecond)
	}

	var args []string
	if allowSID != "" {
		args = append(args, "--allow-sid", allowSID)
	}
	s, err := m.CreateService(serviceName, dst, mgr.Config{
		StartType:   mgr.StartAutomatic,
		DisplayName: "Juzheng Helper",
		Description: "Juzheng 特权守护进程（承载 sing-box 内核，TUN 需 SYSTEM）。",
	}, args...)
	if err != nil {
		return fmt.Errorf("创建服务: %w", err)
	}
	defer s.Close()

	if err := s.Start(); err != nil {
		return fmt.Errorf("启动服务: %w", err)
	}
	log.Printf("helper 服务已安装并启动（allowSID=%q）", allowSID)
	return nil
}

// runUninstall 停止并删除服务，清理已部署的 exe。
func runUninstall() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("连接服务管理器: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("服务未安装或无法打开: %w", err)
	}
	defer s.Close()

	_, _ = s.Control(svc.Stop)
	time.Sleep(500 * time.Millisecond)
	if err := s.Delete(); err != nil {
		return fmt.Errorf("删除服务: %w", err)
	}
	_ = os.Remove(installedExePath())
	_ = os.Remove(filepath.Join(installDirWin(), "wintun.dll"))
	log.Printf("helper 服务已卸载")
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
