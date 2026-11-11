package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	// 固定 UA，不做配置项。
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"
	// 固定播放质量优先级。
	qualityOptions = "best,720p,480p,worst"

	streamlinkBinary = "streamlink"
)

// ProxyConfig 是已校验的代理设置。
type ProxyConfig struct {
	Protocol string
	Host     string
	Port     string
	Username string
	Password string
}

// proxyURL 生成传给 streamlink 的代理地址，认证信息按 URL 规则转义。
func (p ProxyConfig) proxyURL() string {
	u := url.URL{
		Scheme: p.Protocol,
		Host:   net.JoinHostPort(p.Host, p.Port),
	}
	if p.Username != "" {
		u.User = url.UserPassword(p.Username, p.Password)
	}
	return u.String()
}

// redactedURL 用于出错信息展示，去掉密码。
func (p ProxyConfig) redactedURL() string {
	u := url.URL{
		Scheme: p.Protocol,
		Host:   net.JoinHostPort(p.Host, p.Port),
	}
	if p.Username != "" {
		u.User = url.User(p.Username)
	}
	return u.String()
}

func normalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("请输入要播放的 URL。")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("URL 无法解析：%v", err)
	}
	if u.Scheme == "" {
		raw = "https://" + raw
		u, err = url.Parse(raw)
		if err != nil {
			return "", fmt.Errorf("URL 无法解析：%v", err)
		}
	}
	if u.Host == "" {
		return "", fmt.Errorf("URL 缺少主机名：%s", raw)
	}
	return raw, nil
}

func buildProxyConfig(cfg Config) (*ProxyConfig, error) {
	host := strings.TrimSpace(cfg.ProxyHost)
	port := strings.TrimSpace(cfg.ProxyPort)
	username := strings.TrimSpace(cfg.ProxyUsername)

	if host == "" {
		return nil, errors.New("已启用代理，但未填写代理主机（host）。")
	}
	if port == "" {
		return nil, errors.New("已启用代理，但未填写代理端口。")
	}
	portNum, err := strconv.Atoi(port)
	if err != nil || portNum < 1 || portNum > 65535 {
		return nil, fmt.Errorf("代理端口无效：%q，应为 1-65535 之间的数字。", port)
	}
	if strings.Contains(host, " ") {
		return nil, fmt.Errorf("代理主机格式无效：%q。", host)
	}

	protocol := proxyProtocolHTTP
	if cfg.ProxyProtocol == proxyProtocolSOCKS5 {
		protocol = proxyProtocolSOCKS5
	}

	return &ProxyConfig{
		Protocol: protocol,
		Host:     host,
		Port:     port,
		Username: username,
		Password: cfg.ProxyPassword,
	}, nil
}

// streamlinkCandidates 列出各平台常见的 streamlink 安装位置。
// 从 Finder/Dock 启动的 .app 不会继承终端的 PATH（launchd 默认只有
// /usr/bin:/bin:/usr/sbin:/sbin），因此 Homebrew 等目录需要直接探测。
func streamlinkCandidates() []string {
	home, _ := os.UserHomeDir()

	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{"/opt/homebrew/bin/streamlink", "/usr/local/bin/streamlink"}
	case "linux":
		candidates = []string{"/usr/bin/streamlink", "/usr/local/bin/streamlink", "/snap/bin/streamlink"}
	default:
		return nil
	}
	if home != "" {
		candidates = append(candidates, filepath.Join(home, ".local/bin/streamlink"))
	}
	return candidates
}

// findStreamlink 按“用户指定路径 → 平台常见安装位置 → PATH”的顺序查找 streamlink。
func findStreamlink(custom string) (string, error) {
	custom = strings.TrimSpace(custom)
	if custom != "" {
		path, err := resolveStreamlink(custom)
		if err != nil {
			return "", fmt.Errorf("指定的 streamlink 路径不可用：%s\n%v", custom, err)
		}
		return path, nil
	}

	candidates := streamlinkCandidates()
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	if path, err := exec.LookPath(streamlinkBinary); err == nil {
		return path, nil
	}

	checked := append(candidates, "PATH 中的 "+streamlinkBinary+"（当前 PATH："+os.Getenv("PATH")+"）")
	return "", errors.New("未找到 streamlink 可执行文件。已检查：\n  " +
		strings.Join(checked, "\n  ") +
		"\n\n从 Finder/Dock 启动的应用不会继承终端的 PATH，Homebrew 安装的 streamlink 可能因此找不到。" +
		"\n可在界面的「streamlink 路径」中填写可执行文件绝对路径，或先安装：\n" +
		"  Windows: pip install -U streamlink（或使用官方安装包）\n" +
		"  macOS:   brew install streamlink\n" +
		"  Linux:   发行版包管理器，或 pipx install streamlink")
}

// resolveStreamlink 校验用户指定的路径；允许绝对路径或 PATH 中的可执行文件名。
func resolveStreamlink(custom string) (string, error) {
	if !filepath.IsAbs(custom) {
		path, err := exec.LookPath(custom)
		if err != nil {
			return "", fmt.Errorf("未在 PATH 中找到 %q", custom)
		}
		return path, nil
	}

	info, err := os.Stat(custom)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", errors.New("这是一个目录，不是可执行文件")
	}
	return custom, nil
}

func buildStreamlinkArgs(streamURL string, player Player, playerPath string, proxy *ProxyConfig) []string {
	args := []string{
		"--player", playerPath,
		"--player-args", player.Args,
		"--http-header", "User-Agent=" + userAgent,
	}
	if proxy != nil {
		proxyURL := proxy.proxyURL()
		args = append(args, "--http-proxy", proxyURL)
	}
	args = append(args, streamURL, qualityOptions)
	return args
}

// tailBuffer 只保留最近 max 字节输出，避免长时间播放时无限增长。
type tailBuffer struct {
	mu  sync.Mutex
	max int
	buf []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.max {
		t.buf = t.buf[len(t.buf)-t.max:]
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.TrimSpace(string(t.buf))
}

type runningProcess struct {
	cmd     *exec.Cmd
	output  *tailBuffer
	stopped atomic.Bool
}

// stop 终止 streamlink 进程；streamlink 退出后播放器的 stdin 管道随之关闭，播放器会自行结束。
func (p *runningProcess) stop() error {
	p.stopped.Store(true)
	if p.cmd.Process == nil {
		return errors.New("streamlink 未在运行")
	}
	return p.cmd.Process.Kill()
}

func startStreamlink(streamlinkPath string, args []string) (*runningProcess, error) {
	cmd := exec.Command(streamlinkPath, args...)
	output := &tailBuffer{max: 8192}
	cmd.Stdout = output
	cmd.Stderr = output
	applyProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 streamlink 失败：%w", err)
	}
	return &runningProcess{cmd: cmd, output: output}, nil
}

// redact 从文本中去掉代理密码，避免凭据出现在界面错误信息里。
func redact(text string, proxy *ProxyConfig) string {
	if proxy == nil {
		return text
	}
	if proxy.Password != "" {
		text = strings.ReplaceAll(text, proxy.Password, "***")
	}
	text = strings.ReplaceAll(text, proxy.proxyURL(), proxy.redactedURL())
	return text
}
