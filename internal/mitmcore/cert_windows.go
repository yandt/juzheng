//go:build windows

// cert_windows.go：Windows 的 CA 证书信任 —— certutil 装入本机「受信任的根证书颁发机构」(Root) 存储。

package mitmcore

import (
	"crypto/sha1"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// certInstallCmd 返回供用户手动执行的信任命令（前端展示/复制用；需管理员）。
func certInstallCmd(path string) string {
	return fmt.Sprintf(`certutil -addstore -f Root "%s"`, path)
}

// installCertTrust 将 CA 证书装入本机 Root 存储。
// 方式 1：直接执行 certutil（app 已提权时成立）。
// 方式 2：PowerShell Start-Process -Verb RunAs 触发 UAC 提权执行。
func installCertTrust(path string) (string, error) {
	if out, err := exec.Command("certutil", "-addstore", "-f", "Root", path).CombinedOutput(); err == nil {
		return string(out), nil
	}
	// UAC 提权：Start-Process certutil -Verb RunAs（单引号内路径按整体元素传入，容忍空格）
	psPath := strings.ReplaceAll(path, "'", "''")
	ps := fmt.Sprintf(
		`Start-Process -FilePath certutil -Verb RunAs -Wait -ArgumentList @('-addstore','-f','Root','%s')`, psPath)
	out, err := exec.Command("powershell", "-NoProfile", "-Command", ps).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("证书安装失败（可能取消 UAC）：请以管理员运行 %s：%w", certInstallCmd(path), err)
	}
	return "已请求将证书装入「受信任的根证书颁发机构」，请在 UAC 弹窗中允许。", nil
}

// isCertTrusted 校验证书是否已在 Root 存储中：按 SHA1 指纹查 certutil -verifystore Root。
func isCertTrusted(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	cert, err := x509.ParseCertificate(data)
	if err != nil {
		return false
	}
	sum := sha1.Sum(cert.Raw)
	thumb := strings.ToUpper(hex.EncodeToString(sum[:]))
	return exec.Command("certutil", "-verifystore", "Root", thumb).Run() == nil
}
