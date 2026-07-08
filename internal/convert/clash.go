// clash.go — Clash / Clash Meta YAML 的 proxies → sing-box outbound。
// 移植自 scripts/clash2singbox.py 的 convert_proxy。Phase 1 只转节点（proxies），
// 代理组由 Compose 统一建 proxy-node selector 覆盖全部节点。
package convert

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func clashToOutbounds(data []byte) ([]map[string]any, error) {
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("Clash YAML 解析失败: %w", err)
	}
	if len(doc.Proxies) == 0 {
		return nil, fmt.Errorf("Clash 配置里没有 proxies 节点")
	}
	var out []map[string]any
	seen := map[string]int{}
	for _, p := range doc.Proxies {
		ob := convertClashProxy(p)
		if ob == nil {
			continue // 未支持类型跳过
		}
		tag, _ := ob["tag"].(string)
		if tag == "" {
			continue
		}
		if n, dup := seen[tag]; dup {
			seen[tag] = n + 1
			ob["tag"] = fmt.Sprintf("%s-%d", tag, n+1)
		} else {
			seen[tag] = 1
		}
		out = append(out, ob)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("Clash proxies 中没有可转换的节点类型")
	}
	return out, nil
}

func cs(m map[string]any, k string) string {
	switch x := m[k].(type) {
	case string:
		return x
	case int:
		return fmt.Sprintf("%d", x)
	case float64:
		return fmt.Sprintf("%d", int(x))
	}
	return ""
}
func ci(m map[string]any, k string) int {
	switch x := m[k].(type) {
	case int:
		return x
	case float64:
		return int(x)
	case string:
		return atoiSafe(x)
	}
	return 0
}
func cb(m map[string]any, k string) bool {
	b, _ := m[k].(bool)
	return b
}

func convertClashProxy(p map[string]any) map[string]any {
	t := cs(p, "type")
	name := cs(p, "name")
	server := cs(p, "server")
	port := ci(p, "port")
	if name == "" || server == "" {
		return nil
	}
	switch t {
	case "vmess":
		ob := map[string]any{
			"type": "vmess", "tag": name, "server": server, "server_port": port,
			"uuid": cs(p, "uuid"), "security": firstNonEmpty(cs(p, "cipher"), "auto"),
			"alter_id": ci(p, "alterId"),
		}
		net := firstNonEmpty(cs(p, "network"), "tcp")
		ob["network"] = net
		if net == "ws" {
			ws, _ := p["ws-opts"].(map[string]any)
			path := "/"
			var headers any
			if ws != nil {
				path = firstNonEmpty(cs(ws, "path"), "/")
				headers = ws["headers"]
			}
			tr := map[string]any{"type": "ws", "path": path}
			if headers != nil {
				tr["headers"] = headers
			}
			ob["transport"] = tr
		}
		if cb(p, "tls") {
			ob["tls"] = map[string]any{"enabled": true, "server_name": firstNonEmpty(cs(p, "sni"), server)}
		}
		return ob
	case "vless":
		ob := map[string]any{
			"type": "vless", "tag": name, "server": server, "server_port": port,
			"uuid": cs(p, "uuid"), "network": firstNonEmpty(cs(p, "network"), "tcp"),
		}
		if fl := cs(p, "flow"); fl != "" {
			ob["flow"] = fl
		}
		if cb(p, "tls") {
			ob["tls"] = map[string]any{"enabled": true, "server_name": firstNonEmpty(cs(p, "sni"), server)}
		}
		return ob
	case "trojan":
		ob := map[string]any{
			"type": "trojan", "tag": name, "server": server, "server_port": port,
			"password": cs(p, "password"),
		}
		ob["tls"] = map[string]any{"enabled": true, "server_name": firstNonEmpty(cs(p, "sni"), server)}
		return ob
	case "ss", "shadowsocks":
		return map[string]any{
			"type": "shadowsocks", "tag": name, "server": server, "server_port": port,
			"method": cs(p, "cipher"), "password": cs(p, "password"),
		}
	case "hysteria2":
		ob := map[string]any{
			"type": "hysteria2", "tag": name, "server": server, "server_port": port,
			"password": firstNonEmpty(cs(p, "password"), cs(p, "auth")),
		}
		if up := ci(p, "up"); up > 0 {
			ob["up_mbps"] = up
		}
		if down := ci(p, "down"); down > 0 {
			ob["down_mbps"] = down
		}
		return ob
	}
	return nil
}
