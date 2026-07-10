//go:build windows

// sysproxy_windows.go：Windows 系统代理实现 —— 写用户级注册表 Internet Settings，
// 再经 WinINet InternetSetOption 通知刷新（无需重启浏览器/应用即可生效）。
//
// 说明：这是 per-user（HKCU）设置，无需管理员权限。CIDR 形式的 bypass（如 10.0.0.0/8）
// WinINet 不支持，暂原样写入并追加 <local>（TODO：CIDR → 通配符转换）。

package sysproxy

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

// WinINet 刷新选项常量。
const (
	internetOptionSettingsChanged = 39
	internetOptionRefresh         = 37
)

// Get 读取当前系统代理状态。
func Get() State {
	st := DefaultState()
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.QUERY_VALUE)
	if err != nil {
		return st
	}
	defer k.Close()

	if v, _, err := k.GetIntegerValue("ProxyEnable"); err == nil {
		st.Enabled = v != 0
	}
	if s, _, err := k.GetStringValue("ProxyServer"); err == nil && s != "" {
		parseProxyServer(s, &st)
	}
	if s, _, err := k.GetStringValue("ProxyOverride"); err == nil && s != "" {
		var bps []string
		for _, p := range strings.Split(s, ";") {
			p = strings.TrimSpace(p)
			if p != "" && p != "<local>" {
				bps = append(bps, p)
			}
		}
		if len(bps) > 0 {
			st.Bypass = bps
		}
	}
	return st
}

// parseProxyServer 解析 ProxyServer 字段（"host:port" 或 "http=h:p;https=h:p;socks=h:p"）。
func parseProxyServer(s string, st *State) {
	setHostPort := func(hp string) {
		if i := strings.LastIndex(hp, ":"); i >= 0 {
			st.Server = hp[:i]
			if p, err := strconv.Atoi(hp[i+1:]); err == nil {
				st.Port = p
			}
		} else {
			st.Server = hp
		}
	}
	if !strings.Contains(s, "=") {
		st.EnableHTTP, st.EnableHTTPS, st.EnableSOCKS = true, true, true
		setHostPort(s)
		return
	}
	st.EnableHTTP, st.EnableHTTPS, st.EnableSOCKS = false, false, false
	for _, seg := range strings.Split(s, ";") {
		seg = strings.TrimSpace(seg)
		proto, hp, ok := strings.Cut(seg, "=")
		if !ok {
			continue
		}
		switch strings.ToLower(proto) {
		case "http":
			st.EnableHTTP = true
		case "https":
			st.EnableHTTPS = true
		case "socks":
			st.EnableSOCKS = true
		}
		setHostPort(hp)
	}
}

// Set 启用或关闭系统代理，并刷新 WinINet。
func Set(state State) error {
	if state.Port <= 0 {
		state.Port = DefaultMixedPort
	}
	if state.Server == "" {
		state.Server = "127.0.0.1"
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打开 Internet Settings 注册表失败: %w", err)
	}
	defer k.Close()

	if !state.Enabled {
		if err := k.SetDWordValue("ProxyEnable", 0); err != nil {
			return fmt.Errorf("关闭系统代理失败: %w", err)
		}
		return refreshWinINet()
	}

	// 组装 ProxyServer：仅包含启用的协议。全启用且需要 SOCKS 时逐协议写，兼容性最好。
	hp := fmt.Sprintf("%s:%d", state.Server, state.Port)
	var segs []string
	if state.EnableHTTP {
		segs = append(segs, "http="+hp)
	}
	if state.EnableHTTPS {
		segs = append(segs, "https="+hp)
	}
	if state.EnableSOCKS {
		segs = append(segs, "socks="+hp)
	}
	proxyServer := hp
	if len(segs) > 0 {
		proxyServer = strings.Join(segs, ";")
	}

	// bypass：追加 <local> 让本地地址直连（Windows 惯例）。
	var overrides []string
	for _, d := range state.Bypass {
		if d = strings.TrimSpace(d); d != "" {
			overrides = append(overrides, d)
		}
	}
	overrides = append(overrides, "<local>")

	if err := k.SetStringValue("ProxyServer", proxyServer); err != nil {
		return fmt.Errorf("写 ProxyServer 失败: %w", err)
	}
	if err := k.SetStringValue("ProxyOverride", strings.Join(overrides, ";")); err != nil {
		return fmt.Errorf("写 ProxyOverride 失败: %w", err)
	}
	if err := k.SetDWordValue("ProxyEnable", 1); err != nil {
		return fmt.Errorf("启用系统代理失败: %w", err)
	}
	return refreshWinINet()
}

// refreshWinINet 通知 WinINet 配置已变更并刷新，使新代理设置立即对所有 WinINet 客户端生效。
func refreshWinINet() error {
	wininet := windows.NewLazySystemDLL("wininet.dll")
	proc := wininet.NewProc("InternetSetOptionW")
	proc.Call(0, internetOptionSettingsChanged, 0, 0)
	proc.Call(0, internetOptionRefresh, 0, 0)
	return nil
}
