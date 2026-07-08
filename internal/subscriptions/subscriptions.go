// Package subscriptions 管理代理订阅的多实例 CRUD、导入导出、用量信息（L1 系统层）。
//
// 职责：
//   - 订阅 JSON 文件的增删改查（~/.juzheng/schemes/<name>.json）
//   - 从本地文件 / 远程 URL 导入（校验 sing-box 格式 + 抓取 Subscription-Userinfo）
//   - 订阅伙伴信息（机场用量/到期）读写（<name>.info.json）
//   - 切换活动订阅（拷贝到 singbox.json 运行副本 + 更新 meta）
//   - 订阅校验（isValidSingboxConfig）
//
// 依赖：L0 paths、L1 singboxcfg（模板 + meta）。不依赖 Wails、不依赖事件系统。
// 上层 service（subscriptionservice / singboxservice）通过本包操作订阅。
package subscriptions

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/zhanghui/juzheng/internal/convert"
	"github.com/zhanghui/juzheng/internal/paths"
	"github.com/zhanghui/juzheng/internal/singboxcfg"
)

// normalizeImport 把任意支持格式的导入内容（sing-box JSON / Clash YAML / base64 订阅 / 分享链接）
// 转为完整可运行的 sing-box 配置：非原生格式用内嵌模板作脚手架合成。
func normalizeImport(data []byte) (string, error) {
	tmpl, err := singboxcfg.TemplateContent()
	if err != nil {
		return "", fmt.Errorf("读取脚手架模板失败: %w", err)
	}
	out, format, err := convert.NormalizeToSingbox(data, tmpl)
	if err != nil {
		return "", fmt.Errorf("配置转换失败（格式 %s）: %w", format, err)
	}
	return out, nil
}

// defaultUserAgent 是下载订阅时使用的默认 UA。机场常据此识别客户端，
// 集中在此便于统一调整；也可用环境变量 JUZHENG_SUBSCRIPTION_UA 覆盖。
const defaultUserAgent = "clash-verge/v2.0 sing-box/1.13"

// subscriptionUserAgent 返回订阅下载的 UA（环境变量覆盖优先）。
func subscriptionUserAgent() string {
	if ua := os.Getenv("JUZHENG_SUBSCRIPTION_UA"); ua != "" {
		return ua
	}
	return defaultUserAgent
}

// ===== 订阅类型（与原 singbox_types.go 的 SubscriptionMeta/SubscriptionInfo JSON 兼容）=====

// Meta 是一个代理订阅的概要信息（列表项）。
type Meta struct {
	Name      string `json:"name"`      // 订阅名（不含扩展名）
	FileName  string `json:"fileName"`  // 文件名（含 .json）
	Active    bool   `json:"active"`    // 是否为当前活动订阅
	UpdatedAt int64  `json:"updatedAt"` // 修改时间（Unix 秒）
	NodeCount int    `json:"nodeCount"` // outbound 节点数
	SourceURL string `json:"sourceUrl"` // 订阅来源 URL（仅 URL 导入的有）
	Info      *Info  `json:"info"`      // 机场用量/到期信息
}

// Info 是机场通过 Subscription-Userinfo 响应头提供的用量/到期信息。
type Info struct {
	Upload    int64 `json:"upload"`    // 已上传字节
	Download  int64 `json:"download"`  // 已下载字节
	Total     int64 `json:"total"`     // 总流量配额字节
	Expire    int64 `json:"expire"`    // 到期 Unix 时间戳（0=无）
	FetchedAt int64 `json:"fetchedAt"` // 上次从机场更新此信息的时间（Unix 秒）
}

// IsValidSingboxConfig 校验内容是否为有效的 sing-box 配置（含 outbounds 或 inbounds）。
func IsValidSingboxConfig(content []byte) bool {
	var cfg struct {
		Outbounds []any `json:"outbounds"`
		Inbounds  []any `json:"inbounds"`
	}
	if err := json.Unmarshal(content, &cfg); err != nil {
		return false
	}
	return len(cfg.Outbounds) > 0 || len(cfg.Inbounds) > 0
}

// ===== 订阅 CRUD =====

// List 列出所有订阅（活动订阅排首位，其余按更新时间倒序）。
func List() ([]Meta, error) {
	schemesDir, err := paths.SchemesDir()
	if err != nil {
		return nil, err
	}
	meta, _ := singboxcfg.ReadMeta()
	entries, err := os.ReadDir(schemesDir)
	if err != nil {
		return nil, err
	}
	var list []Meta
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		if strings.HasSuffix(e.Name(), ".info.json") {
			continue
		}
		name := e.Name()[:len(e.Name())-len(".json")]
		fi, _ := e.Info()
		nodeCount := 0
		if data, err := os.ReadFile(filepath.Join(schemesDir, e.Name())); err == nil {
			var cfg struct {
				Outbounds []any `json:"outbounds"`
			}
			if json.Unmarshal(data, &cfg) == nil {
				nodeCount = len(cfg.Outbounds)
			}
		}
		subInfo, sourceURL := loadInfo(name)
		list = append(list, Meta{
			Name:      name,
			FileName:  e.Name(),
			Active:    name == meta.ActiveSubscription,
			UpdatedAt: fi.ModTime().Unix(),
			NodeCount: nodeCount,
			SourceURL: sourceURL,
			Info:      subInfo,
		})
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Active != list[j].Active {
			return list[i].Active
		}
		return list[i].UpdatedAt > list[j].UpdatedAt
	})
	return list, nil
}

// Get 读指定订阅 JSON 内容。
func Get(name string) (string, error) {
	p, err := paths.SchemeFile(name)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "", fmt.Errorf("订阅 %q 不存在: %w", name, err)
	}
	return string(data), nil
}

// Set 写订阅 JSON（不存在则新建）。
// Set 写入订阅内容。归一化统一入口：任意支持格式（sing-box/Clash/base64/分享链接）
// 都转成完整 sing-box 配置再落盘；sing-box 原生原样透传（编辑回写等内部调用无副作用）。
func Set(name, content string) error {
	if name == "" {
		return fmt.Errorf("订阅名不能为空")
	}
	norm, err := normalizeImport([]byte(content))
	if err != nil {
		return err
	}
	schemesDir, err := paths.SchemesDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(schemesDir, 0o755); err != nil {
		return err
	}
	p, err := paths.SchemeFile(name)
	if err != nil {
		return err
	}
	return os.WriteFile(p, []byte(norm), 0o600)
}

// Delete 删除订阅（不允许删活动订阅）。
func Delete(name string) error {
	meta, _ := singboxcfg.ReadMeta()
	if name == meta.ActiveSubscription {
		return fmt.Errorf("不能删除活动订阅")
	}
	p, err := paths.SchemeFile(name)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil {
		return err
	}
	if ip, _ := paths.SchemeInfoFile(name); ip != "" {
		os.Remove(ip)
	}
	return nil
}

// Rename 重命名订阅（同步重命名伙伴信息文件 + 更新 meta 活动订阅名）。
func Rename(old, newName string) error {
	if newName == "" {
		return fmt.Errorf("新订阅名不能为空")
	}
	oldPath, err := paths.SchemeFile(old)
	if err != nil {
		return err
	}
	newPath, err := paths.SchemeFile(newName)
	if err != nil {
		return err
	}
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("订阅 %q 已存在", newName)
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return err
	}
	if oldInfo, _ := paths.SchemeInfoFile(old); oldInfo != "" {
		if newInfo, _ := paths.SchemeInfoFile(newName); newInfo != "" {
			os.Rename(oldInfo, newInfo)
		}
	}
	meta, _ := singboxcfg.ReadMeta()
	if meta.ActiveSubscription == old {
		meta.ActiveSubscription = newName
		_ = singboxcfg.WriteMeta(meta)
	}
	return nil
}

// SetActive 设为活动订阅：内容拷到 singbox.json + 更新 meta。
func SetActive(name string) error {
	p, err := paths.SchemeFile(name)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("订阅 %q 不存在: %w", name, err)
	}
	// 运行配置 = Merge(系统 Profile, 该订阅的代理源)：订阅的系统部分丢弃，用 Profile 覆盖，
	// 保证换订阅时 TUN/DNS/MITM 等系统设置不变。合成失败则回退为原样写入。
	running, err := singboxcfg.ComposeRunning(string(data))
	if err != nil {
		running = string(data)
	}
	if err := singboxcfg.WriteSingboxConfig(running); err != nil {
		return err
	}
	meta, _ := singboxcfg.ReadMeta()
	meta.ActiveSubscription = name
	return singboxcfg.WriteMeta(meta)
}

// GetActive 返回当前活动订阅名。
func GetActive() string {
	meta, _ := singboxcfg.ReadMeta()
	return meta.ActiveSubscription
}

// CreateFromTemplate 以模板创建新订阅。source 为空用默认模板；"active" 复制当前活动订阅。
func CreateFromTemplate(name, source string) error {
	var content []byte
	if source == "active" {
		content, _ = singboxcfg.ReadSingboxConfigBytes()
	} else {
		content, _ = singboxcfg.TemplateContent()
	}
	return Set(name, string(content))
}

// ===== 导入 / 导出 =====

// ImportFromFile 从本地文件导入为订阅（校验 sing-box 格式）。
func ImportFromFile(name, path string) error {
	if name == "" {
		return fmt.Errorf("订阅名不能为空")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}
	return Set(name, string(data)) // Set 内部归一化（支持多格式）
}

// ImportFromURL 从远程订阅 URL 下载 sing-box JSON 并存为订阅。
// 同时抓取 Subscription-Userinfo 响应头存为伙伴信息。
func ImportFromURL(name, url string) error {
	if name == "" {
		return fmt.Errorf("订阅名不能为空")
	}
	if url == "" {
		return fmt.Errorf("订阅 URL 不能为空")
	}
	body, subInfo, err := fetch(url)
	if err != nil {
		return err
	}
	if err := Set(name, string(body)); err != nil { // Set 内部归一化（支持多格式）
		return err
	}
	return saveInfo(name, url, subInfo)
}

// RefreshInfo 重新请求订阅 URL 刷新用量/到期信息（不更新节点配置）。
func RefreshInfo(name string) error {
	_, sourceURL := loadInfo(name)
	if sourceURL == "" {
		return fmt.Errorf("订阅 %q 无来源 URL（非 URL 导入），无法刷新用量", name)
	}
	_, subInfo, err := fetch(sourceURL)
	if err != nil {
		return err
	}
	if subInfo == nil {
		return fmt.Errorf("机场未返回用量信息（无 Subscription-Userinfo 头）")
	}
	return saveInfo(name, sourceURL, subInfo)
}

// fetch 下载订阅，返回 body + 解析后的用量信息（可能为 nil）。
func fetch(url string) ([]byte, *Info, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("重定向次数过多")
			}
			return nil
		},
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", subscriptionUserAgent())
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, nil, fmt.Errorf("读取响应失败: %w", err)
	}
	// 不再限制必须 sing-box 原生：格式转换交给 normalizeImport（支持 Clash/base64/分享链接）。
	subInfo := parseUserinfo(resp.Header.Get("Subscription-Userinfo"))
	return body, subInfo, nil
}

// Export 读订阅内容返回（前端配合 Dialogs.SaveFile 写出）。
func Export(name string) (string, error) {
	return Get(name)
}

// ===== 订阅伙伴信息（机场用量/到期）=====

// loadInfo 读取订阅的伙伴信息文件，返回 (Info, sourceURL)。
func loadInfo(name string) (*Info, string) {
	p, err := paths.SchemeInfoFile(name)
	if err != nil {
		return nil, ""
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, ""
	}
	var rec struct {
		SourceURL string `json:"source_url"`
		Info      *Info  `json:"info"`
	}
	if json.Unmarshal(data, &rec) == nil {
		return rec.Info, rec.SourceURL
	}
	return nil, ""
}

// saveInfo 写伙伴信息文件。
func saveInfo(name, sourceURL string, info *Info) error {
	p, err := paths.SchemeInfoFile(name)
	if err != nil {
		return err
	}
	rec := struct {
		SourceURL string `json:"source_url"`
		Info      *Info  `json:"info"`
	}{SourceURL: sourceURL, Info: info}
	data, _ := json.MarshalIndent(rec, "", "  ")
	return os.WriteFile(p, data, 0o600)
}

// parseUserinfo 解析 Subscription-Userinfo 响应头。
// 格式: upload=...; download=...; total=...; expire=...
func parseUserinfo(header string) *Info {
	if header == "" {
		return nil
	}
	info := &Info{}
	for _, part := range strings.Split(header, ";") {
		part = strings.TrimSpace(part)
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.TrimSpace(kv[0])
		v := strings.TrimSpace(kv[1])
		var n int64
		fmt.Sscanf(v, "%d", &n)
		switch k {
		case "upload":
			info.Upload = n
		case "download":
			info.Download = n
		case "total":
			info.Total = n
		case "expire":
			info.Expire = n
		}
	}
	info.FetchedAt = time.Now().Unix()
	if info.Upload == 0 && info.Download == 0 && info.Total == 0 && info.Expire == 0 {
		return nil
	}
	return info
}
