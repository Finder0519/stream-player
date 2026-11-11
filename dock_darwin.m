//go:build darwin

#import <AppKit/AppKit.h>
#import <objc/runtime.h>

extern void streamPlayerDockReopen(void);

// 与 -applicationShouldHandleReopen:hasVisibleWindows: 的签名一致（BOOL 的类型编码为 c）。
static BOOL streamPlayerShouldHandleReopen(id self, SEL _cmd, NSApplication *sender, BOOL hasVisibleWindows) {
	streamPlayerDockReopen();
	return YES;
}

// 返回 0 表示当前没有可用的 delegate，1 表示已安装或已存在同类实现。
int streamPlayerInstallDockReopen(void) {
	@autoreleasepool {
		id delegate = [NSApplication sharedApplication].delegate;
		if (delegate == nil) {
			return 0;
		}

		Class cls = object_getClass(delegate);
		SEL sel = @selector(applicationShouldHandleReopen:hasVisibleWindows:);
		if (class_respondsToSelector(cls, sel)) {
			return 1; // Fyne 或 GLFW 以后自带实现时不要覆盖
		}
		if (!class_addMethod(cls, sel, (IMP)streamPlayerShouldHandleReopen, "c@:@c")) {
			return 0;
		}
		return 1;
	}
}
