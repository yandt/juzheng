// profile.go — 系统 Profile 的读写 + 运行配置合成（Phase 2）。
//
// 系统 Profile（~/.juzheng/profile.json）= 系统脚手架（TUN/DNS/mixed-back/to-mitmproxy/
// MITM 规则/route 骨架/日志/experimental），全局一份，与具体订阅无关。
// 运行配置 = Merge(Profile, 活动订阅的代理源)：换订阅时系统设置不变，只换代理源。
package singboxcfg

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/zhanghui/juzheng/internal/paths"
	"github.com/zhanghui/juzheng/internal/runcfg"
)

// EnsureProfile 首次运行时派生系统 Profile。
// 迁移友好：优先从用户当前的 singbox.json（已有系统设置）拆出，没有才用内嵌模板，
// 避免老用户升级到 Phase 2 后系统设置被重置为模板默认。
func EnsureProfile() error {
	p, err := paths.ProfilePath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(p); err == nil {
		return nil // 已存在
	}
	// 来源优先级：现有 singbox.json → 内嵌模板。
	var raw string
	if cur, err := ReadSingboxConfig(); err == nil && cur != "" {
		raw = Sanitize(cur)
	} else {
		tmpl, err := TemplateContent()
		if err != nil {
			return err
		}
		raw = Sanitize(string(tmpl))
	}
	var full map[string]any
	if err := json.Unmarshal([]byte(raw), &full); err != nil {
		return fmt.Errorf("派生 Profile 解析失败: %w", err)
	}
	profile, _ := runcfg.Split(full)
	return writeJSON(p, profile)
}

// ReadProfile 读取系统 Profile。不存在时从模板派生一份返回（不落盘）。
func ReadProfile() (map[string]any, error) {
	p, err := paths.ProfilePath()
	if err != nil {
		return nil, err
	}
	data, err := readCapped(p)
	if err != nil {
		// 回退：从模板派生
		if tmpl, e := TemplateContent(); e == nil {
			var full map[string]any
			if json.Unmarshal([]byte(Sanitize(string(tmpl))), &full) == nil {
				profile, _ := runcfg.Split(full)
				return profile, nil
			}
		}
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("解析 profile.json 失败: %w", err)
	}
	return m, nil
}

// WriteProfile 原子写入系统 Profile。
func WriteProfile(profile map[string]any) error {
	p, err := paths.ProfilePath()
	if err != nil {
		return err
	}
	return writeJSON(p, profile)
}

// UpdateProfileFromFull 从一份完整配置里抽取系统部分，覆盖写入 Profile。
// 供前端保存配置时同步系统设置（TUN/DNS/MITM 等）到全局 Profile。
func UpdateProfileFromFull(fullJSON string) error {
	var full map[string]any
	if err := json.Unmarshal([]byte(fullJSON), &full); err != nil {
		return fmt.Errorf("解析配置失败: %w", err)
	}
	profile, _ := runcfg.Split(full)
	return WriteProfile(profile)
}

// ComposeRunning 用系统 Profile + 一份订阅（完整配置）的代理源，合成运行配置 JSON。
// 订阅里的系统部分被丢弃，一律用 Profile 覆盖 —— 这是"换订阅系统设置不变"的关键。
func ComposeRunning(subContent string) (string, error) {
	profile, err := ReadProfile()
	if err != nil {
		return "", err
	}
	var sub map[string]any
	if err := json.Unmarshal([]byte(subContent), &sub); err != nil {
		return "", fmt.Errorf("解析订阅失败: %w", err)
	}
	source := runcfg.SourceOf(sub)
	merged := runcfg.Merge(profile, source)
	out, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// 旧/新默认 mixed-back 端口。旧默认 7897 与 Clash Verge 默认 mixed 端口冲突，改为 9788。
const (
	oldDefaultMixedPort = 7897
	newDefaultMixedPort = 9788
)

// MigrateMixedBackPort 一次性迁移：把 profile.json / singbox.json 里 mixed-back inbound 的
// 旧默认端口 7897 改为 9788。仅当端口恰为旧默认时改（不动用户自定义端口）。
func MigrateMixedBackPort() {
	if p, err := ReadProfile(); err == nil {
		if migrateMixedPortInObj(p) {
			_ = WriteProfile(p)
		}
	}
	if s, err := ReadSingboxConfig(); err == nil && s != "" {
		var obj map[string]any
		if json.Unmarshal([]byte(s), &obj) == nil && migrateMixedPortInObj(obj) {
			if data, err := json.MarshalIndent(obj, "", "  "); err == nil {
				_ = WriteSingboxConfig(string(data))
			}
		}
	}
}

// migrateMixedPortInObj 把 obj.inbounds 里 mixed-back 的旧默认端口改为新默认，返回是否改动。
func migrateMixedPortInObj(obj map[string]any) bool {
	ins, ok := obj["inbounds"].([]any)
	if !ok {
		return false
	}
	changed := false
	for _, i := range ins {
		m, ok := i.(map[string]any)
		if !ok {
			continue
		}
		if m["type"] == "mixed" || m["tag"] == "mixed-back" {
			if p, ok := m["listen_port"].(float64); ok && int(p) == oldDefaultMixedPort {
				m["listen_port"] = newDefaultMixedPort
				changed = true
			}
		}
	}
	return changed
}

func writeJSON(path string, v map[string]any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化失败: %w", err)
	}
	return atomicWriteFile(path, data, 0o600)
}
