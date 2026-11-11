//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// applyProcAttr 隐藏 streamlink 子进程的控制台窗口，避免 GUI 下弹出黑框。
func applyProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
