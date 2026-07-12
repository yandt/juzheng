// Package singboxcfg 负责 sing-box 配置文件与应用元数据的读写（L1 系统层）。
//
// 职责：
//   - singbox.json（活动订阅运行副本，helper 读这个）的读写
//   - 首次运行时落盘默认配置模板（内嵌 configs/singbox-template.json）
//   - meta.json（活动订阅名 + 首页卡片配置）的读写
//
// 依赖：L0 paths。不依赖 Wails、不依赖事件系统。
// 上层 service（singboxservice）通过本包读写配置，本包不感知上层。
package singboxcfg

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/zhanghui/juzheng/internal/paths"
)

// MaxConfigBytes 是读取 singbox.json 的大小上限，防止畸形超大文件撑爆内存。
const MaxConfigBytes = 16 * 1024 * 1024

// atomicWriteFile 原子写文件：先写同目录临时文件再 rename，避免并发写或写到一半崩溃导致文件损坏。
// 同目录 rename 在同一文件系统上是原子操作。
func atomicWriteFile(p string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// 出错路径清理临时文件（成功 rename 后 tmpName 已不存在，Remove 无副作用）。
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, p)
}

// 内嵌配置模板（供首次运行落盘）。
//
//go:embed configs/singbox-template.json
var templateFS embed.FS

// TemplateContent 返回内嵌的 sing-box 配置模板内容。
func TemplateContent() ([]byte, error) {
	return templateFS.ReadFile("configs/singbox-template.json")
}

// ===== meta.json 数据结构（与原 singbox_types.go 的 AppMeta/CardConfig JSON 兼容）=====

// AppMeta 是 ~/.juzheng/meta.json 的结构：活动订阅 + 首页卡片配置。
// JSON tag activeScheme 保留不变（兼容老 meta.json）。
type AppMeta struct {
	ActiveSubscription string       `json:"activeScheme"` // 当前活动订阅名
	Cards              []CardConfig `json:"cards"`        // 首页卡片显隐/排序
}

// CardConfig 是单个首页卡片的显隐与排序配置。
type CardConfig struct {
	Key     string `json:"key"`     // 卡片标识（预设枚举，值持久化勿改）
	Visible bool   `json:"visible"` // 是否显示
	Order   int    `json:"order"`   // 排序序号
}

// 预设卡片标识（前端 HomePage 据此渲染对应卡片组件）。字符串值持久化在 meta.json，勿改。
const (
	CardCurrentSubscription = "current-scheme" // 当前订阅（key 值保留兼容）
	CardProxyGroups         = "proxy-groups"   // 代理组节点
	CardQuickToggles        = "quick-toggles"  // 快速开关
	CardTraffic             = "traffic"        // 流量统计
)

// DefaultMeta 返回默认 meta（含 4 个预设卡片，前 3 个可见）。
func DefaultMeta(active string) AppMeta {
	return AppMeta{
		ActiveSubscription: active,
		Cards: []CardConfig{
			{Key: CardCurrentSubscription, Visible: true, Order: 0},
			{Key: CardProxyGroups, Visible: true, Order: 1},
			{Key: CardQuickToggles, Visible: true, Order: 2},
			{Key: CardTraffic, Visible: false, Order: 3},
		},
	}
}

// ===== singbox.json（活动订阅运行副本）读写 =====

// ReadSingboxConfig 读取 ~/.juzheng/singbox.json 内容。
func ReadSingboxConfig() (string, error) {
	p, err := paths.SingboxConfigPath()
	if err != nil {
		return "", err
	}
	data, err := readCapped(p)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// readCapped 读取文件但限制最大字节数，超限返回错误。
func readCapped(p string) ([]byte, error) {
	fi, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	if fi.Size() > MaxConfigBytes {
		return nil, fmt.Errorf("配置文件过大（%d 字节，上限 %d）", fi.Size(), MaxConfigBytes)
	}
	return os.ReadFile(p)
}

// ReadSingboxConfigBytes 读取 ~/.juzheng/singbox.json 原始字节。
func ReadSingboxConfigBytes() ([]byte, error) {
	p, err := paths.SingboxConfigPath()
	if err != nil {
		return nil, err
	}
	return readCapped(p)
}

// WriteSingboxConfig 写入 ~/.juzheng/singbox.json（自动创建父目录）。
func WriteSingboxConfig(content string) error {
	p, err := paths.SingboxConfigPath()
	if err != nil {
		return err
	}
	return atomicWriteFile(p, []byte(content), 0o600)
}

// EnsureDefaultConfig 若 singbox.json 不存在，落盘默认模板。
func EnsureDefaultConfig() error {
	p, err := paths.SingboxConfigPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(p); err == nil {
		return nil // 已存在
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmpl, err := TemplateContent()
	if err != nil {
		return err
	}
	return os.WriteFile(p, tmpl, 0o600)
}

// ===== meta.json 读写 =====

// ReadMeta 读取 meta.json。文件不存在或解析失败时返回默认 meta。
func ReadMeta() (AppMeta, error) {
	p, err := paths.MetaPath()
	if err != nil {
		return DefaultMeta(""), err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return DefaultMeta(""), nil
	}
	var m AppMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return DefaultMeta(""), nil
	}
	return m, nil
}

// WriteMeta 写入 meta.json（自动创建父目录）。
func WriteMeta(m AppMeta) error {
	p, err := paths.MetaPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 meta 失败: %w", err)
	}
	return atomicWriteFile(p, data, 0o600)
}

// EnsureDefaults 初始化应用数据目录（首次运行）：schemes 目录、默认订阅、meta.json、singbox.json。
// 从原 schemes_service.go 的 ensureSubscriptionsAndMeta 下沉至 L1（初始化属于配置层职责，不应在 Wails service 里做）。
func EnsureDefaults() error {
	// 一次性迁移旧版 ~/.juzheng 数据到规范数据目录（必须在任何路径读写之前）。
	if err := migrateLegacyDir(); err != nil {
		return err
	}
	schemesDir, err := paths.SchemesDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(schemesDir, 0o755); err != nil {
		return err
	}

	// 落盘默认 singbox.json（仅当不存在）。
	if err := EnsureDefaultConfig(); err != nil {
		return err
	}
	// 首次运行派生系统 Profile（从模板拆出系统部分）。
	if err := EnsureProfile(); err != nil {
		return err
	}
	// 迁移：mixed-back 旧默认端口 7897（与 Clash 冲突）→ 9788。
	MigrateMixedBackPort()

	metaPath, err := paths.MetaPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(metaPath); err != nil {
		// meta 不存在：首次运行。若 schemes 为空，用内嵌模板创建「默认订阅」。
		entries, _ := os.ReadDir(schemesDir)
		if len(entries) == 0 {
			tmpl, err := TemplateContent()
			if err != nil {
				return err
			}
			defaultPath, err := paths.SchemeFile(defaultSubscriptionName)
			if err != nil {
				return err
			}
			if err := atomicWriteFile(defaultPath, tmpl, 0o600); err != nil {
				return err
			}
		}
		if err := WriteMeta(DefaultMeta(defaultSubscriptionName)); err != nil {
			return err
		}
	}
	return nil
}

// defaultSubscriptionName 是首次运行创建的默认订阅名。
const defaultSubscriptionName = "默认订阅"

// migrateLegacyDir 一次性把旧版 ~/.juzheng 的数据迁到新的规范数据目录（os.UserConfigDir/Juzheng）。
// 仅当新目录尚不存在且旧目录存在时执行：优先整目录 rename（同卷瞬时完成），跨卷等 rename
// 失败则递归复制且保留旧目录（不删，安全回退）。新目录已存在（新装或已迁移）则跳过。
func migrateLegacyDir() error {
	newDir, err := paths.JuzhengDir()
	if err != nil {
		return err
	}
	if _, err := os.Stat(newDir); err == nil {
		return nil // 新目录已存在：新装或已迁移，不动
	}
	legacy, err := paths.LegacyJuzhengDir()
	if err != nil {
		return nil
	}
	if fi, err := os.Stat(legacy); err != nil || !fi.IsDir() {
		return nil // 无旧数据
	}
	if err := os.MkdirAll(filepath.Dir(newDir), 0o755); err != nil {
		return err
	}
	if err := os.Rename(legacy, newDir); err == nil {
		log.Printf("配置目录已迁移: %s → %s", legacy, newDir)
		return nil
	}
	// rename 失败（多为跨卷）：递归复制，保留旧目录作为回退。
	if err := copyDir(legacy, newDir); err != nil {
		return fmt.Errorf("迁移配置目录失败: %w", err)
	}
	log.Printf("配置目录已复制迁移(旧目录保留): %s → %s", legacy, newDir)
	return nil
}

// copyDir 递归复制目录树，保留文件/目录权限。
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, info.Mode().Perm())
		}
		return copyFile(p, target)
	})
}

// copyFile 复制单个文件，保留源权限。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
