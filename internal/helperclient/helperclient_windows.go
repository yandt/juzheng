//go:build windows

// helperclient_windows.go：Windows 下 helper 的部署/卸载/状态。
//
// 安装/卸载需管理员权限：主 app（普通权限）把内嵌的 helper.exe 释放到临时目录，
// 再经 PowerShell `Start-Process -Verb RunAs`（弹 UAC）以管理员运行
// `helper.exe --install --allow-sid <当前用户SID>`，由 helper 自身完成部署+注册服务
// （见 helper/service_install_windows.go）。--allow-sid 让服务把命名管道 ACL 收紧到本用户。

package helperclient

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/zhanghui/juzheng/internal/iproto"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

const serviceName = "juzheng-helper"

// 内嵌 helper 二进制（构建前由 Taskfile 交叉编译到 helper-assets/juzheng-helper.exe）。
//
//go:embed helper-assets/juzheng-helper.exe
var embeddedHelper []byte

// installDir 返回 helper.exe 部署目录 %ProgramData%\Juzheng。
func installDir() string {
	pd := os.Getenv("ProgramData")
	if pd == "" {
		pd = `C:\ProgramData`
	}
	return filepath.Join(pd, "Juzheng")
}

func installExePath() string { return filepath.Join(installDir(), "juzheng-helper.exe") }

// currentUserSID 返回当前进程用户的 SID 字符串（供收紧命名管道 ACL）。
func currentUserSID() (string, error) {
	tok := windows.GetCurrentProcessToken()
	u, err := tok.GetTokenUser()
	if err != nil {
		return "", err
	}
	return u.User.Sid.String(), nil
}

// GetStatus 查询 helper 服务的安装/运行状态。
func GetStatus() Status {
	st := Status{InstallCmd: InstallCommand()}
	if m, err := mgr.Connect(); err == nil {
		defer m.Disconnect()
		if s, err := m.OpenService(serviceName); err == nil {
			st.Installed = true
			st.Loaded = true
			s.Close()
		}
	}
	if resp, err := Call(iproto.ActionPing, ""); err == nil {
		st.Running = true
		st.PID = resp.PID
	}
	return st
}

// InstallCommand 返回等效安装命令（供前端展示/复制）。
func InstallCommand() string {
	return fmt.Sprintf(`juzheng-helper.exe --install  (以管理员运行；注册并启动服务 %s)`, serviceName)
}

// Install 释放 helper.exe 到临时目录，经 UAC 提权运行其 --install 完成部署+注册服务。
func Install() (string, error) {
	sid, err := currentUserSID()
	if err != nil {
		sid = "" // 拿不到 SID 就回退（helper 侧管道 ACL 回退 Authenticated Users）
	}
	tmp := filepath.Join(os.TempDir(), "juzheng-helper-install.exe")
	if err := os.WriteFile(tmp, embeddedHelper, 0o755); err != nil {
		return "", fmt.Errorf("释放临时 helper 失败: %w", err)
	}

	argList := "@('--install'"
	if sid != "" {
		argList += ",'--allow-sid','" + sid + "'"
	}
	argList += ")"
	if out, err := runElevated(tmp, argList); err != nil {
		return out, fmt.Errorf("提权安装失败（可能取消 UAC）: %w", err)
	}

	if waitRunning(6 * time.Second) {
		return "helper 服务已安装并启动", nil
	}
	return "", fmt.Errorf("安装命令已执行，但未检测到 helper 运行；请重试或在 services.msc 查看服务 %s", serviceName)
}

// Uninstall 经 UAC 提权运行 helper --uninstall 停止并删除服务。
func Uninstall() (string, error) {
	exe := installExePath()
	if _, err := os.Stat(exe); err != nil {
		// 已部署 exe 不在，回退用临时释放的 exe 执行卸载（只需操作 SCM）。
		exe = filepath.Join(os.TempDir(), "juzheng-helper-install.exe")
		if _, err2 := os.Stat(exe); err2 != nil {
			if werr := os.WriteFile(exe, embeddedHelper, 0o755); werr != nil {
				return "", fmt.Errorf("释放临时 helper 失败: %w", werr)
			}
		}
	}
	if out, err := runElevated(exe, "@('--uninstall')"); err != nil {
		return out, fmt.Errorf("提权卸载失败（可能取消 UAC）: %w", err)
	}
	return "helper 服务已卸载", nil
}

// runElevated 用 PowerShell Start-Process -Verb RunAs 提权运行 exe（弹 UAC），-Wait 阻塞至结束。
// argList 为 PowerShell 数组字面量，如 "@('--install','--allow-sid','S-...')"。
func runElevated(exe, argList string) (string, error) {
	ps := fmt.Sprintf(
		`Start-Process -FilePath '%s' -Verb RunAs -Wait -WindowStyle Hidden -ArgumentList %s`,
		strings.ReplaceAll(exe, "'", "''"), argList)
	out, err := exec.Command("powershell", "-NoProfile", "-Command", ps).CombinedOutput()
	return string(out), err
}

// waitRunning 在超时内轮询 ping，确认 helper 服务已起并可通信。
func waitRunning(d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if resp, err := Call(iproto.ActionPing, ""); err == nil && resp.OK {
			return true
		}
		time.Sleep(400 * time.Millisecond)
	}
	return false
}
