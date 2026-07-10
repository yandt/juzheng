package singboxcfg

import (
	"encoding/json"
	"testing"
)

// parse 把 Sanitize 输出解析回 map 供断言。
func parse(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("sanitize 输出非合法 JSON: %v\n%s", err, s)
	}
	return m
}

func TestSanitize_StripsCommentsAndLegacyFields(t *testing.T) {
	in := `{
		"_comment": "顶层注释",
		"inbounds": [{"type":"tun","tag":"tun-in","sniff":true,"domain_strategy":"prefer_ipv4"}],
		"outbounds": [{"type":"direct","tag":"direct"}]
	}`
	out := parse(t, Sanitize(in))
	if _, ok := out["_comment"]; ok {
		t.Error("顶层 _comment 未被剥离")
	}
	ib := out["inbounds"].([]any)[0].(map[string]any)
	if _, ok := ib["sniff"]; ok {
		t.Error("legacy 字段 sniff 未被删除")
	}
	if _, ok := ib["domain_strategy"]; ok {
		t.Error("legacy 字段 domain_strategy 未被删除")
	}
}

func TestSanitize_RemovesSpecialOutboundsAndMapsRuleAction(t *testing.T) {
	in := `{
		"outbounds": [
			{"type":"direct","tag":"direct"},
			{"type":"block","tag":"block-out"},
			{"type":"dns","tag":"dns-out"}
		],
		"route": {
			"rules": [
				{"outbound":"block-out","domain":["ads.example"]},
				{"outbound":"dns-out","protocol":"dns"}
			],
			"final": "block-out"
		}
	}`
	out := parse(t, Sanitize(in))

	// block/dns outbound 应被移除。
	for _, ob := range out["outbounds"].([]any) {
		if tag, _ := ob.(map[string]any)["tag"].(string); tag == "block-out" || tag == "dns-out" {
			t.Errorf("特殊 outbound %s 未被移除", tag)
		}
	}
	// 规则的 outbound 应转为 action。
	rules := out["route"].(map[string]any)["rules"].([]any)
	r0 := rules[0].(map[string]any)
	if r0["action"] != "reject" {
		t.Errorf("block 规则未转为 action=reject，实际: %v", r0)
	}
	if _, ok := r0["outbound"]; ok {
		t.Error("转 action 后 outbound 字段应删除")
	}
	r1 := rules[1].(map[string]any)
	if r1["action"] != "hijack-dns" {
		t.Errorf("dns 规则未转为 action=hijack-dns，实际: %v", r1)
	}
	// final 指向被删 outbound 应改 direct。
	if out["route"].(map[string]any)["final"] != "direct" {
		t.Error("final 指向被删 outbound 时应改为 direct")
	}
}

func TestSanitize_DropsGeoRules(t *testing.T) {
	in := `{
		"outbounds": [{"type":"direct","tag":"direct"}],
		"route": {"rules": [
			{"geosite":["cn"],"outbound":"direct"},
			{"geoip":["cn"],"outbound":"direct"},
			{"domain":["keep.example"],"outbound":"direct"}
		]}
	}`
	out := parse(t, Sanitize(in))
	rules := out["route"].(map[string]any)["rules"].([]any)
	if len(rules) != 1 {
		t.Fatalf("含 geosite/geoip 的规则应被丢弃，仅剩 1 条，实际 %d 条", len(rules))
	}
	if got := rules[0].(map[string]any)["domain"].([]any)[0]; got != "keep.example" {
		t.Errorf("保留的规则不对: %v", got)
	}
}

func TestSanitize_InjectsQuicRejectBeforeCapture(t *testing.T) {
	in := `{
		"outbounds": [{"type":"direct","tag":"direct"}],
		"route": {"rules": [
			{"action":"sniff"},
			{"inbound":["tun-in"],"domain_keyword":["ipdata"],"outbound":"to-mitmproxy"},
			{"domain":["other.example"],"outbound":"direct"}
		]}
	}`
	rules := parse(t, Sanitize(in))["route"].(map[string]any)["rules"].([]any)
	// 抓包规则前应多出一条 QUIC reject（同 inbound + 同域名）
	if len(rules) != 4 {
		t.Fatalf("应在抓包规则前注入 1 条 QUIC reject，规则数应为 4，实际 %d", len(rules))
	}
	rej := rules[1].(map[string]any)
	if rej["action"] != "reject" || rej["network"] != "udp" {
		t.Errorf("注入的规则应为 udp reject，实际 %v", rej)
	}
	if kw := rej["domain_keyword"].([]any); kw[0] != "ipdata" {
		t.Errorf("QUIC reject 域名应与抓包一致，实际 %v", kw)
	}
	if ib := rej["inbound"].([]any); ib[0] != "tun-in" {
		t.Errorf("QUIC reject inbound 应与抓包一致，实际 %v", ib)
	}
	// 紧随其后应是抓包规则
	if rules[2].(map[string]any)["outbound"] != "to-mitmproxy" {
		t.Errorf("QUIC reject 应紧邻在抓包规则之前")
	}
	// 幂等：再 sanitize 一次不应重复注入
	rules2 := parse(t, Sanitize(Sanitize(in)))["route"].(map[string]any)["rules"].([]any)
	if len(rules2) != 4 {
		t.Fatalf("二次 sanitize 不应重复注入，规则数应仍为 4，实际 %d", len(rules2))
	}
}

func TestSanitize_FixesUrltestIntervalAndCleansMembers(t *testing.T) {
	in := `{
		"outbounds": [
			{"type":"block","tag":"block-out"},
			{"type":"urltest","tag":"auto","interval":180,"outbounds":["a","block-out"]}
		]
	}`
	out := parse(t, Sanitize(in))
	for _, ob := range out["outbounds"].([]any) {
		m := ob.(map[string]any)
		if m["type"] == "urltest" {
			if m["interval"] != "180s" {
				t.Errorf("interval 数字未转为字符串 秒，实际: %v", m["interval"])
			}
			members := m["outbounds"].([]any)
			if len(members) != 1 || members[0] != "a" {
				t.Errorf("指向被删 outbound 的成员未清理: %v", members)
			}
		}
	}
}

func TestSanitize_RedirectsDanglingDNSDetour(t *testing.T) {
	in := `{
		"dns": {"servers": [{"tag":"s","detour":"nonexistent"}]},
		"outbounds": [{"type":"direct","tag":"direct"}]
	}`
	out := parse(t, Sanitize(in))
	srv := out["dns"].(map[string]any)["servers"].([]any)[0].(map[string]any)
	if srv["detour"] != "direct" {
		t.Errorf("指向不存在 outbound 的 DNS detour 应改为 direct，实际: %v", srv["detour"])
	}
}

func TestSanitize_InvalidJSONReturnedVerbatim(t *testing.T) {
	in := `{not json`
	if Sanitize(in) != in {
		t.Error("非法 JSON 应原样返回")
	}
}
