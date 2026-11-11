//go:build linux

package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"
)

// instanceSocketPath 返回单实例 socket 路径，优先使用用户私有的运行时目录。
func instanceSocketPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, fmt.Sprintf("stream-player-%d.sock", os.Getuid()))
}

// listenInstance 让程序在 Linux 上保持单实例：已有实例时通知它显示窗口并返回 true；
// 否则监听 socket，收到通知后调用 onShow。
// 这是没有托盘宿主时的兜底（例如 GNOME 未安装 AppIndicator 扩展），
// 窗口隐藏后重新启动程序即可唤回窗口，不会出现无法找回的死局。
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
			onShow()
		}
	}()

	return false, func() {
		_ = ln.Close()
		_ = os.Remove(path)
	}
}
