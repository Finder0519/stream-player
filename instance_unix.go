//go:build !windows

package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
)

// instanceSocketPath 返回单实例 socket 路径，优先使用用户私有的运行时目录。
// macOS 没有 XDG_RUNTIME_DIR，退回 os.TempDir()（即用户私有的 TMPDIR）。
func instanceSocketPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, fmt.Sprintf("stream-player-%d.sock", os.Getuid()))
}

// listenInstance 让程序在 macOS / Linux 上保持单实例：已有实例时通知它显示窗口并返回 true；
// 否则监听 socket，收到通知后调用 onShow。
// Linux 上没有托盘宿主时这是唤回窗口的兜底（例如 GNOME 未安装 AppIndicator 扩展）；
// macOS 上 Finder 启动 .app 本来就会复用实例，这里额外覆盖终端重复执行 ./stream-player 的情况。
func listenInstance(onShow func()) (alreadyRunning bool, stop func()) {
	path := instanceSocketPath()

	if conn, err := net.DialTimeout("unix", path, 500*time.Millisecond); err == nil {
		_, _ = conn.Write([]byte("show\n"))
		_ = conn.Close()
		return true, func() {}
	}

	// 上次进程异常退出可能残留 socket 文件，先清理再监听。
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		log.Printf("单实例监听失败，按普通启动继续：%v", err)
		return false, func() {}
	}
	_ = os.Chmod(path, 0o600)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			buf := make([]byte, 16)
			_, _ = conn.Read(buf)
			_ = conn.Close()
			// 显示窗口必须在 Fyne 主线程执行，accept goroutine 不能直接调用。
			fyne.Do(onShow)
		}
	}()

	return false, func() {
		_ = ln.Close()
		_ = os.Remove(path)
	}
}
