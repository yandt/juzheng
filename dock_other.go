//go:build !darwin

package main

// setDockVisible 在非 macOS 平台无 Dock 概念，空实现。
// Windows 下窗口显隐由托盘 + 窗口 Show/Hide 承担（见 main.go 的托盘逻辑）。
func setDockVisible(_ bool) {}
