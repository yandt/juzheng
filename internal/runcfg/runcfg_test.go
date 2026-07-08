package runcfg

import (
	"encoding/json"
	"testing"
)

const fullCfg = `{
  "log": {"level": "info"},
  "dns": {"servers": [{"tag":"s","address":"8.8.8.8"}]},
  "inbounds": [
    {"type":"tun","tag":"tun-in","mtu":9000},
    {"type":"mixed","tag":"mixed-back","listen_port":1081}
  ],
  "outbounds": [
    {"type":"vmess","tag":"node-a","server":"a.com","server_port":443},
    {"type":"trojan","tag":"node-b","server":"b.com","server_port":443},
    {"type":"selector","tag":"my-group","outbounds":["node-a","node-b"]},
    {"type":"selector","tag":"proxy-node","outbounds":["node-a"],"default":"node-a"},
    {"type":"direct","tag":"direct"},
    {"type":"http","tag":"to-mitmproxy","server":"127.0.0.1","server_port":9080}
  ],
  "route": {
    "final": "proxy-node",
    "auto_detect_interface": true,
    "rules": [
      {"domain_suffix":["ads.com"],"outbound":"node-a"},
      {"domain_suffix":["anthropic.com"],"outbound":"to-mitmproxy"},
      {"protocol":"dns","action":"hijack-dns"}
    ]
  }
}`

func parse(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestSplit(t *testing.T) {
	full := parse(t, fullCfg)
	profile, source := Split(full)

	// Profile 拿到系统段
	if profile["log"] == nil || profile["dns"] == nil || profile["inbounds"] == nil {
		t.Error("profile 缺少 log/dns/inbounds")
	}
	// Profile 系统 outbound：direct + to-mitmproxy（proxy-node 被剔除，node-* 归源）
	sysOut := profile["outbounds"].([]any)
	if len(sysOut) != 2 {
		t.Errorf("profile 系统 outbound 应为 2（direct/to-mitmproxy），实际 %d", len(sysOut))
	}
	// Source 节点 2 个，组 1 个（my-group；proxy-node 不算）
	if n := len(source["outbounds"].([]any)); n != 2 {
		t.Errorf("源节点应 2 个，实际 %d", n)
	}
	if g := len(source["groups"].([]any)); g != 1 {
		t.Errorf("源组应 1 个（my-group），实际 %d", g)
	}
	// route 规则：node-a 规则 → 源；to-mitmproxy/dns → Profile
	if sr := len(source["rules"].([]any)); sr != 1 {
		t.Errorf("源用户规则应 1 条，实际 %d", sr)
	}
	if pr := len(profile["route"].(map[string]any)["rules"].([]any)); pr != 2 {
		t.Errorf("profile 系统规则应 2 条，实际 %d", pr)
	}
	// route.final 保留在 profile
	if profile["route"].(map[string]any)["final"] != "proxy-node" {
		t.Error("profile.route.final 应保留 proxy-node")
	}
}

func TestMergeRebuildsProxyNodeAndSystemPersists(t *testing.T) {
	full := parse(t, fullCfg)
	profile, source := Split(full)

	// 模拟"换订阅"：改 Profile 的 TUN mtu，再用另一份源合成，验证系统设置随 Profile 走。
	profile["inbounds"].([]any)[0].(map[string]any)["mtu"] = 1400

	merged := Merge(profile, source)
	// 系统设置（TUN mtu）来自 Profile
	if merged["inbounds"].([]any)[0].(map[string]any)["mtu"].(float64) != 1400 {
		t.Error("合成后 TUN mtu 应来自 Profile(1400)")
	}
	// proxy-node 被重建，覆盖源节点/组
	obs := merged["outbounds"].([]any)
	var pn map[string]any
	for _, o := range obs {
		m := o.(map[string]any)
		if m["tag"] == "proxy-node" {
			pn = m
		}
	}
	if pn == nil || pn["type"] != "selector" {
		t.Fatal("合成后应重建 proxy-node 选择器")
	}
	if len(pn["outbounds"].([]any)) != 3 { // node-a, node-b, my-group
		t.Errorf("proxy-node 应覆盖 3 个成员（2 节点+1 组），实际 %d", len(pn["outbounds"].([]any)))
	}
	// 系统 outbound（direct/to-mitmproxy）仍在
	tags := map[string]bool{}
	for _, o := range obs {
		tags[o.(map[string]any)["tag"].(string)] = true
	}
	if !tags["direct"] || !tags["to-mitmproxy"] {
		t.Error("合成后应保留 direct/to-mitmproxy")
	}
	// route.rules = 源规则(1) + 系统规则(2)
	if n := len(merged["route"].(map[string]any)["rules"].([]any)); n != 3 {
		t.Errorf("合成后 route.rules 应 3 条，实际 %d", n)
	}
}

func TestRoundTrip(t *testing.T) {
	full := parse(t, fullCfg)
	p, s := Split(full)
	merged := Merge(p, s)
	// 再拆一次应得到等价结果（幂等性）
	p2, s2 := Split(merged)
	if len(s2["outbounds"].([]any)) != len(s["outbounds"].([]any)) {
		t.Error("round-trip 源节点数不一致")
	}
	if len(p2["route"].(map[string]any)["rules"].([]any)) != len(p["route"].(map[string]any)["rules"].([]any)) {
		t.Error("round-trip 系统规则数不一致")
	}
}
