//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

// setDockIconVisible 运行时切换 Dock 图标：visible!=0 → Regular(显示)，否则 Accessory(隐藏)。
// Wails v3 未导出运行时切换激活策略的接口，这里直接调 AppKit。
static void setDockIconVisible(int visible) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [NSApp setActivationPolicy:(visible ? NSApplicationActivationPolicyRegular
                                            : NSApplicationActivationPolicyAccessory)];
    });
}
*/
import "C"

// setDockVisible 运行时显隐 macOS Dock 图标（true=显示/Regular，false=隐藏/Accessory）。
func setDockVisible(v bool) {
	if v {
		C.setDockIconVisible(1)
	} else {
		C.setDockIconVisible(0)
	}
}
