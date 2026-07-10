//go:build !windows

// preflight_other.go：非 Windows 平台无需 TUN 驱动预检（macOS 用系统 utun）。

package main

// preflightStart 非 Windows 为空实现。
func preflightStart(_ string) error { return nil }
