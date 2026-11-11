package main

import (
	"fmt"
	"log"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

const (
	protocolLabelHTTP   = "HTTP"
	protocolLabelSOCKS5 = "SOCKS5"
)

type launcherUI struct {
	win fyne.Window

	app  fyne.App
	tray desktop.App

	players []Player

	urlEntry    *widget.Entry
	proxyCheck  *widget.Check
	protocol    *widget.Select
	hostEntry   *widget.Entry
	portEntry   *widget.Entry
	userEntry   *widget.Entry
	passEntry   *widget.Entry
	playerSel   *widget.Select
	linkEntry   *widget.Entry
	ffmpegEntry *widget.Entry
	launchBtn   *widget.Button
	stopBtn     *widget.Button
	status      *widget.Label
	configInfo  *widget.Label

	proc *runningProcess

	proxyFields []fyne.Disableable
}

func newUI(w fyne.Window) *launcherUI {
	cfg, loadErr := loadConfig()

	u := &launcherUI{
		win:     w,
		players: playersForPlatform(),
	}

	u.urlEntry = widget.NewEntry()
	u.urlEntry.SetPlaceHolder("https://example.com/live")

	u.proxyCheck = widget.NewCheck("启用代理", nil)

	u.protocol = widget.NewSelect([]string{protocolLabelHTTP, protocolLabelSOCKS5}, nil)
	u.hostEntry = widget.NewEntry()
	u.hostEntry.SetPlaceHolder("127.0.0.1")
	u.portEntry = widget.NewEntry()
	u.portEntry.SetPlaceHolder("7890")
	u.userEntry = widget.NewEntry()
	u.userEntry.SetPlaceHolder("可选")
	u.passEntry = widget.NewPasswordEntry()
	u.passEntry.SetPlaceHolder("可选")

	u.proxyFields = []fyne.Disableable{u.protocol, u.hostEntry, u.portEntry, u.userEntry, u.passEntry}
	u.proxyCheck.OnChanged = u.setProxyEnabled

	names := make([]string, len(u.players))
	for i, p := range u.players {
		names[i] = p.Name
	}
	u.playerSel = widget.NewSelect(names, nil)

	u.linkEntry = widget.NewEntry()
	u.linkEntry.SetPlaceHolder("留空自动检测，例如 /opt/homebrew/bin/streamlink")

	u.ffmpegEntry = widget.NewEntry()
	u.ffmpegEntry.SetPlaceHolder("留空自动检测，例如 /opt/homebrew/bin/ffmpeg")

	u.launchBtn = widget.NewButton("启动播放", u.launch)
	u.launchBtn.Importance = widget.HighImportance

	u.stopBtn = widget.NewButton("停止", u.stop)
	u.stopBtn.Importance = widget.DangerImportance
	u.stopBtn.Disable()

	u.status = widget.NewLabel("待启动")
	u.status.Wrapping = fyne.TextWrapWord

	path, err := configPath()
	if err != nil {
		path = err.Error()
	}
	u.configInfo = widget.NewLabelWithStyle("配置文件："+path, fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
	u.configInfo.Wrapping = fyne.TextWrapWord

	u.applyConfig(cfg)
	if loadErr != nil {
		log.Printf("配置加载失败：%v", loadErr)
		u.setStatus(loadErr.Error())
	}

	return u
}

// setupTray 启用系统托盘。关闭窗口只隐藏窗口并保存配置，拉流在后台继续；
// 左键点击托盘图标显示窗口，右键弹出菜单。
func (u *launcherUI) setupTray(a fyne.App, w fyne.Window) {
	da, ok := a.(desktop.App)
	if !ok {
		return
	}
	u.app = a
	u.tray = da

	da.SetSystemTrayIcon(fyne.NewStaticResource("icon.png", trayIconPNG))
	// SetSystemTrayWindow 把左键绑定为显示窗口、右键绑定为弹出菜单，
	// 同时会把关闭行为改成隐藏，因此这里重新设置以保留关闭时的配置保存。
	da.SetSystemTrayWindow(w)
	w.SetCloseIntercept(func() {
		_ = saveConfig(u.snapshot())
		w.Hide()
	})

	// macOS 上窗口隐藏后点击程序坞图标同样唤回窗口，与左键点击托盘图标一致。
	installDockReopen(u.showWindow)

	u.refreshTrayMenu()

	// 无论从哪个入口退出（托盘菜单、信号等），都先结束 streamlink，
	// 避免它和播放器被 launchd 收养后继续在后台拉流。
	a.Lifecycle().SetOnStopped(u.killRunning)
}

// refreshTrayMenu 按当前播放状态重建托盘菜单，「停止播放」仅在拉流时可用。
func (u *launcherUI) refreshTrayMenu() {
	if u.tray == nil {
		return
	}

	stopItem := fyne.NewMenuItem("停止播放", u.stop)
	stopItem.Disabled = u.proc == nil

	quitItem := fyne.NewMenuItem("退出", u.quit)
	quitItem.IsQuit = true // 告诉 Fyne 这就是退出项，避免再追加一个英文 Quit

	u.tray.SetSystemTrayMenu(fyne.NewMenu("Stream Player",
		fyne.NewMenuItem("打开窗口", u.showWindow),
		stopItem,
		fyne.NewMenuItemSeparator(),
		quitItem,
	))
}

func (u *launcherUI) showWindow() {
	u.win.Show()
	u.win.RequestFocus()
}

// quit 结束拉流、保存配置后退出程序。
func (u *launcherUI) quit() {
	if u.proc != nil {
		_ = u.proc.stop()
	}
	_ = saveConfig(u.snapshot())
	if u.app != nil {
		u.app.Quit()
	}
}

// killRunning 是退出时的兜底清理，由 Fyne 生命周期回调触发。
func (u *launcherUI) killRunning() {
	if u.proc != nil {
		_ = u.proc.stop()
	}
}

func (u *launcherUI) content() fyne.CanvasObject {
	header := widget.NewLabelWithStyle("Streamlink 播放器", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	urlForm := widget.NewForm(
		widget.NewFormItem("播放地址", u.urlEntry),
		widget.NewFormItem("播放器", u.playerSel),
		widget.NewFormItem("streamlink 路径", u.linkEntry),
		widget.NewFormItem("ffmpeg 路径", u.ffmpegEntry),
	)
	proxyForm := widget.NewForm(
		widget.NewFormItem("协议", u.protocol),
		widget.NewFormItem("主机", u.hostEntry),
		widget.NewFormItem("端口", u.portEntry),
		widget.NewFormItem("用户名", u.userEntry),
		widget.NewFormItem("密码", u.passEntry),
	)

	fixed := widget.NewLabel("固定参数：质量 " + qualityOptions + "，使用固定 Chrome User-Agent。")
	fixed.Wrapping = fyne.TextWrapWord

	body := container.NewVBox(
		header,
		urlForm,
		widget.NewSeparator(),
		u.proxyCheck,
		proxyForm,
		widget.NewSeparator(),
		fixed,
		container.NewGridWithColumns(2, u.launchBtn, u.stopBtn),
		u.status,
		widget.NewSeparator(),
		u.configInfo,
	)
	return container.NewPadded(body)
}

func (u *launcherUI) applyConfig(cfg Config) {
	u.urlEntry.SetText(cfg.URL)
	u.hostEntry.SetText(cfg.ProxyHost)
	u.portEntry.SetText(cfg.ProxyPort)
	u.userEntry.SetText(cfg.ProxyUsername)
	u.passEntry.SetText(cfg.ProxyPassword)
	u.linkEntry.SetText(cfg.StreamlinkPath)
	u.ffmpegEntry.SetText(cfg.FFmpegPath)

	if cfg.ProxyProtocol == proxyProtocolSOCKS5 {
		u.protocol.SetSelected(protocolLabelSOCKS5)
	} else {
		u.protocol.SetSelected(protocolLabelHTTP)
	}

	selected := 0
	for i, p := range u.players {
		if p.ID == cfg.Player {
			selected = i
			break
		}
	}
	u.playerSel.SetSelectedIndex(selected)

	u.proxyCheck.SetChecked(cfg.UseProxy)
	u.setProxyEnabled(cfg.UseProxy)
}

func (u *launcherUI) setProxyEnabled(enabled bool) {
	for _, field := range u.proxyFields {
		if enabled {
			field.Enable()
		} else {
			field.Disable()
		}
	}
}

func (u *launcherUI) selectedPlayer() (Player, bool) {
	for _, p := range u.players {
		if p.Name == u.playerSel.Selected {
			return p, true
		}
	}
	return Player{}, false
}

func (u *launcherUI) snapshot() Config {
	protocol := proxyProtocolHTTP
	if u.protocol.Selected == protocolLabelSOCKS5 {
		protocol = proxyProtocolSOCKS5
	}

	cfg := Config{
		URL:            u.urlEntry.Text,
		UseProxy:       u.proxyCheck.Checked,
		ProxyProtocol:  protocol,
		ProxyHost:      u.hostEntry.Text,
		ProxyPort:      u.portEntry.Text,
		ProxyUsername:  u.userEntry.Text,
		ProxyPassword:  u.passEntry.Text,
		StreamlinkPath: u.linkEntry.Text,
		FFmpegPath:     u.ffmpegEntry.Text,
	}
	if player, ok := u.selectedPlayer(); ok {
		cfg.Player = player.ID
	}
	return cfg
}

func (u *launcherUI) setStatus(text string) {
	u.status.SetText(text)
}

func (u *launcherUI) launch() {
	cfg := u.snapshot()

	streamURL, err := normalizeURL(cfg.URL)
	if err != nil {
		u.fail("无法启动", err.Error())
		return
	}

	var proxy *ProxyConfig
	if cfg.UseProxy {
		proxy, err = buildProxyConfig(cfg)
		if err != nil {
			u.fail("代理配置有误", err.Error())
			return
		}
	}

	player, ok := u.selectedPlayer()
	if !ok {
		u.fail("无法启动", "请选择播放器。")
		return
	}

	streamlinkPath, err := findStreamlink(cfg.StreamlinkPath)
	if err != nil {
		u.fail("缺少 streamlink", err.Error())
		return
	}

	ffmpegPath, err := findFFmpeg(cfg.FFmpegPath)
	if err != nil {
		u.fail("缺少 ffmpeg", err.Error())
		return
	}

	playerPath, err := player.Resolve()
	if err != nil {
		u.fail("缺少播放器", err.Error())
		return
	}

	saveErr := saveConfig(cfg)

	proc, err := startStreamlink(streamlinkPath, buildStreamlinkArgs(streamURL, player, playerPath, ffmpegPath, proxy))
	if err != nil {
		u.fail("启动失败", redact(err.Error(), proxy))
		return
	}

	proxyDesc := "未启用"
	if proxy != nil {
		proxyDesc = proxy.redactedURL()
	}
	log.Printf("启动播放：streamlink=%s ffmpeg=%s 播放器=%s（%s） 代理=%s", streamlinkPath, ffmpegPath, player.Name, playerPath, proxyDesc)

	u.proc = proc
	u.launchBtn.Disable()
	u.stopBtn.Enable()
	u.refreshTrayMenu()
	status := fmt.Sprintf("已启动 streamlink，播放器：%s。可点击「停止」结束本次播放。", player.Name)
	if saveErr != nil {
		log.Printf("配置保存失败：%v", saveErr)
		status += "（配置保存失败：" + saveErr.Error() + "）"
	}
	u.setStatus(status)

	go func() {
		waitErr := proc.cmd.Wait()
		output := redact(proc.output.String(), proxy)
		log.Printf("streamlink 已退出：%v\n%s", waitErr, output)

		fyne.Do(func() {
			u.proc = nil
			u.launchBtn.Enable()
			u.stopBtn.Disable()
			u.refreshTrayMenu()
			if proc.stopped.Load() {
				u.setStatus("已停止播放。")
				return
			}
			if waitErr != nil {
				u.setStatus("streamlink 已退出，播放失败。")
				u.fail("播放失败", redact(fmt.Sprintf("%v\n\n%s", waitErr, output), proxy))
				return
			}
			u.setStatus("播放结束。")
		})
	}()
}

func (u *launcherUI) stop() {
	if u.proc == nil || u.proc.stopped.Load() {
		return
	}
	log.Printf("用户请求停止 streamlink")
	u.stopBtn.Disable()
	u.setStatus("正在停止 streamlink…")
	if err := u.proc.stop(); err != nil {
		u.stopBtn.Enable()
		u.fail("停止失败", err.Error())
	}
}

func (u *launcherUI) fail(title, message string) {
	log.Printf("错误[%s]：%s", title, message)
	u.setStatus(title + "：" + firstLine(message))

	label := widget.NewLabel(message)
	label.Wrapping = fyne.TextWrapWord

	scroll := container.NewVScroll(label)
	scroll.SetMinSize(fyne.NewSize(440, 150))

	d := dialog.NewCustom(title, "关闭", scroll, u.win)
	d.Resize(fyne.NewSize(500, 300))
	d.Show()
}

func firstLine(text string) string {
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		return text[:i]
	}
	return text
}
