// sanitize.go：把用户配置转为 sing-box 1.13+ 严格 JSON 兼容格式（内存转换，不改磁盘文件）。
//
// 从 singbox_service.go（L3 薄壳）下沉至此（L1 配置层）—— 配置转换是配置层职责。
//
// 可插拔：转换规则不再散落为 if/else 硬编码，而是集中到本文件顶部的注册表：
//   - specialOutbounds：sing-box 已移除的特殊 outbound 类型 → route action 映射
//   - legacyInboundFields：已弃用需删除的 inbound 字段
//   - commentKeys：需递归剥离的注释键
// 需支持新类型/新字段时，改注册表或调用 Register* 即可，无需改转换逻辑。

package singboxcfg

import (
	"encoding/json"
	"fmt"
)

// SpecialOutbound 描述一个 sing-box 新版已移除的特殊 outbound 类型，及其对应的 route action 替代。
type SpecialOutbound struct {
	Type   string // outbound 的 "type" 值，如 "block" / "dns"
	Action string // route rule 中的 action 替代值，如 "reject" / "hijack-dns"
}

// specialOutbounds 是特殊 outbound 注册表（可插拔）。
// sing-box 1.11+ 起把 block/dns 从 outbound 改为 route action。
var specialOutbounds = []SpecialOutbound{
	{Type: "block", Action: "reject"},
	{Type: "dns", Action: "hijack-dns"},
}

// legacyInboundFields 是需从 inbound 删除的弃用字段注册表（可插拔）。
// sing-box 1.11 弃用、1.13 移除。
var legacyInboundFields = []string{
	"sniff", "sniff_override_destination", "domain_strategy",
	"udp_disable_domain_unmapping", "destination_override", "source_ip_override",
}

// commentKeys 是需递归剥离的注释键（sing-box 严格 JSON 不认未知字段）。
var commentKeys = []string{"_comment"}

// geoRuleKeys 是 sing-box 1.12 删除内置 geo 数据库后需整条丢弃的 route rule 字段。
var geoRuleKeys = []string{"geosite", "geoip"}

// mitmCaptureOutbound 是明文抓包的解密出口 tag（命中它的规则 = 抓包规则）。
const mitmCaptureOutbound = "to-mitmproxy"

// domainMatchKeys 域名类匹配器键（注入 QUIC reject 规则时从抓包规则原样复制，保持范围一致）。
var domainMatchKeys = []string{"domain", "domain_suffix", "domain_keyword", "domain_regex"}

// RegisterSpecialOutbound 注册一个特殊 outbound 类型 → action 映射（供扩展/测试）。
func RegisterSpecialOutbound(so SpecialOutbound) { specialOutbounds = append(specialOutbounds, so) }

// RegisterLegacyInboundField 注册一个需删除的 inbound 弃用字段。
func RegisterLegacyInboundField(field string) { legacyInboundFields = append(legacyInboundFields, field) }

// Sanitize 把配置 JSON 字符串转为 sing-box 兼容格式（内存转换）。
// 只影响传给 helper 的字符串，不修改磁盘配置文件（文件保留注释/legacy 字段供用户查看/旧版兼容）。
// 解析失败时原样返回，交由 sing-box 自行报错。
func Sanitize(cfgStr string) string {
	var obj map[string]any
	if err := json.Unmarshal([]byte(cfgStr), &obj); err != nil {
		return cfgStr
	}

	// 1. 递归剥离注释键。
	stripKeys(obj, commentKeys)

	// 2. 删除 inbound legacy 字段。
	if inbounds, ok := obj["inbounds"].([]any); ok {
		for _, ib := range inbounds {
			if m, ok := ib.(map[string]any); ok {
				for _, f := range legacyInboundFields {
					delete(m, f)
				}
			}
		}
	}

	// 3. 移除特殊 outbound（block/dns），记录其 tag → 特殊类型，供后续 route/引用清理。
	specialByType := make(map[string]string) // outbound type → action
	for _, so := range specialOutbounds {
		specialByType[so.Type] = so.Action
	}
	removedTags := make(map[string]string) // 被删 outbound 的 tag → action
	var newOutbounds []any
	if outbounds, ok := obj["outbounds"].([]any); ok {
		for _, ob := range outbounds {
			m, ok := ob.(map[string]any)
			if !ok {
				newOutbounds = append(newOutbounds, ob)
				continue
			}
			t, _ := m["type"].(string)
			if action, special := specialByType[t]; special {
				if tag, _ := m["tag"].(string); tag != "" {
					removedTags[tag] = action
				}
				continue // 丢弃该 outbound
			}
			// urltest 的 interval 必须是字符串（如 "3m"），订阅可能返回数字。
			if t == "urltest" {
				fixIntervalField(m)
			}
			newOutbounds = append(newOutbounds, ob)
		}
	}
	obj["outbounds"] = newOutbounds

	// 4. 清理 selector/urltest 组里指向被删 outbound 的引用。
	for _, ob := range newOutbounds {
		if m, ok := ob.(map[string]any); ok {
			t, _ := m["type"].(string)
			if t == "selector" || t == "urltest" {
				if members, ok := m["outbounds"].([]any); ok {
					var cleaned []any
					for _, member := range members {
						if tag, ok := member.(string); ok {
							if _, removed := removedTags[tag]; removed {
								continue
							}
						}
						cleaned = append(cleaned, member)
					}
					m["outbounds"] = cleaned
				}
			}
		}
	}

	// 5. 收集有效 outbound tag，修正 DNS detour 指向不存在的 outbound → direct。
	validTags := make(map[string]bool)
	for _, ob := range newOutbounds {
		if m, ok := ob.(map[string]any); ok {
			if tag, ok := m["tag"].(string); ok {
				validTags[tag] = true
			}
		}
	}
	if dns, ok := obj["dns"].(map[string]any); ok {
		if servers, ok := dns["servers"].([]any); ok {
			for _, s := range servers {
				if sm, ok := s.(map[string]any); ok {
					if detour, ok := sm["detour"].(string); ok && !validTags[detour] {
						sm["detour"] = "direct"
					}
				}
			}
		}
	}

	// 6. 强制禁用 cache_file（helper 以 root 运行，cache.db 默认路径只读）。
	if exp, ok := obj["experimental"].(map[string]any); ok {
		if cf, ok := exp["cache_file"].(map[string]any); ok {
			cf["enabled"] = false
			delete(cf, "path")
		}
	}

	// 7. route.rules：丢弃含 geosite/geoip 的规则；特殊 outbound → action；final 指向被删则改 direct。
	if route, ok := obj["route"].(map[string]any); ok {
		if rules, ok := route["rules"].([]any); ok {
			var newRules []any
			for _, r := range rules {
				rm, ok := r.(map[string]any)
				if !ok {
					newRules = append(newRules, r)
					continue
				}
				if hasAnyKey(rm, geoRuleKeys) {
					continue // 整条丢弃
				}
				if ob, _ := rm["outbound"].(string); ob != "" {
					if action, found := removedTags[ob]; found {
						delete(rm, "outbound")
						rm["action"] = action
					}
				}
				newRules = append(newRules, rm)
			}
			route["rules"] = newRules
		}
		if final, ok := route["final"].(string); ok {
			if _, found := removedTags[final]; found {
				route["final"] = "direct"
			}
		}
		// 8. 明文抓包链路：为每条抓包(→to-mitmproxy)规则注入一条同域名的 QUIC(UDP) reject 规则。
		injectQuicReject(route)
	}

	out, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return cfgStr
	}
	return string(out)
}

// injectQuicReject 在每条"抓包(outbound=to-mitmproxy)"规则前插入一条 QUIC(UDP) reject 规则，
// 域名匹配器与抓包规则完全一致。
//
// 原因：明文抓包靠 go-mitmproxy 的 HTTP CONNECT(TCP) 解密，而 HTTP/3 走 QUIC(UDP)，
// HTTP 代理拦不到。若不拒掉，命中抓包域名的 HTTP/3 流量会绕过解密（漏抓）。
// 拒掉这些域名的 UDP 后，浏览器/应用会自动降级到可被解密的 TCP TLS，从而不漏。
// 幂等：已存在紧邻的同款 reject（action=reject+network=udp）则跳过，避免重复启动累积。
func injectQuicReject(route map[string]any) {
	rules, ok := route["rules"].([]any)
	if !ok {
		return
	}
	out := make([]any, 0, len(rules)+2)
	for _, r := range rules {
		rm, ok := r.(map[string]any)
		if ok {
			if ob, _ := rm["outbound"].(string); ob == mitmCaptureOutbound && hasAnyKey(rm, domainMatchKeys) && !quicRejectAlreadyBefore(out) {
				reject := map[string]any{"action": "reject", "network": "udp"}
				if inbound, ok := rm["inbound"]; ok {
					reject["inbound"] = inbound
				}
				for _, k := range domainMatchKeys {
					if v, ok := rm[k]; ok {
						reject[k] = v
					}
				}
				out = append(out, reject)
			}
		}
		out = append(out, r)
	}
	route["rules"] = out
}

// quicRejectAlreadyBefore 报告已构建切片的末尾是否已是一条 QUIC reject 规则（幂等保护）。
func quicRejectAlreadyBefore(out []any) bool {
	if len(out) == 0 {
		return false
	}
	if m, ok := out[len(out)-1].(map[string]any); ok {
		act, _ := m["action"].(string)
		net, _ := m["network"].(string)
		return act == "reject" && net == "udp"
	}
	return false
}

// fixIntervalField 把 interval 数字（秒）转为 sing-box 要求的字符串 "<n>s"。
// JSON 解码后数字恒为 float64，这里只需处理 float64。
func fixIntervalField(m map[string]any) {
	if iv, ok := m["interval"].(float64); ok {
		m["interval"] = fmt.Sprintf("%ds", int(iv))
	}
}

// hasAnyKey 报告 m 是否含 keys 中的任一键。
func hasAnyKey(m map[string]any, keys []string) bool {
	for _, k := range keys {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}

// stripKeys 递归移除 map 中的指定键。
func stripKeys(o any, keys []string) {
	switch v := o.(type) {
	case map[string]any:
		for _, k := range keys {
			delete(v, k)
		}
		for _, child := range v {
			stripKeys(child, keys)
		}
	case []any:
		for _, item := range v {
			stripKeys(item, keys)
		}
	}
}
