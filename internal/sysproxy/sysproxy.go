// Package sysproxy 管理 macOS 系统代理设置（L1 系统层）。
//
// 职责：
//   - 查询当前系统代理状态（networksetup -getwebproxy 等）
//   - 启用/关闭系统代理（HTTP/HTTPS/SOCKS，按网络服务遍历）
//   - 设置绕过域名列表
//   - networksetup 改全局设置需 root，用 osascript 提权
//
// 依赖：仅 stdlib。不依赖 Wails、不依赖事件系统。
package sysproxy

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// DefaultMixedPort 默认 mixed inbound 端口（与配置模板 mixed-back 一致）。
const DefaultMixedPort = 9788

// State 反映系统代理当前状态（也用作设置项）。
type State struct {
	Enabled     bool     `json:"enabled"`     // 是否已设为系统代理
	Server      string   `json:"server"`      // 代理服务器（如 127.0.0.1）
	Port        int      `json:"port"`        // 代理端口
	EnableHTTP  bool     `json:"enableHttp"`  // 启用 HTTP 代理
	EnableHTTPS bool     `json:"enableHttps"` // 启用 HTTPS 代理
	EnableSOCKS bool     `json:"enableSocks"` // 启用 SOCKS 代理
	Bypass      []string `json:"bypass"`      // 绕过代理的域名/IP 列表
}

// DefaultState 返回默认系统代理设置。
func DefaultState() State {
	return State{
		Server:      "127.0.0.1",
		Port:        DefaultMixedPort,
		EnableHTTP:  true,
		EnableHTTPS: true,
		EnableSOCKS: true,
		Bypass:      []string{"127.0.0.1", "192.168.0.0/16", "10.0.0.0/8", "172.16.0.0/12", "localhost", "*.local"},
	}
}

// listNetworkServices 列出所有网络服务名（过滤说明行和帮助文本）。
func listNetworkServices() ([]string, error) {
	out, err := exec.Command("networksetup", "-listallnetworkservices").Output()
	if err != nil {
		return nil, err
	}
	var services []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "An asterisk") {
			continue
		}
		if strings.HasPrefix(line, "networksetup") || strings.Contains(line, "<") {
			continue
		}
		services = append(services, line)
	}
	return services, nil
}

// Get 查询系统代理当前状态（读第一个网络服务的 web proxy）。
func Get() State {
	st := DefaultState()
	services, err := listNetworkServices()
	if err != nil || len(services) == 0 {
		return st
	}
	out, err := exec.Command("networksetup", "-getwebproxy", services[0]).Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Enabled: Yes") {
				st.Enabled = true
			} else if strings.HasPrefix(line, "Server:") {
				st.Server = strings.TrimSpace(strings.TrimPrefix(line, "Server:"))
			} else if strings.HasPrefix(line, "Port:") {
				fmt.Sscanf(strings.TrimSpace(strings.TrimPrefix(line, "Port:")), "%d", &st.Port)
			}
		}
	}
	if bpOut, err := exec.Command("networksetup", "-getproxybypassdomains", services[0]).Output(); err == nil {
		var bps []string
		for _, line := range strings.Split(string(bpOut), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.Contains(line, "There aren") {
				bps = append(bps, line)
			}
		}
		if len(bps) > 0 {
			st.Bypass = bps
		}
	}
	return st
}

// Set 按用户设置启用或关闭系统代理。
// State.Enabled=true：按启用的类型（HTTP/HTTPS/SOCKS）+ 端口 + bypass 设置。
// State.Enabled=false：关闭所有网络服务的代理。
func Set(state State) error {
	if state.Port <= 0 {
		state.Port = DefaultMixedPort
	}
	if state.Server == "" {
		state.Server = "127.0.0.1"
	}
	services, err := listNetworkServices()
	if err != nil {
		return fmt.Errorf("获取网络服务列表失败: %w", err)
	}
	var cmds []string
	for _, svc := range services {
		esc := strings.ReplaceAll(svc, "\"", "\\\"")
		if state.Enabled {
			if state.EnableHTTP {
				cmds = append(cmds, fmt.Sprintf(`networksetup -setwebproxy "%s" %s %d`, esc, state.Server, state.Port))
			} else {
				cmds = append(cmds, fmt.Sprintf(`networksetup -setwebproxystate "%s" off`, esc))
			}
			if state.EnableHTTPS {
				cmds = append(cmds, fmt.Sprintf(`networksetup -setsecurewebproxy "%s" %s %d`, esc, state.Server, state.Port))
			} else {
				cmds = append(cmds, fmt.Sprintf(`networksetup -setsecurewebproxystate "%s" off`, esc))
			}
			if state.EnableSOCKS {
				cmds = append(cmds, fmt.Sprintf(`networksetup -setsocksfirewallproxy "%s" %s %d`, esc, state.Server, state.Port))
			} else {
				cmds = append(cmds, fmt.Sprintf(`networksetup -setsocksfirewallproxystate "%s" off`, esc))
			}
		} else {
			cmds = append(cmds,
				fmt.Sprintf(`networksetup -setwebproxystate "%s" off`, esc),
				fmt.Sprintf(`networksetup -setsecurewebproxystate "%s" off`, esc),
				fmt.Sprintf(`networksetup -setsocksfirewallproxystate "%s" off`, esc),
			)
		}
	}
	if state.Enabled && len(state.Bypass) > 0 {
		// 每个 bypass 域名单独加双引号（防止 * / 等特殊字符被 shell 解析）
		var args []string
		for _, d := range state.Bypass {
			d = strings.TrimSpace(d)
			if d != "" {
				args = append(args, `"`+strings.ReplaceAll(d, `"`, `\"`)+`"`)
			}
		}
		bypassArgs := strings.Join(args, " ")
		for _, svc := range services {
			esc := strings.ReplaceAll(svc, "\"", "\\\"")
			if bypassArgs != "" {
				cmds = append(cmds, fmt.Sprintf(`networksetup -setproxybypassdomains "%s" %s`, esc, bypassArgs))
			}
		}
	}
	shellScript := "#!/bin/bash\nset -e\n" + strings.Join(cmds, "\n") + "\n"
	// 用唯一临时文件（O_EXCL）避免固定文件名被并发覆盖或被其他进程预置（TOCTOU）。
	f, err := os.CreateTemp("", "juzheng-sysproxy-*.sh")
	if err != nil {
		return fmt.Errorf("创建临时脚本失败: %w", err)
	}
	tmpScript := f.Name()
	defer os.Remove(tmpScript)
	if _, err := f.WriteString(shellScript); err != nil {
		f.Close()
		return fmt.Errorf("写入临时脚本失败: %w", err)
	}
	if err := f.Chmod(0o700); err != nil {
		f.Close()
		return fmt.Errorf("设置脚本权限失败: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("关闭临时脚本失败: %w", err)
	}

	script := fmt.Sprintf(`do shell script "/bin/bash %s" with administrator privileges`, tmpScript)
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("设置系统代理失败（可能取消密码）: %w; 输出: %s", err, string(out))
	}
	return nil
}
