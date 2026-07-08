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
