//go:build !linux

package main

// listenInstance 在非 Linux 平台不做单实例处理。
func listenInstance(func()) (alreadyRunning bool, stop func()) {
	return false, func() {}
}
