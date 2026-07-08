// Package helperclient 是 juzheng-helper 守护进程的客户端（L1 系统层）。
//
// 职责：
//   - 通过 unix socket 向 helper 发送 IPC 请求（启动/停止/查询 sing-box 内核）
//   - helper 二进制的部署/卸载/状态查询（osascript 提权 + launchctl）
//   - 内嵌 helper 二进制 + launchd plist 资源
//
// 依赖：L0 iproto（IPC 协议）。不依赖 Wails、不依赖事件系统。
// 上层 service（singboxservice）通过本包控制内核 + 管理 helper 生命周期。
package helperclient

import (
	"bufio"
	"bytes"
	_ "embed"
	stdjson "encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/zhanghui/juzheng/internal/iproto"
)

// helper 部署位置（launchd plist 指向这里）。
const (
	helperInstallPath = "/usr/local/libexec/juzheng-helper"
	helperPlistPath   = "/Library/LaunchDaemons/com.zhanghui.juzheng.helper.plist"
	helperLabel       = "com.zhanghui.juzheng.helper"
)

// 内嵌 helper 资源（构建前由 Taskfile 落盘到 helper-assets/）。
//
//go:embed helper-assets/juzheng-helper
var embeddedHelper []byte

//go:embed helper-assets/com.zhanghui.juzheng.helper.plist
var embeddedPlist []byte

// Status 反映 helper 守护进程的安装与运行状态。
type Status struct {
	Installed  bool   `json:"installed"`           // plist 是否已部署到 /Library/LaunchDaemons
	Running    bool   `json:"running"`             // helper 进程是否在运行
	Loaded     bool   `json:"loaded"`              // launchd 是否已加载
	PID        int    `json:"pid"`                 // helper pid
	InstallCmd string `json:"installCmd"`          // 安装命令（供复制）
}

// Call 向 helper 发送一个 IPC 请求并返回响应。
// action 见 iproto.Action* 常量；config 仅 start 时需要。
func Call(action, config string) (*iproto.IPCResponse, error) {
	conn, err := net.DialTimeout("unix", iproto.ResolveSocketPath(), 2*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	req := iproto.IPCRequest{Action: action, Config: config}
	data, err := stdjson.Marshal(req)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write(append(data, '\n')); err != nil {
		return nil, err
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return nil, err
	}
	var resp iproto.IPCResponse
	if err := stdjson.Unmarshal([]byte(line), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetStatus 查询 helper 安装与运行状态。
func GetStatus() Status {
	st := Status{}
	if _, err := os.Stat(helperPlistPath); err == nil {
		st.Installed = true
	}
	if err := exec.Command("launchctl", "print", "system/"+helperLabel).Run(); err == nil {
		st.Loaded = true
	}
	if resp, err := Call(iproto.ActionPing, ""); err == nil {
		st.Running = true
		st.PID = resp.PID
	}
	st.InstallCmd = InstallCommand()
	return st
}

// InstallCommand 返回安装命令字符串（供前端展示/复制）。
func InstallCommand() string {
	return fmt.Sprintf(`osascript -e 'do shell script "<DEPLOY_SCRIPT>" with administrator privileges'`)
}

// Install 部署 helper 二进制 + plist，并 launchctl 加载。需提权。
// 把当前用户 UID 注入 plist，helper 据此只接受本用户（+root）的 IPC 连接。
func Install() (string, error) {
	// 唯一临时目录：避免多进程/多次安装用固定文件名互相覆盖。
	tmpDir, err := os.MkdirTemp("", "juzheng-install-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)
	tmpBin := filepath.Join(tmpDir, "juzheng-helper")
	tmpPlist := filepath.Join(tmpDir, "com.zhanghui.juzheng.helper.plist")

	// 注入安装用户 UID（helper 的 UID 白名单）。
	plist := bytes.ReplaceAll(embeddedPlist, []byte("__ALLOWED_UID__"), []byte(strconv.Itoa(os.Getuid())))

	if err := os.WriteFile(tmpBin, embeddedHelper, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(tmpPlist, plist, 0o644); err != nil {
		return "", err
	}

	script := fmt.Sprintf(`
install -m 0755 -o root -g wheel "%s" "%s" &&
install -m 0644 -o root -g wheel "%s" "%s" &&
launchctl bootout system/%s 2>/dev/null; launchctl bootstrap system "%s" &&
rm -f "%s" "%s"
`, tmpBin, helperInstallPath, tmpPlist, helperPlistPath, helperLabel, helperPlistPath, tmpBin, tmpPlist)

	out, err := exec.Command("osascript", "-e",
		fmt.Sprintf(`do shell script %q with administrator privileges`, script)).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("安装失败（可能用户取消或密码错误）: %w", err)
	}
	log.Printf("helper 已安装并加载")
	return string(out), nil
}

// Uninstall 卸载 helper。需提权。
func Uninstall() (string, error) {
	script := fmt.Sprintf(`
launchctl bootout system/%s 2>/dev/null;
rm -f "%s" "%s"
`, helperLabel, helperPlistPath, helperInstallPath)
	out, err := exec.Command("osascript", "-e",
		fmt.Sprintf(`do shell script %q with administrator privileges`, script)).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("卸载失败: %w", err)
	}
	return string(out), nil
}
