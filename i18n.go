package main

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2/lang"
)

// 支持的语言代码，也是 config.json 中 language 字段的取值。
const (
	langZh = "zh"
	langEn = "en"
)

// 语言下拉框中显示的名称。
const (
	langNameZh = "简体中文"
	langNameEn = "English"
)

// currentLang 是当前界面语言。程序只有一个窗口，用包级变量即可。
var currentLang = langZh

// defaultLanguage 按系统语言给出首次启动的默认界面语言：
// 中文系统使用简体中文，其余使用英文。
func defaultLanguage() string {
	if strings.HasPrefix(strings.ToLower(string(lang.SystemLocale())), "zh") {
		return langZh
	}
	return langEn
}

func normalizeLanguage(code string) string {
	if code == langEn {
		return langEn
	}
	return langZh
}

func setLanguage(code string) {
	currentLang = normalizeLanguage(code)
}

func languageName(code string) string {
	if normalizeLanguage(code) == langEn {
		return langNameEn
	}
	return langNameZh
}

func languageFromName(name string) string {
	if name == langNameEn {
		return langEn
	}
	return langZh
}

// 界面文案的键。
const (
	keyAppTitle = "app.title"

	keyLabelURL          = "label.url"
	keyLabelPlayer       = "label.player"
	keyLabelStreamlink   = "label.streamlink_path"
	keyLabelFFmpeg       = "label.ffmpeg_path"
	keyLabelProtocol     = "label.protocol"
	keyLabelHostPort     = "label.host_port"
	keyLabelUserPass     = "label.user_pass"
	keyProxyEnable       = "label.enable_proxy"
	keyPlaceholderOpt    = "placeholder.optional"
	keyPlaceholderLink   = "placeholder.streamlink"
	keyPlaceholderFFmpeg = "placeholder.ffmpeg"
	keyFixedParams       = "label.fixed_params"
	keyLogTitle          = "label.log_title"

	keyBtnLaunch = "button.launch"
	keyBtnStop   = "button.stop"

	keyStatusIdle             = "status.idle"
	keyStatusRunning          = "status.running"
	keyStatusConfigSaveFailed = "status.config_save_failed"
	keyStatusStopped          = "status.stopped"
	keyStatusFailed           = "status.failed"
	keyStatusFinished         = "status.finished"
	keyStatusStopping         = "status.stopping"
	keyStatusError            = "status.error"
	keyConfigInfo             = "label.config_info"

	keyTrayOpen = "tray.open"
	keyTrayStop = "tray.stop"
	keyTrayQuit = "tray.quit"

	keyDialogClose     = "dialog.close"
	keyFailTitleStart  = "fail.title.start"
	keyFailTitleProxy  = "fail.title.proxy"
	keyFailTitleLink   = "fail.title.streamlink"
	keyFailTitleFFmpeg = "fail.title.ffmpeg"
	keyFailTitlePlayer = "fail.title.player"
	keyFailTitleLaunch = "fail.title.launch"
	keyFailTitlePlay   = "fail.title.playback"
	keyFailTitleStop   = "fail.title.stop"
	keySelectPlayer    = "fail.select_player"

	keyCfgDir     = "config.dir"
	keyCfgRead    = "config.read"
	keyCfgParse   = "config.parse"
	keyCfgMkdir   = "config.mkdir"
	keyCfgMarshal = "config.marshal"
	keyCfgWrite   = "config.write"

	keyURLRequired       = "launch.url_required"
	keyURLParse          = "launch.url_parse"
	keyURLNoHost         = "launch.url_no_host"
	keyProxyHostMissing  = "launch.proxy_host_missing"
	keyProxyPortMissing  = "launch.proxy_port_missing"
	keyProxyPortInvalid  = "launch.proxy_port_invalid"
	keyProxyHostInvalid  = "launch.proxy_host_invalid"
	keyLinkPathInvalid   = "launch.streamlink_path_invalid"
	keyFFmpegPathInvalid = "launch.ffmpeg_path_invalid"
	keyPathEntry         = "launch.path_entry"
	keyLinkNotFound      = "launch.streamlink_not_found"
	keyFFmpegNotFound    = "launch.ffmpeg_not_found"
	keyNotInPath         = "launch.not_in_path"
	keyIsDirectory       = "launch.is_directory"
	keyNotRunning        = "launch.not_running"
	keyStartFailed       = "launch.start_failed"
	keyPlayerNotFound    = "player.not_found"
)

// messages 保存每条界面文案的简体中文和英文版本：[0] 中文，[1] 英文。
var messages = map[string][2]string{
	keyAppTitle: {`Streamlink 播放器`, `Streamlink Player`},

	keyLabelURL:          {`播放地址`, `Playback URL`},
	keyLabelPlayer:       {`播放器`, `Player`},
	keyLabelStreamlink:   {`streamlink 路径`, `Streamlink path`},
	keyLabelFFmpeg:       {`ffmpeg 路径`, `FFmpeg path`},
	keyLabelProtocol:     {`协议`, `Protocol`},
	keyLabelHostPort:     {`主机 / 端口`, `Host / Port`},
	keyLabelUserPass:     {`用户名 / 密码`, `Username / Password`},
	keyProxyEnable:       {`启用代理`, `Use proxy`},
	keyPlaceholderOpt:    {`可选`, `Optional`},
	keyPlaceholderLink:   {`留空自动检测，例如 /opt/homebrew/bin/streamlink`, `Leave empty to auto-detect, e.g. /opt/homebrew/bin/streamlink`},
	keyPlaceholderFFmpeg: {`留空自动检测，例如 /opt/homebrew/bin/ffmpeg`, `Leave empty to auto-detect, e.g. /opt/homebrew/bin/ffmpeg`},
	keyFixedParams:       {`固定参数：质量 %s，使用固定 Chrome User-Agent。`, `Fixed options: quality %s, fixed Chrome User-Agent.`},
	keyLogTitle:          {`streamlink 输出`, `streamlink output`},

	keyBtnLaunch: {`启动播放`, `Start Playback`},
	keyBtnStop:   {`停止`, `Stop`},

	keyStatusIdle:             {`待启动`, `Not started`},
	keyStatusRunning:          {`已启动 streamlink，播放器：%s。可点击「停止」结束本次播放。`, `streamlink started. Player: %s. Click "Stop" to end playback.`},
	keyStatusConfigSaveFailed: {`（配置保存失败：%s）`, ` (failed to save config: %s)`},
	keyStatusStopped:          {`已停止播放。`, `Playback stopped.`},
	keyStatusFailed:           {`streamlink 已退出，播放失败。`, `streamlink exited; playback failed.`},
	keyStatusFinished:         {`播放结束。`, `Playback finished.`},
	keyStatusStopping:         {`正在停止 streamlink…`, `Stopping streamlink…`},
	keyStatusError:            {`%s：%s`, `%s: %s`},
	keyConfigInfo:             {`配置文件：%s`, `Config file: %s`},

	keyTrayOpen: {`打开窗口`, `Open Window`},
	keyTrayStop: {`停止播放`, `Stop Playback`},
	keyTrayQuit: {`退出`, `Quit`},

	keyDialogClose:     {`关闭`, `Close`},
	keyFailTitleStart:  {`无法启动`, `Unable to Start`},
	keyFailTitleProxy:  {`代理配置有误`, `Invalid Proxy Settings`},
	keyFailTitleLink:   {`缺少 streamlink`, `streamlink Not Found`},
	keyFailTitleFFmpeg: {`缺少 ffmpeg`, `ffmpeg Not Found`},
	keyFailTitlePlayer: {`缺少播放器`, `Player Not Found`},
	keyFailTitleLaunch: {`启动失败`, `Failed to Start`},
	keyFailTitlePlay:   {`播放失败`, `Playback Failed`},
	keyFailTitleStop:   {`停止失败`, `Failed to Stop`},
	keySelectPlayer:    {`请选择播放器。`, `Please select a player.`},

	keyCfgDir:     {`无法定位用户配置目录：%w`, `Cannot locate the user config directory: %w`},
	keyCfgRead:    {`读取配置文件 %s 失败：%w`, `Failed to read config file %s: %w`},
	keyCfgParse:   {`配置文件 %s 格式错误，已使用默认设置：%w`, `Config file %s is malformed; default settings are used: %w`},
	keyCfgMkdir:   {`创建配置目录失败：%w`, `Failed to create the config directory: %w`},
	keyCfgMarshal: {`序列化配置失败：%w`, `Failed to encode config: %w`},
	keyCfgWrite:   {`写入配置文件 %s 失败：%w`, `Failed to write config file %s: %w`},

	keyURLRequired:       {`请输入要播放的 URL。`, `Please enter a URL to play.`},
	keyURLParse:          {`URL 无法解析：%v`, `Cannot parse the URL: %v`},
	keyURLNoHost:         {`URL 缺少主机名：%s`, `The URL has no host name: %s`},
	keyProxyHostMissing:  {`已启用代理，但未填写代理主机（host）。`, `Proxy is enabled, but the proxy host is empty.`},
	keyProxyPortMissing:  {`已启用代理，但未填写代理端口。`, `Proxy is enabled, but the proxy port is empty.`},
	keyProxyPortInvalid:  {`代理端口无效：%q，应为 1-65535 之间的数字。`, `Invalid proxy port: %q; expected a number between 1 and 65535.`},
	keyProxyHostInvalid:  {`代理主机格式无效：%q。`, `Invalid proxy host: %q.`},
	keyLinkPathInvalid:   {`指定的 streamlink 路径不可用：%s\n%v`, `The specified streamlink path is not usable: %s\n%v`},
	keyFFmpegPathInvalid: {`指定的 ffmpeg 路径不可用：%s\n%v`, `The specified ffmpeg path is not usable: %s\n%v`},
	keyPathEntry:         {`PATH 中的 %s（当前 PATH：%s）`, `%s in PATH (current PATH: %s)`},
	keyLinkNotFound: {`未找到 streamlink 可执行文件。已检查：\n  %s\n\n从 Finder/Dock 启动的应用不会继承终端的 PATH，Homebrew 安装的 streamlink 可能因此找不到。` +
		`\n可在界面的「streamlink 路径」中填写可执行文件绝对路径，或先安装：\n` +
		`  Windows: pip install -U streamlink（或使用官方安装包）\n` +
		`  macOS:   brew install streamlink\n` +
		`  Linux:   发行版包管理器，或 pipx install streamlink`,
		`streamlink executable not found. Checked:\n  %s\n\n` +
			`Apps launched from Finder/Dock do not inherit the terminal PATH, so a Homebrew-installed streamlink may not be found.` +
			`\nEnter the absolute path in the "Streamlink path" field, or install it first:\n` +
			`  Windows: pip install -U streamlink (or use the official installer)\n` +
			`  macOS:   brew install streamlink\n` +
			`  Linux:   your distribution package manager, or pipx install streamlink`},
	keyFFmpegNotFound: {`未找到 ffmpeg 可执行文件。启动参数 --ffmpeg-copyts 依赖 ffmpeg，请先安装并确保它在 PATH 中。已检查：\n  %s\n` +
		`可在界面的「ffmpeg 路径」中填写可执行文件绝对路径。\n\n安装方式：\n` +
		`  Windows: winget install ffmpeg（或从 https://ffmpeg.org 下载后把 bin 目录加入 PATH）\n` +
		`  macOS:   brew install ffmpeg\n` +
		`  Linux:   发行版包管理器，如 sudo apt install ffmpeg`,
		`ffmpeg executable not found. The --ffmpeg-copyts option requires ffmpeg; install it and make sure it is in PATH. Checked:\n  %s\n` +
			`Enter the absolute path in the "FFmpeg path" field.\n\nInstallation:\n` +
			`  Windows: winget install ffmpeg (or download from https://ffmpeg.org and add its bin directory to PATH)\n` +
			`  macOS:   brew install ffmpeg\n` +
			`  Linux:   your distribution package manager, e.g. sudo apt install ffmpeg`},
	keyNotInPath:   {`未在 PATH 中找到 %q`, `%q not found in PATH`},
	keyIsDirectory: {`这是一个目录，不是可执行文件`, `This is a directory, not an executable file`},
	keyNotRunning:  {`streamlink 未在运行`, `streamlink is not running`},
	keyStartFailed: {`启动 streamlink 失败：%w`, `Failed to start streamlink: %w`},
	keyPlayerNotFound: {`未找到播放器 %s。已检查以下位置：\n  %s\n\n请安装该播放器，或在界面中选择其它播放器。`,
		`Player %s not found. Checked the following locations:\n  %s\n\nInstall the player, or select a different one in the UI.`},
}

// tf 返回当前语言的文案模板，供 fmt.Errorf 等需要保留格式动词的场合使用。
func tf(key string) string {
	msg, ok := messages[key]
	if !ok {
		return key
	}
	if currentLang == langEn {
		return msg[1]
	}
	return msg[0]
}

// t 返回当前语言的界面文案，args 按模板格式化。
func t(key string, args ...any) string {
	format := tf(key)
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}
