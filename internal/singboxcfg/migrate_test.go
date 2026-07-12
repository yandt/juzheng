package singboxcfg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zhanghui/juzheng/internal/paths"
)

// TestMigrateLegacyDir 验证：旧 ~/.juzheng 数据被迁到规范目录，且迁移幂等。
func TestMigrateLegacyDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config")) // Linux 沙箱

	// 造旧数据（含子目录）。
	legacy := filepath.Join(home, ".juzheng")
	mkdir(t, filepath.Join(legacy, "schemes"))
	write(t, filepath.Join(legacy, "meta.json"), `{"active":"x"}`)
	write(t, filepath.Join(legacy, "schemes", "a.json"), `{}`)

	if err := migrateLegacyDir(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	newDir, err := paths.JuzhengDir()
	if err != nil {
		t.Fatal(err)
	}
	if newDir == legacy {
		t.Fatalf("new dir should differ from legacy: %s", newDir)
	}
	if b, err := os.ReadFile(filepath.Join(newDir, "meta.json")); err != nil || string(b) != `{"active":"x"}` {
		t.Fatalf("meta not migrated: err=%v content=%q", err, b)
	}
	if _, err := os.Stat(filepath.Join(newDir, "schemes", "a.json")); err != nil {
		t.Fatalf("scheme not migrated: %v", err)
	}

	// 幂等：新目录已存在，再迁不报错、不破坏。
	if err := migrateLegacyDir(); err != nil {
		t.Fatalf("migrate idempotent: %v", err)
	}
}

func mkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
		t.Fatal(err)
	}
}
