// Package convert 是外部订阅 → sing-box 运行配置的转换系统（L1 系统层）。
//
// 职责：把各种来源格式归一化为可运行的 sing-box JSON。
//   - sing-box 原生 JSON：已完整，原样返回。
//   - Clash YAML / base64 订阅 / 分享链接：先转成 sing-box 代理 outbounds，
//     再合成进「脚手架模板」（保留 TUN/mixed-back/to-mitmproxy/MITM 等系统结构）。
//
// 设计：格式可插拔（Detect 探测 + 各 parser 独立）；系统脚手架与用户代理分离，
// Compose 负责合成。依赖：仅 stdlib + yaml。不依赖 Wails。
package convert

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Format 是探测出的输入格式。
type Format string

const (
	FormatSingbox Format = "singbox" // sing-box 原生 JSON（含 outbounds/inbounds）
	FormatClash   Format = "clash"   // Clash / Clash Meta YAML（含 proxies）
	FormatBase64  Format = "base64"  // base64 编码的分享链接订阅
	FormatLinks   Format = "links"   // 明文分享链接（每行一个）
	FormatUnknown Format = "unknown"
)

// 系统脚手架 outbound tag（Compose 时从模板保留，不当作用户节点）。
var systemOutboundTags = map[string]bool{
	"direct": true, "block": true, "dns-out": true, "to-mitmproxy": true, "proxy-node": true,
}

// linkSchemes 是支持的分享链接协议前缀。
var linkSchemes = []string{"vmess://", "vless://", "ss://", "trojan://", "hysteria2://", "hy2://"}

func hasSchemeLink(s string) bool {
	for _, sc := range linkSchemes {
		if strings.Contains(s, sc) {
			return true
		}
	}
	return false
}

// tryBase64 尝试把字符串按 base64 解码（兼容 URL-safe 与无填充）。
func tryBase64(s string) (string, bool) {
	s = strings.TrimSpace(s)
	// 去掉换行（有的订阅 base64 带换行）
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\n", ""), "\r", "")
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if dec, err := enc.DecodeString(s); err == nil {
			return string(dec), true
		}
	}
	return "", false
}

// Detect 探测输入格式。
func Detect(data []byte) Format {
	s := strings.TrimSpace(string(data))
	if s == "" {
		return FormatUnknown
	}
	// 1. sing-box 原生 JSON
	if s[0] == '{' {
		var m map[string]any
		if json.Unmarshal(data, &m) == nil {
			if _, ok := m["outbounds"]; ok {
				return FormatSingbox
			}
			if _, ok := m["inbounds"]; ok {
				return FormatSingbox
			}
		}
	}
	// 2. 明文分享链接
	if hasSchemeLink(s) {
		return FormatLinks
	}
	// 3. Clash YAML（含 proxies 键）
	var y map[string]any
	if yaml.Unmarshal(data, &y) == nil {
		if _, ok := y["proxies"]; ok {
			return FormatClash
		}
	}
	// 4. base64 订阅：解码后含分享链接
	if dec, ok := tryBase64(s); ok && hasSchemeLink(dec) {
		return FormatBase64
	}
	return FormatUnknown
}

// NormalizeToSingbox 把导入内容转为完整可运行的 sing-box 配置 JSON。
// sing-box 原生原样返回；其他格式转成 outbounds 后合进 scaffold 脚手架。
// 返回 (sing-box JSON, 探测到的格式, error)。
func NormalizeToSingbox(data []byte, scaffold []byte) (string, Format, error) {
	f := Detect(data)
	switch f {
	case FormatSingbox:
		return string(data), f, nil
	case FormatClash:
		proxies, err := clashToOutbounds(data)
		if err != nil {
			return "", f, err
		}
		out, err := Compose(scaffold, proxies)
		return out, f, err
	case FormatLinks:
		proxies := linksToOutbounds(string(data))
		if len(proxies) == 0 {
			return "", f, fmt.Errorf("未解析出任何节点")
		}
		out, err := Compose(scaffold, proxies)
		return out, f, err
	case FormatBase64:
		dec, _ := tryBase64(strings.TrimSpace(string(data)))
		proxies := linksToOutbounds(dec)
		if len(proxies) == 0 {
			return "", f, fmt.Errorf("base64 解码后未解析出任何节点")
		}
		out, err := Compose(scaffold, proxies)
		return out, f, err
	default:
		return "", f, fmt.Errorf("无法识别的配置格式（支持 sing-box JSON / Clash YAML / base64 订阅 / 分享链接）")
	}
}

// Compose 把用户代理节点合成进脚手架 sing-box 配置：
//   - 保留脚手架的 inbounds/route/dns/log/experimental 等系统结构不变；
//   - 保留脚手架里的系统 outbound（direct/to-mitmproxy/dns-out/block）；
//   - 移除脚手架里占位的用户节点，注入传入的代理节点；
//   - 建一个 tag="proxy-node" 的 selector 组覆盖所有导入节点（route.final 仍指向它）。
func Compose(scaffold []byte, proxies []map[string]any) (string, error) {
	var obj map[string]any
	if err := json.Unmarshal(scaffold, &obj); err != nil {
		return "", fmt.Errorf("脚手架模板解析失败: %w", err)
	}

	// 保留系统 outbound（proxy-node 占位除外，它将被 selector 组替换）。
	var kept []any
	if arr, ok := obj["outbounds"].([]any); ok {
		for _, o := range arr {
			m, ok := o.(map[string]any)
			if !ok {
				continue
			}
			tag, _ := m["tag"].(string)
			if tag == "proxy-node" {
				continue // 占位，替换为 selector 组
			}
			if systemOutboundTags[tag] {
				kept = append(kept, m) // direct/to-mitmproxy/dns-out/block 保留
			}
			// 其余（旧的用户节点）丢弃
		}
	}

	// 注入导入节点 + proxy-node selector 组。
	var nodeTags []string
	var nodeObs []any
	for _, p := range proxies {
		tag, _ := p["tag"].(string)
		if tag == "" || systemOutboundTags[tag] {
			continue // 跳过空 tag / 与系统 tag 冲突的
		}
		nodeTags = append(nodeTags, tag)
		nodeObs = append(nodeObs, p)
	}
	if len(nodeTags) == 0 {
		return "", fmt.Errorf("无有效代理节点")
	}
	selector := map[string]any{
		"type":      "selector",
		"tag":       "proxy-node",
		"outbounds": toAnySlice(nodeTags),
		"default":   nodeTags[0],
	}

	// 顺序：导入节点 → proxy-node 组 → 系统 outbound。
	final := append([]any{}, nodeObs...)
	final = append(final, selector)
	final = append(final, kept...)
	obj["outbounds"] = final

	out, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func toAnySlice(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}
