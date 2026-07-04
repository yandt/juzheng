// SingBoxService：主 app 侧的 sing-box 控制器。
// 不直接跑 sing-box（TUN 需 root），而是通过 unix socket 控制 root helper 守护进程。
// 同时负责 helper 二进制的安装/卸载/状态查询。
//
// 这样主 app 本身不需要 sing-box 依赖（保持轻量），sing-box 跑在独立 helper 进程里。

package main

import (
	"bufio"
	"context"
	"embed"
	stdjson "encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// IPC 协议常量（与 helper/ipc.go 保持一致）。
const helperSocketPath = "/var/run/juzheng-helper.sock"

// helper 部署位置（launchd plist 指向这里）。
const (
	helperInstallPath = "/usr/local/libexec/juzheng-helper"
	helperPlistPath   = "/Library/LaunchDaemons/com.zhanghui.juzheng.helper.plist"
	helperLabel       = "com.zhanghui.juzheng.helper"
)

// SingBoxService 是 Wails Service，作为 helper 的控制端 + 生命周期管理。
type SingBoxService struct {
	app     *application.App
	options SingBoxServiceOptions

	mu      sync.Mutex
	running bool
}

// ServiceStartup 由 Wails 在应用启动时调用。
func (s *SingBoxService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}
	s.options = SingBoxServiceOptions{
		ConfigPath: filepath.Join(configDir, "juzheng", "singbox.json"),
	}
	// 落盘默认配置模板（首次运行）。
	if err := s.ensureConfig(); err != nil {
		log.Printf("初始化 sing-box 配置模板失败: %v", err)
	}
	return nil
}

// ServiceShutdown 由 Wails 在应用退出时调用。
func (s *SingBoxService) ServiceShutdown() error {
	// 不主动停止 helper（KeepAlive 会重启）；仅停止 sing-box 内核。
	return s.Stop()
}

// SetApp 注入 application.App。
func (s *SingBoxService) SetApp(app *application.App) { s.app = app }

// GetOptions 返回当前配置。
func (s *SingBoxService) GetOptions() SingBoxServiceOptions {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.options
}

// IsRunning 返回 sing-box 内核是否在运行（通过询问 helper）。
func (s *SingBoxService) IsRunning() bool {
	resp, err := s.helperCall(IPCRequest{Action: "status"})
	if err != nil {
		return false
	}
	s.mu.Lock()
	s.running = resp.Running
	r := s.running
	s.mu.Unlock()
	return r
}

// Start 让 helper 启动 sing-box 内核。
func (s *SingBoxService) Start() (string, error) {
	cfg, err := s.GetConfig()
	if err != nil {
		return "", fmt.Errorf("读取配置失败: %w", err)
	}
	resp, err := s.helperCall(IPCRequest{Action: "start", Config: cfg})
	if err != nil {
		return "", fmt.Errorf("连接 helper 失败（是否已安装并运行？）: %w", err)
	}
	if !resp.OK {
		return "", fmt.Errorf("%s", resp.Message)
	}
	s.mu.Lock()
	s.running = resp.Running
	s.mu.Unlock()
	if s.app != nil {
		s.app.Event.Emit("singbox:started", map[string]any{})
	}
	return "started", nil
}

// Stop 让 helper 停止 sing-box 内核。
func (s *SingBoxService) Stop() error {
	resp, err := s.helperCall(IPCRequest{Action: "stop"})
	if err != nil {
		// helper 不可达时也算停止。
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		return nil
	}
	s.mu.Lock()
	s.running = resp.Running
	s.mu.Unlock()
	if s.app != nil {
		s.app.Event.Emit("singbox:stopped", map[string]any{})
	}
	return nil
}

// GetConfig 读取配置文件内容。
func (s *SingBoxService) GetConfig() (string, error) {
	s.mu.Lock()
	cfgPath := s.options.ConfigPath
	s.mu.Unlock()
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SetConfig 写入配置文件。
func (s *SingBoxService) SetConfig(content string) error {
	s.mu.Lock()
	cfgPath := s.options.ConfigPath
	s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(cfgPath, []byte(content), 0o600)
}

// --- helper 生命周期管理 ---

// HelperStatus 反映 helper 守护进程的安装与运行状态。
type HelperStatus struct {
	Installed bool   `json:"installed"` // plist 是否已部署到 /Library/LaunchDaemons
	Running   bool   `json:"running"`   // helper 进程是否在运行
	Loaded    bool   `json:"loaded"`    // launchd 是否已加载
	PID       int    `json:"pid"`       // helper pid
	InstallCmd string `json:"installCmd"` // 安装命令（供复制）
}

// GetHelperStatus 查询 helper 状态。
func (s *SingBoxService) GetHelperStatus() HelperStatus {
	st := HelperStatus{}
	if _, err := os.Stat(helperPlistPath); err == nil {
		st.Installed = true
	}
	// launchctl print 检查是否已加载。
	if err := exec.Command("launchctl", "print", "system/"+helperLabel).Run(); err == nil {
		st.Loaded = true
	}
	// 通过 ping helper 确认进程存活。
	if resp, err := s.helperCall(IPCRequest{Action: "ping"}); err == nil {
		st.Running = true
		st.PID = resp.PID
	}
	st.InstallCmd = s.installCommand()
	return st
}

// installCommand 返回安装命令字符串（供前端展示/复制）。
// 用 osascript 提权执行 deploy + launchctl bootstrap。
func (s *SingBoxService) installCommand() string {
	return fmt.Sprintf(`osascript -e 'do shell script "<DEPLOY_SCRIPT>" with administrator privileges'`)
}

// InstallHelper 部署 helper 二进制 + plist，并 launchctl 加载。需提权。
// deployScript: 落盘 helper 二进制（从内嵌资源）和 plist，再 bootstrap。
func (s *SingBoxService) InstallHelper() (string, error) {
	// helper 二进制内嵌在主 app 里（go:embed），先落盘到临时位置，
	// 再用 osascript 提权把它 + plist 拷到系统目录并 bootstrap。
	helperBin, err := s.readEmbeddedHelper()
	if err != nil {
		return "", fmt.Errorf("读取内嵌 helper 失败: %w", err)
	}
	plistData, err := s.readEmbeddedPlist()
	if err != nil {
		return "", fmt.Errorf("读取内嵌 plist 失败: %w", err)
	}

	// 写到用户可写的临时位置（osascript 里再 sudo 拷走）。
	tmpBin := filepath.Join(os.TempDir(), "juzheng-helper")
	tmpPlist := filepath.Join(os.TempDir(), "com.zhanghui.juzheng.helper.plist")
	if err := os.WriteFile(tmpBin, helperBin, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(tmpPlist, plistData, 0o644); err != nil {
		return "", err
	}

	// 提权：拷贝 + 卸载旧的 + bootstrap 新的。
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

// UninstallHelper 卸载 helper。需提权。
func (s *SingBoxService) UninstallHelper() (string, error) {
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

// NewSingBoxService 构造函数。
func NewSingBoxService() *SingBoxService {
	return &SingBoxService{}
}

// ensureConfig 落盘默认配置模板。
func (s *SingBoxService) ensureConfig() error {
	if _, err := os.Stat(s.options.ConfigPath); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.options.ConfigPath), 0o755); err != nil {
		return err
	}
	tmpl, err := singboxTemplateFS.ReadFile("configs/singbox-template.json")
	if err != nil {
		return err
	}
	return os.WriteFile(s.options.ConfigPath, tmpl, 0o600)
}

// --- IPC 客户端 ---

// helperCall 向 helper 发送一个 IPC 请求并返回响应。
func (s *SingBoxService) helperCall(req IPCRequest) (*IPCResponse, error) {
	conn, err := net.DialTimeout("unix", helperSocketPath, 2*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))

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
	var resp IPCResponse
	if err := stdjson.Unmarshal([]byte(line), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// readEmbeddedHelper / readEmbeddedPlist 读取打包进 app 的 helper 资源。
// 这些资源在构建时通过 Taskfile 落盘到 helper-assets/ 再 go:embed。

//go:embed helper-assets/juzheng-helper
var embeddedHelper []byte

//go:embed helper-assets/com.zhanghui.juzheng.helper.plist
var embeddedPlist []byte

//go:embed configs/singbox-template.json
var singboxTemplateFS embed.FS

func (s *SingBoxService) readEmbeddedHelper() ([]byte, error) { return embeddedHelper, nil }
func (s *SingBoxService) readEmbeddedPlist() ([]byte, error) { return embeddedPlist, nil }
