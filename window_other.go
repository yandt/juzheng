//go:build !darwin

package main

// framelessWindow：非 macOS（Windows/Linux）用无边框窗口，隐藏系统默认标题栏。
// 窗口拖拽靠前端顶栏的 -webkit-app-region: drag 区域；窗口控制靠界面内自绘的三按钮。
const framelessWindow = true

// hideOnMinimise：非 macOS 走系统默认最小化到任务栏（不隐藏），与自绘最小化按钮行为一致。
const hideOnMinimise = false
