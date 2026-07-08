// links.go — 分享链接（vmess/vless/ss/trojan/hysteria2://）→ sing-box outbound。
package convert

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
)

// linksToOutbounds 解析多行分享链接（每行一个）为 sing-box outbounds。
// 无法解析的行跳过；tag 去重（重名追加序号）。
func linksToOutbounds(text string) []map[string]any {
	var out []map[string]any
	seen := map[string]int{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		ob := parseLink(line)
		if ob == nil {
			continue
		}
		tag, _ := ob["tag"].(string)
		if tag == "" {
			tag = "node"
		}
		if n, dup := seen[tag]; dup {
			seen[tag] = n + 1
			tag = tag + "-" + strconv.Itoa(n+1)
			ob["tag"] = tag
		} else {
			seen[tag] = 1
		}
		out = append(out, ob)
	}
	return out
}

// parseLink 分派单条分享链接。
func parseLink(link string) map[string]any {
	switch {
	case strings.HasPrefix(link, "vmess://"):
		return parseVmess(link)
	case strings.HasPrefix(link, "vless://"):
		return parseVless(link)
	case strings.HasPrefix(link, "ss://"):
		return parseSS(link)
	case strings.HasPrefix(link, "trojan://"):
		return parseTrojan(link)
	case strings.HasPrefix(link, "hysteria2://"):
		return parseHysteria2(strings.TrimPrefix(link, "hysteria2://"))
	case strings.HasPrefix(link, "hy2://"):
		return parseHysteria2(strings.TrimPrefix(link, "hy2://"))
	}
	return nil
}

func b64any(s string) (string, bool) {
	s = strings.TrimSpace(s)
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if dec, err := enc.DecodeString(s); err == nil {
			return string(dec), true
		}
	}
	return "", false
}

func atoiSafe(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// vmess://base64({v,ps,add,port,id,aid,scy,net,type,host,path,tls,sni})
func parseVmess(link string) map[string]any {
	raw := strings.TrimPrefix(link, "vmess://")
	dec, ok := b64any(raw)
	if !ok {
		return nil
	}
	var v map[string]any
	if json.Unmarshal([]byte(dec), &v) != nil {
		return nil
	}
	getS := func(k string) string {
		switch x := v[k].(type) {
		case string:
			return x
		case float64:
			return strconv.Itoa(int(x))
		}
		return ""
	}
	ob := map[string]any{
		"type":        "vmess",
		"tag":         firstNonEmpty(getS("ps"), getS("add")),
		"server":      getS("add"),
		"server_port": atoiSafe(getS("port")),
		"uuid":        getS("id"),
		"security":    firstNonEmpty(getS("scy"), "auto"),
		"alter_id":    atoiSafe(getS("aid")),
	}
	net := firstNonEmpty(getS("net"), "tcp")
	ob["network"] = net
	if net == "ws" {
		tr := map[string]any{"type": "ws", "path": firstNonEmpty(getS("path"), "/")}
		if h := getS("host"); h != "" {
			tr["headers"] = map[string]any{"Host": h}
		}
		ob["transport"] = tr
	}
	if getS("tls") == "tls" {
		ob["tls"] = map[string]any{"enabled": true, "server_name": firstNonEmpty(getS("sni"), getS("host"), getS("add"))}
	}
	return ob
}

// vless://uuid@host:port?params#name
func parseVless(link string) map[string]any {
	u, err := url.Parse(link)
	if err != nil || u.User == nil {
		return nil
	}
	q := u.Query()
	ob := map[string]any{
		"type":        "vless",
		"tag":         firstNonEmpty(decodeFragment(u.Fragment), u.Hostname()),
		"server":      u.Hostname(),
		"server_port": atoiSafe(u.Port()),
		"uuid":        u.User.Username(),
		"network":     firstNonEmpty(q.Get("type"), "tcp"),
	}
	if fl := q.Get("flow"); fl != "" {
		ob["flow"] = fl
	}
	if q.Get("security") == "tls" || q.Get("security") == "reality" {
		ob["tls"] = map[string]any{"enabled": true, "server_name": firstNonEmpty(q.Get("sni"), u.Hostname())}
	}
	if q.Get("type") == "ws" {
		tr := map[string]any{"type": "ws", "path": firstNonEmpty(q.Get("path"), "/")}
		if h := q.Get("host"); h != "" {
			tr["headers"] = map[string]any{"Host": h}
		}
		ob["transport"] = tr
	}
	return ob
}

// ss://base64(method:password)@host:port#name  或  ss://base64(method:password@host:port)#name
func parseSS(link string) map[string]any {
	body := strings.TrimPrefix(link, "ss://")
	name := ""
	if i := strings.Index(body, "#"); i >= 0 {
		name = decodeFragment(body[i+1:])
		body = body[:i]
	}
	if i := strings.Index(body, "?"); i >= 0 { // 去掉 plugin 等参数
		body = body[:i]
	}
	var method, password, host, port string
	if at := strings.LastIndex(body, "@"); at >= 0 {
		// SIP002: base64(method:password)@host:port
		userPart, ok := b64any(body[:at])
		if !ok {
			userPart = body[:at]
		}
		if c := strings.Index(userPart, ":"); c >= 0 {
			method, password = userPart[:c], userPart[c+1:]
		}
		hp := body[at+1:]
		host, port = splitHostPort(hp)
	} else {
		// 旧式：base64(method:password@host:port)
		dec, ok := b64any(body)
		if !ok {
			return nil
		}
		if at := strings.LastIndex(dec, "@"); at >= 0 {
			userPart := dec[:at]
			if c := strings.Index(userPart, ":"); c >= 0 {
				method, password = userPart[:c], userPart[c+1:]
			}
			host, port = splitHostPort(dec[at+1:])
		}
	}
	if host == "" {
		return nil
	}
	return map[string]any{
		"type":        "shadowsocks",
		"tag":         firstNonEmpty(name, host),
		"server":      host,
		"server_port": atoiSafe(port),
		"method":      method,
		"password":    password,
	}
}

// trojan://password@host:port?sni=..#name
func parseTrojan(link string) map[string]any {
	u, err := url.Parse(link)
	if err != nil || u.User == nil {
		return nil
	}
	q := u.Query()
	ob := map[string]any{
		"type":        "trojan",
		"tag":         firstNonEmpty(decodeFragment(u.Fragment), u.Hostname()),
		"server":      u.Hostname(),
		"server_port": atoiSafe(u.Port()),
		"password":    u.User.Username(),
	}
	sni := firstNonEmpty(q.Get("sni"), q.Get("peer"), u.Hostname())
	ob["tls"] = map[string]any{"enabled": true, "server_name": sni}
	return ob
}

// hysteria2://password@host:port?sni=..&insecure=1#name
func parseHysteria2(rest string) map[string]any {
	u, err := url.Parse("hysteria2://" + rest)
	if err != nil || u.User == nil {
		return nil
	}
	q := u.Query()
	ob := map[string]any{
		"type":        "hysteria2",
		"tag":         firstNonEmpty(decodeFragment(u.Fragment), u.Hostname()),
		"server":      u.Hostname(),
		"server_port": atoiSafe(u.Port()),
		"password":    u.User.Username(),
	}
	tls := map[string]any{"enabled": true, "server_name": firstNonEmpty(q.Get("sni"), u.Hostname())}
	if q.Get("insecure") == "1" || q.Get("insecure") == "true" {
		tls["insecure"] = true
	}
	ob["tls"] = tls
	return ob
}

func splitHostPort(hp string) (string, string) {
	if i := strings.LastIndex(hp, ":"); i >= 0 {
		return hp[:i], hp[i+1:]
	}
	return hp, ""
}

func decodeFragment(f string) string {
	if s, err := url.QueryUnescape(f); err == nil {
		return s
	}
	return f
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
