// Package runcfg 负责「系统 Profile」与「代理源」的拆分/合成（L1 系统层）。
//
// 目标：把系统脚手架（TUN/DNS/mixed-back/to-mitmproxy/MITM 规则/route 骨架/日志等）
// 与用户代理（节点/组/用户路由规则）解耦。运行配置 = Merge(Profile, 代理源)。
// 换订阅时系统设置（Profile）不变，只换代理源。
//
// 拆分归属（按 outbound tag）：
//   - 系统 outbound：direct/block/dns-out/to-mitmproxy（proxy-node 由 Merge 重建，不入 Profile）
//   - 用户节点：有具体协议、非系统 tag → 代理源
//   - 用户组：selector/urltest 且 tag != proxy-node → 代理源
//   - route.rules：outbound 指向用户节点/组 → 代理源；否则（系统出口/action/无 outbound）→ Profile
//   - inbounds/dns/log/experimental/route 其余字段（final 等）→ Profile
//
// 依赖：仅 stdlib。不依赖 Wails。
package runcfg

import "encoding/json"

// systemTags 是系统托管的 outbound tag（不当作用户节点）。
var systemTags = map[string]bool{
	"direct": true, "block": true, "dns-out": true, "to-mitmproxy": true, "proxy-node": true,
}

// ProxyGroupTag 是 Merge 自动重建的主出口选择器 tag（route.final 通常指向它）。
const ProxyGroupTag = "proxy-node"

func isGroupType(t string) bool { return t == "selector" || t == "urltest" }

func deepCopy(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if json.Unmarshal(b, &out) != nil {
		return v
	}
	return out
}

func toAnySlice(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

// Split 把完整 sing-box 配置拆成 (Profile 系统部分, Source 代理源)。
// Source 是内部存储结构（含 outbounds/groups/rules 三键），非直接可运行的 sing-box。
func Split(full map[string]any) (profile map[string]any, source map[string]any) {
	profile = map[string]any{}
	source = map[string]any{}

	// 顶层系统段直接归 Profile。
	for _, k := range []string{"log", "dns", "experimental", "inbounds"} {
		if v, ok := full[k]; ok {
			profile[k] = deepCopy(v)
		}
	}

	// outbounds 拆分。
	userTags := map[string]bool{}
	var sysOut, srcNodes, srcGroups []any
	if arr, ok := full["outbounds"].([]any); ok {
		for _, o := range arr {
			m, ok := o.(map[string]any)
			if !ok {
				continue
			}
			tag, _ := m["tag"].(string)
			t, _ := m["type"].(string)
			if systemTags[tag] {
				if tag != ProxyGroupTag { // proxy-node 由 Merge 重建
					sysOut = append(sysOut, deepCopy(m))
				}
				continue
			}
			if isGroupType(t) {
				srcGroups = append(srcGroups, deepCopy(m))
			} else {
				srcNodes = append(srcNodes, deepCopy(m))
			}
			userTags[tag] = true
		}
	}
	profile["outbounds"] = sysOut
	source["outbounds"] = srcNodes
	source["groups"] = srcGroups

	// route 拆分：规则按 outbound 归属；其余字段（final 等）归 Profile。
	var srcRules, sysRules []any
	routeProfile := map[string]any{}
	if route, ok := full["route"].(map[string]any); ok {
		for k, v := range route {
			if k != "rules" {
				routeProfile[k] = deepCopy(v)
			}
		}
		if rules, ok := route["rules"].([]any); ok {
			for _, r := range rules {
				rm, ok := r.(map[string]any)
				ob := ""
				if ok {
					ob, _ = rm["outbound"].(string)
				}
				if ob != "" && userTags[ob] {
					srcRules = append(srcRules, deepCopy(r))
				} else {
					sysRules = append(sysRules, deepCopy(r))
				}
			}
		}
	}
	routeProfile["rules"] = sysRules
	profile["route"] = routeProfile
	source["rules"] = srcRules

	return profile, source
}

// Merge 把 Profile 与 Source 合成完整可运行 sing-box 配置。
//   - outbounds = 源节点 + 源组 + 重建的 proxy-node 选择器 + Profile 系统 outbound
//   - route.rules = Profile 系统规则（前，MITM/系统优先）+ 源用户规则（后）
//   - inbounds/dns/log/experimental/route 其余字段 = Profile
func Merge(profile map[string]any, source map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range profile {
		out[k] = deepCopy(v)
	}

	// outbounds 合成。
	var obs []any
	var memberTags []string
	if nodes, ok := source["outbounds"].([]any); ok {
		for _, o := range nodes {
			obs = append(obs, deepCopy(o))
			if m, ok := o.(map[string]any); ok {
				if tag, _ := m["tag"].(string); tag != "" {
					memberTags = append(memberTags, tag)
				}
			}
		}
	}
	if groups, ok := source["groups"].([]any); ok {
		for _, o := range groups {
			obs = append(obs, deepCopy(o))
			if m, ok := o.(map[string]any); ok {
				if tag, _ := m["tag"].(string); tag != "" {
					memberTags = append(memberTags, tag)
				}
			}
		}
	}
	// 重建 proxy-node 主选择器（覆盖所有源节点/组）。
	if len(memberTags) > 0 {
		obs = append(obs, map[string]any{
			"type":      "selector",
			"tag":       ProxyGroupTag,
			"outbounds": toAnySlice(memberTags),
			"default":   memberTags[0],
		})
	}
	// Profile 系统 outbound（direct/to-mitmproxy/dns-out/block）。
	if sys, ok := profile["outbounds"].([]any); ok {
		for _, o := range sys {
			obs = append(obs, deepCopy(o))
		}
	}
	out["outbounds"] = obs

	// route.rules = 源用户规则 + Profile 系统规则。
	route := map[string]any{}
	if pr, ok := profile["route"].(map[string]any); ok {
		for k, v := range pr {
			if k != "rules" {
				route[k] = deepCopy(v)
			}
		}
	}
	var rules []any
	// MITM/系统优先：Profile 系统规则在前，源用户规则在后。
	if pr, ok := profile["route"].(map[string]any); ok {
		if prr, ok := pr["rules"].([]any); ok {
			rules = append(rules, prr...)
		}
	}
	if sr, ok := source["rules"].([]any); ok {
		rules = append(rules, sr...)
	}
	route["rules"] = rules
	out["route"] = route

	return out
}

// SourceOf 从完整配置里取代理源（Split 的 source 部分），便于按名加载订阅代理源。
func SourceOf(full map[string]any) map[string]any {
	_, s := Split(full)
	return s
}
