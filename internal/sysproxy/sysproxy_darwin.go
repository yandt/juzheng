//go:build darwin

// sysproxy_darwin.go：macOS 系统代理实现 —— networksetup（按网络服务设置）+ scutil（取活动服务）。

package sysproxy

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

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

// activeNetworkService 返回当前主(活动)网络服务名 —— macOS 系统代理是按网络服务设置的，
// 应操作"正在联网的那个服务"（scutil 主接口 → networksetup 服务名映射）。失败回退第一个服务。
func activeNetworkService() string {
	dev := ""
	if out, err := exec.Command("sh", "-c", `echo 'show State:/Network/Global/IPv4' | scutil | awk -F': ' '/PrimaryInterface/ {print $2}'`).Output(); err == nil {
		dev = strings.TrimSpace(string(out))
	}
	if dev != "" {
		if orderOut, err := exec.Command("networksetup", "-listnetworkserviceorder").Output(); err == nil {
			lines := strings.Split(string(orderOut), "\n")
			for i, line := range lines {
				// 形如 "(Hardware Port: Wi-Fi, Device: en0)"，其上一行为 "(2) Wi-Fi"
				if strings.Contains(line, "Device: "+dev+")") && i > 0 {
					prev := strings.TrimSpace(lines[i-1])
					if idx := strings.Index(prev, ") "); idx >= 0 {
						return strings.TrimSpace(prev[idx+2:])
					}
				}
			}
		}
	}
	if services, err := listNetworkServices(); err == nil && len(services) > 0 {
		return services[0]
	}
	return ""
}

// Get 查询系统代理当前状态（读当前活动网络服务的 web proxy）。
func Get() State {
	st := DefaultState()
	active := activeNetworkService()
	if active == "" {
		return st
	}
	services := []string{active}
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
	// 目标服务：启用只设当前活动服务（= macOS 网络设置里正在联网的那个，避免污染其它服务）；
	// 禁用则清所有服务，彻底清理任何残留（含旧版本/切换网络前设过的）。
	targets := services
	if state.Enabled {
		if active := activeNetworkService(); active != "" {
			targets = []string{active}
		}
	}
	var cmds []string
	for _, svc := range targets {
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
		for _, svc := range targets {
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

	// 优先直接以当前用户执行：admin 用户改 networksetup 代理无需提权（与 Clash Verge 一致，不弹密码框）。
	if out, err := exec.Command("/bin/bash", tmpScript).CombinedOutput(); err == nil {
		return nil
	} else {
		log.Printf("直接设置系统代理失败，回退提权: %v; 输出: %s", err, string(out))
	}
	// 回退：非管理员用户 networksetup 需提权，用 osascript 弹密码框。
	script := fmt.Sprintf(`do shell script "/bin/bash %s" with administrator privileges`, tmpScript)
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("设置系统代理失败（可能取消密码）: %w; 输出: %s", err, string(out))
	}
	return nil
}
