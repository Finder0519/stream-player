//go:build windows

package main

import (
	"log"
	"syscall"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
)

const (
	// Local\ 命名空间按登录会话隔离：同一用户重复启动只有一个实例，
	// 同一台机器上的其它登录用户互不影响（配置本来也是按用户存放的）。
	instanceMutexName   = `Local\io.github.stream-player.single-instance`
	showWindowEventName = `Local\io.github.stream-player.show-window`

	eventModifyState = 0x0002 // EVENT_MODIFY_STATE

	// 已有实例可能刚创建互斥体、还没来得及创建事件，因此打开事件时短暂重试。
	showEventAttempts  = 20
	showEventRetryWait = 50 * time.Millisecond
)

var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutexW        = kernel32.NewProc("CreateMutexW")
	procCreateEventW        = kernel32.NewProc("CreateEventW")
	procOpenEventW          = kernel32.NewProc("OpenEventW")
	procSetEvent            = kernel32.NewProc("SetEvent")
	procWaitForSingleObject = kernel32.NewProc("WaitForSingleObject")
)

// listenInstance 在 Windows 上用命名互斥体保证单实例：
// 已有实例时通过命名事件请它显示窗口并返回 true；
// 否则持有互斥体，之后每次收到事件都调用 onShow（在 Fyne 主线程执行）。
func listenInstance(onShow func()) (alreadyRunning bool, stop func()) {
	mutex, existed, err := createMutex(instanceMutexName)
	if err != nil {
		// 互斥体不可用时退回普通启动，行为与旧版本一致，不阻塞用户使用。
		log.Printf("单实例互斥体创建失败，按普通启动继续：%v", err)
		return false, func() {}
	}
	if existed {
		// 互斥体由已有实例持有，本进程只是打开了它，先关闭自己的句柄再通知对方。
		_ = syscall.CloseHandle(mutex)
		if err := signalShowWindow(); err != nil {
			log.Printf("已有实例在运行，但通知其显示窗口失败：%v", err)
		}
		return true, func() {}
	}

	event, err := createShowEvent()
	if err != nil {
		// 仍持有互斥体：不会再出现第二个进程，只是重复启动无法唤回隐藏的窗口。
		log.Printf("单实例事件创建失败，重复启动时将无法唤回窗口：%v", err)
		return false, func() { _ = syscall.CloseHandle(mutex) }
	}

	go func() {
		for {
			r, _, _ := procWaitForSingleObject.Call(uintptr(event), syscall.INFINITE)
			if r != syscall.WAIT_OBJECT_0 {
				return
			}
			fyne.Do(onShow)
		}
	}()

	return false, func() {
		// 关闭事件会唤醒等待中的 goroutine，让它自然退出。
		_ = syscall.CloseHandle(event)
		_ = syscall.CloseHandle(mutex)
	}
}

// createMutex 创建或打开命名互斥体；existed 表示该互斥体已由其它进程持有。
func createMutex(name string) (handle syscall.Handle, existed bool, err error) {
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, false, err
	}

	h, _, callErr := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(namePtr)))
	if h == 0 {
		return 0, false, callErr
	}
	// Go 的 syscall 在每次调用前会把 last error 清零，这里的比较不会读到旧值。
	return syscall.Handle(h), callErr == syscall.ERROR_ALREADY_EXISTS, nil
}

func createShowEvent() (syscall.Handle, error) {
	namePtr, err := syscall.UTF16PtrFromString(showWindowEventName)
	if err != nil {
		return 0, err
	}
	// bManualReset 传 false（自动复位）：每次 SetEvent 只唤醒一次等待，等待结束后自动复位。
	h, _, callErr := procCreateEventW.Call(0, 0, 0, uintptr(unsafe.Pointer(namePtr)))
	if h == 0 {
		return 0, callErr
	}
	return syscall.Handle(h), nil
}

// signalShowWindow 打开已有实例的命名事件并置位，请求它显示窗口。
func signalShowWindow() error {
	namePtr, err := syscall.UTF16PtrFromString(showWindowEventName)
	if err != nil {
		return err
	}

	var (
		handle  uintptr
		callErr error
	)
	for attempt := 0; attempt < showEventAttempts; attempt++ {
		handle, _, callErr = procOpenEventW.Call(eventModifyState, 0, uintptr(unsafe.Pointer(namePtr)))
		if handle != 0 || callErr != syscall.ERROR_FILE_NOT_FOUND {
			break
		}
		time.Sleep(showEventRetryWait)
	}
	if handle == 0 {
		return callErr
	}
	defer syscall.CloseHandle(syscall.Handle(handle))

	r, _, callErr := procSetEvent.Call(handle)
	if r == 0 {
		return callErr
	}
	return nil
}
