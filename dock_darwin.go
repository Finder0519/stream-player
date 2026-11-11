//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit

int streamPlayerInstallDockReopen(void);
*/
import "C"

import (
	"log"

	"fyne.io/fyne/v2"
)

// dockReopen 保存点击 Dock 图标时要执行的操作。
var dockReopen func()

//export streamPlayerDockReopen
func streamPlayerDockReopen() {
	if show := dockReopen; show != nil {
		// 回调在 AppKit 主线程上、Fyne 事件循环内部触发，
		// 交给 Fyne 主循环执行，避免在事件处理过程中重入。
		fyne.Do(show)
	}
}

// installDockReopen 让窗口隐藏后点击程序坞图标重新显示窗口。
// Fyne 与 GLFW 都没有实现 applicationShouldHandleReopen:hasVisibleWindows:，
// 这里在运行时给现有的 NSApplicationDelegate 补上该方法。
func installDockReopen(show func()) {
	dockReopen = show
	if C.streamPlayerInstallDockReopen() == 0 {
		log.Printf("未能安装 Dock 点击唤回窗口的处理：NSApplication delegate 不可用")
	}
}
