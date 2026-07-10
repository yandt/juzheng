//go:build darwin

// cert_darwin.go：macOS 的 CA 证书信任 —— security add-trusted-cert 装入系统钥匙串。

package mitmcore

import (
	"fmt"
	"os/exec"
)

// certInstallCmd 返回供用户手动执行的信任命令（前端展示/复制用）。
func certInstallCmd(path string) string {
	return fmt.Sprintf(`sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain "%s"`, path)
}

// installCertTrust 信任 CA 证书。
// 方式 1：osascript 提权执行 security add-trusted-cert（系统弹密码框）。
// 方式 2：降级 —— open 打开 .cer 文件，让用户在钥匙串应用里手动信任。
func installCertTrust(path string) (string, error) {
	script := fmt.Sprintf(`do shell script "security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain \"%s\"" with administrator privileges`, path)
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err == nil {
		return string(out), nil
	}
	if openErr := exec.Command("open", path).Run(); openErr != nil {
		return string(out), fmt.Errorf("证书安装失败，请手动双击证书文件信任: %s: %w", path, err)
	}
	return "已打开证书文件，请在钥匙串访问中双击该证书 → 信任 → 始终信任", nil
}

// isCertTrusted 校验证书是否已被系统信任。
func isCertTrusted(path string) bool {
	return exec.Command("security", "verify-cert", "-c", path).Run() == nil
}
