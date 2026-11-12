package main

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"

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
	logTitle    *widget.Label
	logGrid     *widget.TextGrid
	logScroll   *container.Scroll

	proc *runningProcess

	// runningPlayer 记录当前拉流使用的播放器名称，切换语言时用来重建状态文案。
	runningPlayer string

	// header、表单与固定参数说明需要随语言切换就地更新文字。
	header    *widget.Label
	urlForm   *widget.Form
	proxyForm *widget.Form
	fixed     *widget.Label

	proxyFields []fyne.Disableable
}

func newUI(w fyne.Window) *launcherUI {
	// 先用系统语言兜底，读到配置后再按用户保存的选择覆盖。
	setLanguage(defaultLanguage())
	cfg, loadErr := loadConfig()
	setLanguage(cfg.Language)

	u := &launcherUI{
		win:     w,
		players: playersForPlatform(),
	}

	u.urlEntry = widget.NewEntry()
	u.urlEntry.SetPlaceHolder("https://example.com/live")

	u.proxyCheck = widget.NewCheck("", nil)

	u.protocol = widget.NewSelect([]string{protocolLabelHTTP, protocolLabelSOCKS5}, nil)
	u.hostEntry = widget.NewEntry()
	u.hostEntry.SetPlaceHolder("127.0.0.1")
	u.portEntry = widget.NewEntry()
	u.portEntry.SetPlaceHolder("7890")
	u.userEntry = widget.NewEntry()
	u.passEntry = widget.NewPasswordEntry()

	u.proxyFields = []fyne.Disableable{u.protocol, u.hostEntry, u.portEntry, u.userEntry, u.passEntry}
	u.proxyCheck.OnChanged = u.setProxyEnabled

	names := make([]string, len(u.players))
	for i, p := range u.players {
		names[i] = p.Name
	}
	u.playerSel = widget.NewSelect(names, nil)

	u.linkEntry = widget.NewEntry()
	u.ffmpegEntry = widget.NewEntry()

	u.launchBtn = widget.NewButton("", u.launch)
	u.launchBtn.Importance = widget.HighImportance

	u.stopBtn = widget.NewButton("", u.stop)
	u.stopBtn.Importance = widget.DangerImportance
	u.stopBtn.Disable()

	u.status = widget.NewLabel(t(keyStatusIdle))
	u.status.Wrapping = fyne.TextWrapWord

	u.configInfo = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
	u.configInfo.Wrapping = fyne.TextWrapWord

	u.applyConfig(cfg)
	if loadErr != nil {
		log.Printf("配置加载失败：%v", loadErr)
		u.setStatus(loadErr.Error())
	}

	return u
}

// setupTray 启用系统托盘。关闭窗口只隐藏窗口并保存配置，拉流在后台继续；
// macOS 上单击菜单栏图标直接展开菜单，Windows/Linux 上单击图标显示窗口。
func (u *launcherUI) setupTray(a fyne.App, w fyne.Window) {
	da, ok := a.(desktop.App)
	if !ok {
		return
	}
	u.app = a
	u.tray = da

	da.SetSystemTrayIcon(fyne.NewStaticResource("icon.png", trayIconPNG))
	// SetSystemTrayWindow 会把左键绑定为显示窗口，同时把关闭行为改成隐藏。
	// Fyne 没有提供「单击只弹菜单」的开关，macOS 上只能不调用它：
	// 不设置托盘窗口时 systray 的单击默认展开菜单。关闭拦截改为下面自行设置。
	if runtime.GOOS != "darwin" {
		da.SetSystemTrayWindow(w)
	}
	w.SetCloseIntercept(func() {
		_ = saveConfig(u.snapshot())
		w.Hide()
	})

	// macOS 上窗口隐藏后点击程序坞图标同样唤回窗口，与托盘菜单里的「打开窗口」一致。
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

	stopItem := fyne.NewMenuItem(t(keyTrayStop), u.stop)
	stopItem.Disabled = u.proc == nil

	quitItem := fyne.NewMenuItem(t(keyTrayQuit), u.quit)
	quitItem.IsQuit = true // 告诉 Fyne 这就是退出项，避免再追加一个英文 Quit

	u.tray.SetSystemTrayMenu(fyne.NewMenu(t(keyAppTitle),
		fyne.NewMenuItem(t(keyTrayOpen), u.showWindow),
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
	u.header = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	u.urlForm = widget.NewForm(
		widget.NewFormItem("", u.urlEntry),
		widget.NewFormItem("", u.playerSel),
		widget.NewFormItem("", u.linkEntry),
		widget.NewFormItem("", u.ffmpegEntry),
	)
	// 主机与端口、用户名与密码各占一行，减少表单高度占用。
	u.proxyForm = widget.NewForm(
		widget.NewFormItem("", u.protocol),
		widget.NewFormItem("", container.NewGridWithColumns(2, u.hostEntry, u.portEntry)),
		widget.NewFormItem("", container.NewGridWithColumns(2, u.userEntry, u.passEntry)),
	)

	u.fixed = widget.NewLabel("")
	u.fixed.Wrapping = fyne.TextWrapWord

	u.logTitle = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	u.logGrid = widget.NewTextGrid()
	// 关闭 TextGrid 自带滚动，由外层 Scroll 统一滚动，便于按需自动跟随末尾。
	u.logGrid.Scroll = fyne.ScrollNone
	u.logScroll = container.NewScroll(u.logGrid)
	u.logScroll.SetMinSize(fyne.NewSize(200, 120))

	langSel := widget.NewSelect([]string{langNameZh, langNameEn}, func(name string) {
		u.switchLanguage(languageFromName(name))
	})
	langSel.SetSelected(languageName(currentLang))

	u.applyTexts()

	form := container.NewVBox(
		container.NewBorder(nil, nil, nil, langSel, u.header),
		u.urlForm,
		widget.NewSeparator(),
		u.proxyCheck,
		u.proxyForm,
		widget.NewSeparator(),
		u.fixed,
		container.NewGridWithColumns(2, u.launchBtn, u.stopBtn),
		u.status,
		u.configInfo,
	)
	// 日志占用窗口剩余高度，固定在界面最下方。
	logPane := container.NewBorder(
		container.NewVBox(widget.NewSeparator(), u.logTitle),
		nil, nil, nil,
		u.logScroll,
	)
	return container.NewPadded(container.NewBorder(form, nil, nil, nil, logPane))
}

// applyTexts 按当前语言更新所有静态界面文字。
func (u *launcherUI) applyTexts() {
	u.header.SetText(t(keyAppTitle))

	u.proxyCheck.Text = t(keyProxyEnable)
	u.proxyCheck.Refresh()
	u.userEntry.SetPlaceHolder(t(keyPlaceholderOpt))
	u.passEntry.SetPlaceHolder(t(keyPlaceholderOpt))
	u.linkEntry.SetPlaceHolder(t(keyPlaceholderLink))
	u.ffmpegEntry.SetPlaceHolder(t(keyPlaceholderFFmpeg))
	u.launchBtn.SetText(t(keyBtnLaunch))
	u.stopBtn.SetText(t(keyBtnStop))

	setFormTexts(u.urlForm, t(keyLabelURL), t(keyLabelPlayer), t(keyLabelStreamlink), t(keyLabelFFmpeg))
	setFormTexts(u.proxyForm, t(keyLabelProtocol), t(keyLabelHostPort), t(keyLabelUserPass))

	u.fixed.SetText(t(keyFixedParams, qualityOptions))
	u.logTitle.SetText(t(keyLogTitle))

	path, err := configPath()
	if err != nil {
		path = err.Error()
	}
	u.configInfo.SetText(t(keyConfigInfo, path))
}

// setFormTexts 依次改写表单各行的标签文字，行数不足时忽略多余文字。
func setFormTexts(form *widget.Form, texts ...string) {
	for i := 0; i < len(form.Items) && i < len(texts); i++ {
		form.Items[i].Text = texts[i]
	}
	form.Refresh()
}

// switchLanguage 切换界面语言：更新界面文字、托盘菜单并保存选择。
func (u *launcherUI) switchLanguage(lang string) {
	if normalizeLanguage(lang) == currentLang {
		return
	}
	setLanguage(lang)
	u.applyTexts()
	u.refreshTrayMenu()

	// 状态栏是动态文案，切换后按当前运行状态重新生成。
	if u.proc != nil {
		u.setStatus(t(keyStatusRunning, u.runningPlayer))
	} else {
		u.setStatus(t(keyStatusIdle))
	}

	if err := saveConfig(u.snapshot()); err != nil {
		log.Printf("配置保存失败：%v", err)
	}
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
		Language:       currentLang,
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
		u.fail(t(keyFailTitleStart), err.Error())
		return
	}

	var proxy *ProxyConfig
	if cfg.UseProxy {
		proxy, err = buildProxyConfig(cfg)
		if err != nil {
			u.fail(t(keyFailTitleProxy), err.Error())
			return
		}
	}

	player, ok := u.selectedPlayer()
	if !ok {
		u.fail(t(keyFailTitleStart), t(keySelectPlayer))
		return
	}

	streamlinkPath, err := findStreamlink(cfg.StreamlinkPath)
	if err != nil {
		u.fail(t(keyFailTitleLink), err.Error())
		return
	}

	ffmpegPath, err := findFFmpeg(cfg.FFmpegPath)
	if err != nil {
		u.fail(t(keyFailTitleFFmpeg), err.Error())
		return
	}

	playerPath, err := player.Resolve()
	if err != nil {
		u.fail(t(keyFailTitlePlayer), err.Error())
		return
	}

	saveErr := saveConfig(cfg)

	proc, err := startStreamlink(streamlinkPath, buildStreamlinkArgs(streamURL, player, playerPath, ffmpegPath, proxy))
	if err != nil {
		u.fail(t(keyFailTitleLaunch), redact(err.Error(), proxy))
		return
	}

	proxyDesc := "未启用"
	if proxy != nil {
		proxyDesc = proxy.redactedURL()
	}
	log.Printf("启动播放：streamlink=%s ffmpeg=%s 播放器=%s（%s） 代理=%s", streamlinkPath, ffmpegPath, player.Name, playerPath, proxyDesc)

	u.proc = proc
	u.runningPlayer = player.Name
	u.logGrid.SetText("")
	u.logScroll.ScrollToTop()
	u.launchBtn.Disable()
	u.stopBtn.Enable()
	u.refreshTrayMenu()
	status := t(keyStatusRunning, player.Name)
	if saveErr != nil {
		log.Printf("配置保存失败：%v", saveErr)
		status += t(keyStatusConfigSaveFailed, saveErr.Error())
	}
	u.setStatus(status)

	logDone := make(chan struct{})
	go u.followLog(proc, proxy, logDone)

	go func() {
		waitErr := proc.cmd.Wait()
		close(logDone)
		output := redact(proc.output.String(), proxy)
		log.Printf("streamlink 已退出：%v\n%s", waitErr, output)

		fyne.Do(func() {
			u.proc = nil
			u.launchBtn.Enable()
			u.stopBtn.Disable()
			u.refreshTrayMenu()
			if proc.stopped.Load() {
				u.setStatus(t(keyStatusStopped))
				return
			}
			if waitErr != nil {
				u.setStatus(t(keyStatusFailed))
				u.fail(t(keyFailTitlePlay), redact(fmt.Sprintf("%v\n\n%s", waitErr, output), proxy))
				return
			}
			u.setStatus(t(keyStatusFinished))
		})
	}()
}

// followLog 周期性把 streamlink 的最新输出同步到界面日志区。
// done 关闭后补一次刷新再退出，保证进程退出前的输出也能显示出来。
func (u *launcherUI) followLog(proc *runningProcess, proxy *ProxyConfig, done <-chan struct{}) {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	var last string
	for {
		select {
		case <-ticker.C:
			text := redact(proc.output.String(), proxy)
			if text != last {
				last = text
				u.setLog(text)
			}
		case <-done:
			u.setLog(redact(proc.output.String(), proxy))
			return
		}
	}
}

// setLog 更新日志区文字；用户已向上滚动查看历史时不再强制跳到底部。
func (u *launcherUI) setLog(text string) {
	fyne.Do(func() {
		atBottom := u.logScroll.Offset.Y+u.logScroll.Size().Height >= u.logScroll.Content.Size().Height-4
		u.logGrid.SetText(text)
		if atBottom {
			u.logScroll.ScrollToBottom()
		}
	})
}

func (u *launcherUI) stop() {
	if u.proc == nil || u.proc.stopped.Load() {
		return
	}
	log.Printf("用户请求停止 streamlink")
	u.stopBtn.Disable()
	u.setStatus(t(keyStatusStopping))
	if err := u.proc.stop(); err != nil {
		u.stopBtn.Enable()
		u.fail(t(keyFailTitleStop), err.Error())
	}
}

func (u *launcherUI) fail(title, message string) {
	log.Printf("错误[%s]：%s", title, message)
	u.setStatus(t(keyStatusError, title, firstLine(message)))

	label := widget.NewLabel(message)
	label.Wrapping = fyne.TextWrapWord

	scroll := container.NewVScroll(label)
	scroll.SetMinSize(fyne.NewSize(440, 150))

	d := dialog.NewCustom(title, t(keyDialogClose), scroll, u.win)
	d.Resize(fyne.NewSize(500, 300))
	d.Show()
}

func firstLine(text string) string {
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		return text[:i]
	}
	return text
}
