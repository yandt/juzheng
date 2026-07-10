//go:build darwin

package main

// framelessWindow：macOS 不用无边框 —— 走 MacWindow 的透明标题栏方案，
// 既隐藏标题文字又保留红黄绿窗口按钮。
const framelessWindow = false

// hideOnMinimise：macOS 最小化时隐藏窗口（靠 Dock/托盘唤起，符合原交互）。
const hideOnMinimise = true
