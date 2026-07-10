// monitor.go — 内容层监控引擎（在 MITM 解密后的明文上做规则匹配 → 转发/报警/修改/拦截）。
//
// 与"抓包域名"正交：抓包域名决定哪些域名进 MITM 解密；监控规则决定解密后怎么处理。
// 规则支持运行中热更新（monitorEngine 内部 RWMutex），无需重启代理。
package mitmcore

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
	"github.com/zhanghui/juzheng/internal/events"
)

// ===== 规则模型（JSON 与前端对齐）=====

// MonitorCondition 单个匹配条件。respBody 在请求阶段不可用（视为不命中）。
type MonitorCondition struct {
	Field string `json:"field"` // host|url|path|reqBody|respBody
	Op    string `json:"op"`    // contains|equals|suffix|regex
	Value string `json:"value"`
}

// MonitorBlock action=block 的返回。Status=0 默认 403，Body="" 默认提示。
type MonitorBlock struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

// MonitorModify action=modify 的改写规格。
type MonitorModify struct {
	Target      string `json:"target"`      // body|header|status|url
	BodyOp      string `json:"bodyOp"`      // replace|regexReplace|setFull
	Find        string `json:"find"`        // replace/regexReplace 的查找
	Replace     string `json:"replace"`     // 替换为（regexReplace 支持 $1）
	HeaderName  string `json:"headerName"`  // target=header
	HeaderValue string `json:"headerValue"` // 空=删除该头
	Status      int    `json:"status"`      // target=status（仅 response）
	URL         string `json:"url"`         // target=url（仅 request，改目标/重定向）
}

// MonitorRule 一条监控规则。conditions 按 logic(and/or) 组合。
// action：forward(白名单放行,终止)|alert(报警,继续)|modify(改写,继续)|block(拦截,终止)。
type MonitorRule struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Enabled    bool               `json:"enabled"`
	Phase      string             `json:"phase"` // request|response|both
	Logic      string             `json:"logic"` // and|or
	Conditions []MonitorCondition `json:"conditions"`
	Action     string             `json:"action"`
	Block      *MonitorBlock      `json:"block,omitempty"`
	Modify     *MonitorModify     `json:"modify,omitempty"`
	AlertLevel string             `json:"alertLevel"` // info|warn|danger
}

// MonitorHit 命中事件（上抛前端：报警面板 + 流量标记）。
type MonitorHit struct {
	FlowID   string `json:"flowId"`
	RuleID   string `json:"ruleId"`
	RuleName string `json:"ruleName"`
	Phase    string `json:"phase"`  // request|response
	Action   string `json:"action"` // alert|modify|block
	Field    string `json:"field"`  // 命中的（首个）条件字段
	Level    string `json:"level"`  // info|warn|danger
	Host     string `json:"host"`
	URL      string `json:"url"`
	Time     int64  `json:"time"` // unix 毫秒
}

// ===== 引擎 =====

type compiledRule struct {
	r      MonitorRule
	condRe []*regexp.Regexp // 与 Conditions 一一对应，非 regex 为 nil
	findRe *regexp.Regexp   // modify body regexReplace 编译结果
}

// monitorEngine 线程安全规则引擎，支持运行中热更新。
type monitorEngine struct {
	mu    sync.RWMutex
	rules []compiledRule
}

func newMonitorEngine() *monitorEngine { return &monitorEngine{} }

// SetRules 替换全部规则（预编译正则）。运行中可随时调用。
func (e *monitorEngine) SetRules(rules []MonitorRule) {
	compiled := make([]compiledRule, 0, len(rules))
	for _, r := range rules {
		cr := compiledRule{r: r, condRe: make([]*regexp.Regexp, len(r.Conditions))}
		for i, c := range r.Conditions {
			if c.Op == "regex" {
				if re, err := regexp.Compile(c.Value); err == nil {
					cr.condRe[i] = re
				}
			}
		}
		if r.Action == "modify" && r.Modify != nil && r.Modify.Target == "body" && r.Modify.BodyOp == "regexReplace" {
			if re, err := regexp.Compile(r.Modify.Find); err == nil {
				cr.findRe = re
			}
		}
		compiled = append(compiled, cr)
	}
	e.mu.Lock()
	e.rules = compiled
	e.mu.Unlock()
}

func matchOne(c MonitorCondition, re *regexp.Regexp, host, urlStr, path, reqBody, respBody string) bool {
	var subject string
	switch c.Field {
	case "host":
		subject = host
	case "url":
		subject = urlStr
	case "path":
		subject = path
	case "reqBody":
		subject = reqBody
	case "respBody":
		subject = respBody
	default:
		return false
	}
	switch c.Op {
	case "contains":
		return strings.Contains(subject, c.Value)
	case "equals":
		return subject == c.Value
	case "suffix":
		return strings.HasSuffix(subject, c.Value)
	case "regex":
		return re != nil && re.MatchString(subject)
	}
	return false
}

// matches 评估规则条件，返回是否命中及首个命中字段。
func (cr compiledRule) matches(host, urlStr, path, reqBody, respBody string) (bool, string) {
	if len(cr.r.Conditions) == 0 {
		return false, ""
	}
	if cr.r.Logic == "or" {
		for i, c := range cr.r.Conditions {
			if matchOne(c, cr.condRe[i], host, urlStr, path, reqBody, respBody) {
				return true, c.Field
			}
		}
		return false, ""
	}
	// and（默认）：全部命中
	for i, c := range cr.r.Conditions {
		if !matchOne(c, cr.condRe[i], host, urlStr, path, reqBody, respBody) {
			return false, ""
		}
	}
	return true, cr.r.Conditions[0].Field
}

func phaseMatches(rulePhase, cur string) bool { return rulePhase == cur || rulePhase == "both" }

// evalRequest 请求阶段评估并执行动作，返回命中事件。命中 forward/block 终止。
func (e *monitorEngine) evalRequest(f *proxy.Flow) []MonitorHit {
	e.mu.RLock()
	rules := e.rules
	e.mu.RUnlock()
	if len(rules) == 0 || f.Request == nil {
		return nil
	}
	host, urlStr, path := f.Request.URL.Host, f.Request.URL.String(), f.Request.URL.Path
	reqBody := decodedReq(f)
	var hits []MonitorHit
	for _, cr := range rules {
		if !cr.r.Enabled || !phaseMatches(cr.r.Phase, "request") {
			continue
		}
		ok, field := cr.matches(host, urlStr, path, reqBody, "")
		if !ok {
			continue
		}
		switch cr.r.Action {
		case "forward":
			return hits // 白名单：命中即放行并终止
		case "alert":
			hits = append(hits, newHit(f, cr.r, "request", "alert", field))
		case "modify":
			applyModifyRequest(cr, f)
			hits = append(hits, newHit(f, cr.r, "request", "modify", field))
			reqBody = decodedReq(f)
		case "block":
			setBlockResponse(cr.r, f)
			hits = append(hits, newHit(f, cr.r, "request", "block", field))
			return hits
		}
	}
	return hits
}

// evalResponse 响应阶段评估并执行动作。
func (e *monitorEngine) evalResponse(f *proxy.Flow) []MonitorHit {
	e.mu.RLock()
	rules := e.rules
	e.mu.RUnlock()
	if len(rules) == 0 || f.Request == nil || f.Response == nil {
		return nil
	}
	host, urlStr, path := f.Request.URL.Host, f.Request.URL.String(), f.Request.URL.Path
	reqBody, respBody := decodedReq(f), decodedResp(f)
	var hits []MonitorHit
	for _, cr := range rules {
		if !cr.r.Enabled || !phaseMatches(cr.r.Phase, "response") {
			continue
		}
		ok, field := cr.matches(host, urlStr, path, reqBody, respBody)
		if !ok {
			continue
		}
		switch cr.r.Action {
		case "forward":
			return hits
		case "alert":
			hits = append(hits, newHit(f, cr.r, "response", "alert", field))
		case "modify":
			applyModifyResponse(cr, f)
			hits = append(hits, newHit(f, cr.r, "response", "modify", field))
			respBody = decodedResp(f)
		case "block":
			setBlockResponse(cr.r, f)
			hits = append(hits, newHit(f, cr.r, "response", "block", field))
			return hits
		}
	}
	return hits
}

func newHit(f *proxy.Flow, r MonitorRule, phase, action, field string) MonitorHit {
	lvl := r.AlertLevel
	if lvl == "" {
		lvl = "info"
	}
	return MonitorHit{
		FlowID: f.Id.String(), RuleID: r.ID, RuleName: r.Name,
		Phase: phase, Action: action, Field: field, Level: lvl,
		Host: f.Request.URL.Host, URL: f.Request.URL.String(),
		Time: time.Now().UnixMilli(),
	}
}

func decodedReq(f *proxy.Flow) string {
	if f.Request == nil {
		return ""
	}
	if b, err := f.Request.DecodedBody(); err == nil {
		return string(b)
	}
	return string(f.Request.Body)
}

func decodedResp(f *proxy.Flow) string {
	if f.Response == nil {
		return ""
	}
	if b, err := f.Response.DecodedBody(); err == nil {
		return string(b)
	}
	return string(f.Response.Body)
}

func setBlockResponse(r MonitorRule, f *proxy.Flow) {
	status, body := 403, "已被 juzheng 监控规则拦截"
	if r.Block != nil {
		if r.Block.Status != 0 {
			status = r.Block.Status
		}
		if r.Block.Body != "" {
			body = r.Block.Body
		}
	}
	h := http.Header{}
	h.Set("Content-Type", "text/plain; charset=utf-8")
	f.Response = &proxy.Response{StatusCode: status, Header: h, Body: []byte(body)}
}

func applyModifyRequest(cr compiledRule, f *proxy.Flow) {
	m := cr.r.Modify
	if m == nil || f.Request == nil {
		return
	}
	switch m.Target {
	case "body":
		b, err := f.Request.DecodedBody()
		if err != nil {
			b = f.Request.Body
		}
		nb := applyBodyOp(m, cr.findRe, b)
		f.Request.Body = nb
		f.Request.Header.Del("Content-Encoding")
		f.Request.Header.Set("Content-Length", strconv.Itoa(len(nb)))
	case "header":
		if m.HeaderName == "" {
			return
		}
		if m.HeaderValue == "" {
			f.Request.Header.Del(m.HeaderName)
		} else {
			f.Request.Header.Set(m.HeaderName, m.HeaderValue)
		}
	case "url":
		if m.URL != "" {
			if u, err := url.Parse(m.URL); err == nil {
				f.Request.URL = u
			}
		}
	}
}

func applyModifyResponse(cr compiledRule, f *proxy.Flow) {
	m := cr.r.Modify
	if m == nil || f.Response == nil {
		return
	}
	switch m.Target {
	case "body":
		f.Response.ReplaceToDecodedBody() // 解压 + 去 Content-Encoding + 重算长度
		nb := applyBodyOp(m, cr.findRe, f.Response.Body)
		f.Response.Body = nb
		f.Response.Header.Set("Content-Length", strconv.Itoa(len(nb)))
	case "header":
		if m.HeaderName == "" {
			return
		}
		if m.HeaderValue == "" {
			f.Response.Header.Del(m.HeaderName)
		} else {
			f.Response.Header.Set(m.HeaderName, m.HeaderValue)
		}
	case "status":
		if m.Status != 0 {
			f.Response.StatusCode = m.Status
		}
	}
}

func applyBodyOp(m *MonitorModify, findRe *regexp.Regexp, body []byte) []byte {
	switch m.BodyOp {
	case "setFull":
		return []byte(m.Replace)
	case "regexReplace":
		if findRe != nil {
			return findRe.ReplaceAll(body, []byte(m.Replace))
		}
		return body
	case "replace":
		if m.Find == "" {
			return body
		}
		return []byte(strings.ReplaceAll(string(body), m.Find, m.Replace))
	}
	return body
}

// ===== monitorAddon（挂在 captureAddon 之后：先记录原始流量，再评估执行）=====

type monitorAddon struct {
	proxy.BaseAddon
	engine  *monitorEngine
	emitter events.EventEmitter
}

func (a *monitorAddon) Request(f *proxy.Flow) {
	for _, h := range a.engine.evalRequest(f) {
		a.emitter.Emit(events.MonitorHit, h)
	}
}

func (a *monitorAddon) Response(f *proxy.Flow) {
	for _, h := range a.engine.evalResponse(f) {
		a.emitter.Emit(events.MonitorHit, h)
	}
}
