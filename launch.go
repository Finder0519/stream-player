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
	ffmpegBinary     = "ffmpeg"
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
		return "", errors.New(t(keyURLRequired))
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf(tf(keyURLParse), err)
	}
	if u.Scheme == "" {
		raw = "https://" + raw
		u, err = url.Parse(raw)
		if err != nil {
			return "", fmt.Errorf(tf(keyURLParse), err)
		}
	}
	if u.Host == "" {
		return "", fmt.Errorf(tf(keyURLNoHost), raw)
	}
	return raw, nil
}

func buildProxyConfig(cfg Config) (*ProxyConfig, error) {
	host := strings.TrimSpace(cfg.ProxyHost)
	port := strings.TrimSpace(cfg.ProxyPort)
	username := strings.TrimSpace(cfg.ProxyUsername)

	if host == "" {
		return nil, errors.New(t(keyProxyHostMissing))
	}
	if port == "" {
		return nil, errors.New(t(keyProxyPortMissing))
	}
	portNum, err := strconv.Atoi(port)
	if err != nil || portNum < 1 || portNum > 65535 {
		return nil, fmt.Errorf(tf(keyProxyPortInvalid), port)
	}
	if strings.Contains(host, " ") {
		return nil, fmt.Errorf(tf(keyProxyHostInvalid), host)
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

// programFilesDir 返回 Windows 的 Program Files 目录（%ProgramFiles%），
// 环境变量缺失时退回默认路径。
func programFilesDir() string {
	if dir := os.Getenv("ProgramFiles"); dir != "" {
		return dir
	}
	return `C:\Program Files`
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
	case "windows":
		// Windows 官方安装包默认装在该目录；PATH 优先，这里只作兜底。
		return []string{filepath.Join(programFilesDir(), "Streamlink", "bin", "streamlink.exe")}
	default:
		return nil
	}
	if home != "" {
		candidates = append(candidates, filepath.Join(home, ".local/bin/streamlink"))
	}
	return candidates
}

// ffmpegCandidates 列出各平台常见的 ffmpeg 安装位置，原因同 streamlinkCandidates。
func ffmpegCandidates() []string {
	home, _ := os.UserHomeDir()

	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{"/opt/homebrew/bin/ffmpeg", "/usr/local/bin/ffmpeg"}
	case "linux":
		candidates = []string{"/usr/bin/ffmpeg", "/usr/local/bin/ffmpeg", "/snap/bin/ffmpeg"}
	case "windows":
		// Windows 官方安装包自带的 ffmpeg；PATH 优先，这里只作兜底。
		return []string{filepath.Join(programFilesDir(), "Streamlink", "ffmpeg", "ffmpeg.exe")}
	default:
		return nil
	}
	if home != "" {
		candidates = append(candidates, filepath.Join(home, ".local/bin/ffmpeg"))
	}
	return candidates
}

// findExecutable 查找可执行文件。Windows 上 PATH（环境变量）优先，找不到再试
// 固定安装路径；其它平台顺序相反：先试固定安装路径，再回退到 PATH。
func findExecutable(name string, candidates []string, errKey string) (string, error) {
	lookPath := func() (string, bool) {
		path, err := exec.LookPath(name)
		return path, err == nil
	}
	findCandidate := func() (string, bool) {
		for _, candidate := range candidates {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, true
			}
		}
		return "", false
	}

	if runtime.GOOS == "windows" {
		if path, ok := lookPath(); ok {
			return path, nil
		}
		if path, ok := findCandidate(); ok {
			return path, nil
		}
	} else {
		if path, ok := findCandidate(); ok {
			return path, nil
		}
		if path, ok := lookPath(); ok {
			return path, nil
		}
	}

	checked := append(candidates, t(keyPathEntry, name, os.Getenv("PATH")))
	return "", errors.New(t(errKey, strings.Join(checked, "\n  ")))
}

// findStreamlink 按平台约定顺序查找 streamlink。
func findStreamlink() (string, error) {
	return findExecutable(streamlinkBinary, streamlinkCandidates(), keyLinkNotFound)
}

// findFFmpeg 按平台约定顺序查找 ffmpeg。
// 启动参数包含 --ffmpeg-copyts，缺少 ffmpeg 时 streamlink 无法正常播放。
func findFFmpeg() (string, error) {
	return findExecutable(ffmpegBinary, ffmpegCandidates(), keyFFmpegNotFound)
}

func buildStreamlinkArgs(streamURL string, player Player, playerPath, ffmpegPath string, proxy *ProxyConfig) []string {
	args := []string{
		"--player", playerPath,
		"--player-args", player.Args,
		// 显式指定 ffmpeg：streamlink 自己只按 PATH 查找，从 Finder 启动的 .app
		// 不继承终端的 PATH，找不到 ffmpeg 时无法合并分离的音视频轨（无声音）。
		"--ffmpeg-ffmpeg", ffmpegPath,
		"--http-header", "User-Agent=" + userAgent,
		"--ffmpeg-copyts",
		"--stream-segment-threads", "5",
		"--hls-live-edge", "5",
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
		return errors.New(t(keyNotRunning))
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
		return nil, fmt.Errorf(tf(keyStartFailed), err)
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
