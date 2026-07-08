package convert

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

const scaffold = `{
  "log": {"level": "info"},
  "inbounds": [{"type":"tun","tag":"tun-in"},{"type":"mixed","tag":"mixed-back","listen_port":1081}],
  "outbounds": [
    {"type":"direct","tag":"proxy-node"},
    {"type":"direct","tag":"direct"},
    {"type":"http","tag":"to-mitmproxy","server":"127.0.0.1","server_port":9080}
  ],
  "route": {"final":"proxy-node","rules":[]}
}`

func vmessLink(name string) string {
	j, _ := json.Marshal(map[string]any{
		"v": "2", "ps": name, "add": "vmess.example.com", "port": "443",
		"id": "11111111-2222-3333-4444-555555555555", "aid": "0",
		"scy": "auto", "net": "ws", "path": "/ray", "host": "cdn.example.com", "tls": "tls",
	})
	return "vmess://" + base64.StdEncoding.EncodeToString(j)
}

func TestDetect(t *testing.T) {
	cases := []struct {
		in   string
		want Format
	}{
		{scaffold, FormatSingbox},
		{"trojan://pass@t.example.com:443?sni=t.example.com#trojan-a", FormatLinks},
		{"proxies:\n  - {name: n1, type: ss, server: s.com, port: 8388, cipher: aes-256-gcm, password: p}\n", FormatClash},
		{base64.StdEncoding.EncodeToString([]byte("trojan://pass@t.com:443#a\nss://" + base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:pw")) + "@s.com:8388#b")), FormatBase64},
		{"", FormatUnknown},
	}
	for i, c := range cases {
		if got := Detect([]byte(c.in)); got != c.want {
			t.Errorf("case %d: Detect = %q, want %q", i, got, c.want)
		}
	}
}

func TestParseLinks(t *testing.T) {
	links := strings.Join([]string{
		vmessLink("vmess-a"),
		"vless://11111111-2222-3333-4444-555555555555@vless.example.com:443?type=ws&security=tls&sni=vless.example.com&path=/v#vless-b",
		"ss://" + base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:mypass")) + "@ss.example.com:8388#ss-c",
		"trojan://tjpass@trojan.example.com:443?sni=trojan.example.com#trojan-d",
		"hysteria2://hy2pass@hy.example.com:8443?sni=hy.example.com&insecure=1#hy-e",
	}, "\n")
	obs := linksToOutbounds(links)
	if len(obs) != 5 {
		t.Fatalf("解析出 %d 个节点，期望 5", len(obs))
	}
	byType := map[string]map[string]any{}
	for _, o := range obs {
		byType[o["type"].(string)] = o
	}
	if v := byType["vmess"]; v == nil || v["uuid"] != "11111111-2222-3333-4444-555555555555" || v["network"] != "ws" {
		t.Errorf("vmess 解析不对: %+v", v)
	}
	if v := byType["vless"]; v == nil || v["server"] != "vless.example.com" || v["tls"] == nil {
		t.Errorf("vless 解析不对: %+v", v)
	}
	if v := byType["shadowsocks"]; v == nil || v["method"] != "aes-256-gcm" || v["password"] != "mypass" {
		t.Errorf("ss 解析不对: %+v", v)
	}
	if v := byType["trojan"]; v == nil || v["server_port"] != 443 {
		t.Errorf("trojan 解析不对: %+v", v)
	}
	if v := byType["hysteria2"]; v == nil || v["password"] != "hy2pass" {
		t.Errorf("hysteria2 解析不对: %+v", v)
	}
}

func TestClashToOutbounds(t *testing.T) {
	y := `
proxies:
  - {name: ss-node, type: ss, server: s.example.com, port: 8388, cipher: aes-256-gcm, password: pw}
  - {name: vmess-node, type: vmess, server: v.example.com, port: 443, uuid: uid-1, cipher: auto, network: ws, tls: true, ws-opts: {path: /ws}}
  - {name: unknown-node, type: snell, server: x.com, port: 1}
`
	obs, err := clashToOutbounds([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 2 { // snell 跳过
		t.Fatalf("转换出 %d 个节点，期望 2（snell 应跳过）", len(obs))
	}
}

func TestNormalizeAndCompose(t *testing.T) {
	links := "trojan://pw@t.example.com:443?sni=t.example.com#node1\ntrojan://pw@t2.example.com:443#node2"
	out, format, err := NormalizeToSingbox([]byte(links), []byte(scaffold))
	if err != nil {
		t.Fatal(err)
	}
	if format != FormatLinks {
		t.Errorf("格式 = %q, 期望 links", format)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("合成结果非合法 JSON: %v", err)
	}
	outbounds := obj["outbounds"].([]any)
	tags := map[string]map[string]any{}
	for _, o := range outbounds {
		m := o.(map[string]any)
		tags[m["tag"].(string)] = m
	}
	// 系统脚手架保留
	if tags["direct"] == nil || tags["to-mitmproxy"] == nil {
		t.Error("系统 outbound（direct/to-mitmproxy）应保留")
	}
	// proxy-node 变成 selector 组，覆盖导入节点
	pn := tags["proxy-node"]
	if pn == nil || pn["type"] != "selector" {
		t.Fatalf("proxy-node 应为 selector 组: %+v", pn)
	}
	members := pn["outbounds"].([]any)
	if len(members) != 2 {
		t.Errorf("proxy-node 组应含 2 个成员，实际 %d", len(members))
	}
	// 导入节点存在
	if tags["node1"] == nil || tags["node2"] == nil {
		t.Error("导入的 node1/node2 应存在于 outbounds")
	}
	// route.final 仍指向 proxy-node（合成不动 route）
	if obj["route"].(map[string]any)["final"] != "proxy-node" {
		t.Error("route.final 应保持 proxy-node")
	}
}

func TestSingboxPassthrough(t *testing.T) {
	out, format, err := NormalizeToSingbox([]byte(scaffold), []byte(scaffold))
	if err != nil {
		t.Fatal(err)
	}
	if format != FormatSingbox {
		t.Errorf("格式 = %q, 期望 singbox", format)
	}
	if out != scaffold {
		t.Error("sing-box 原生输入应原样返回")
	}
}
