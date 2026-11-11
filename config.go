package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	proxyProtocolHTTP   = "http"
	proxyProtocolSOCKS5 = "socks5"
)

// Config 是持久化到用户配置目录的全部设置。
type Config struct {
	URL           string `json:"url"`
	UseProxy      bool   `json:"use_proxy"`
	ProxyProtocol string `json:"proxy_protocol"`
	ProxyHost     string `json:"proxy_host"`
	ProxyPort     string `json:"proxy_port"`
	ProxyUsername string `json:"proxy_username"`
	ProxyPassword string `json:"proxy_password"`
	Player        string `json:"player"`
	// StreamlinkPath 为空时按平台候选路径和 PATH 自动查找。
	StreamlinkPath string `json:"streamlink_path"`
}

func defaultConfig() Config {
	players := playersForPlatform()
	return Config{
		ProxyProtocol: proxyProtocolHTTP,
		Player:        players[0].ID,
	}
}

// configPath 返回配置文件的绝对路径：
// macOS 为 ~/Library/Application Support/stream-player/config.json，
// Linux 为 ~/.config/stream-player/config.json，
// Windows 为 %APPDATA%\stream-player\config.json。
func configPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("无法定位用户配置目录：%w", err)
	}
	return filepath.Join(base, "stream-player", "config.json"), nil
}

// loadConfig 读取配置；文件不存在时返回默认值，解析失败时返回默认值和一个可读的错误。
func loadConfig() (Config, error) {
	cfg := defaultConfig()
	path, err := configPath()
	if err != nil {
		return cfg, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("读取配置文件 %s 失败：%w", path, err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaultConfig(), fmt.Errorf("配置文件 %s 格式错误，已使用默认设置：%w", path, err)
	}
	if cfg.ProxyProtocol != proxyProtocolSOCKS5 {
		cfg.ProxyProtocol = proxyProtocolHTTP
	}
	return cfg, nil
}

func saveConfig(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("创建配置目录失败：%w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败：%w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("写入配置文件 %s 失败：%w", path, err)
	}
	return nil
}
