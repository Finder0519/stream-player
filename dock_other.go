//go:build !darwin

package main

// installDockReopen 在非 macOS 平台没有 Dock 唤回窗口的行为，保留空实现。
func installDockReopen(func()) {}
