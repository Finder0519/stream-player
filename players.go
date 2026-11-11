package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Player 描述一个可选的播放器及其在 streamlink 中的调用方式。
type Player struct {
	ID   string
	Name string
	// Args 作为 streamlink 的 --player-args 传入。
	// {playerinput} 是 streamlink 的占位符，表示把输入源（stdin 管道时为 "-"）放在该位置。
	Args string
	// candidates 依次为安装路径和 PATH 中的可执行文件名，按顺序探测。
	candidates []string
}

// playersForPlatform 按当前运行平台给出播放器候选。
func playersForPlatform() []Player {
	switch runtime.GOOS {
	case "darwin":
		return darwinPlayers()
	case "windows":
		return windowsPlayers()
	default:
		return linuxPlayers()
	}
}

func darwinPlayers() []Player {
	home, _ := os.UserHomeDir()

	iina := []string{"/Applications/IINA.app/Contents/MacOS/iina-cli"}
	if home != "" {
		iina = append(iina, filepath.Join(home, "Applications/IINA.app/Contents/MacOS/iina-cli"))
	}
	// IINA 需要通过 --stdin 从 stdin 管道读取。stdin 模式下 {playerinput} 会被
	// streamlink 替换为 "-"，与后面的 "-stdin" 拼成 "--stdin"。
	iina = append(iina, "iina-cli", "iina")

	vlc := []string{"/Applications/VLC.app/Contents/MacOS/VLC", "vlc"}

	return []Player{
		{ID: "iina", Name: "IINA", Args: "{playerinput}-stdin", candidates: iina},
		{ID: "vlc", Name: "VLC", Args: "--no-video-title-show", candidates: vlc},
	}
}

func windowsPlayers() []Player {
	pf := os.Getenv("ProgramFiles")
	pf86 := os.Getenv("ProgramFiles(x86)")

	pot := []string{
		under(pf, "DAUM", "PotPlayer", "PotPlayerMini64.exe"),
		under(pf, "DAUM", "PotPlayer", "PotPlayerMini.exe"),
		under(pf86, "DAUM", "PotPlayer", "PotPlayerMini64.exe"),
		under(pf86, "DAUM", "PotPlayer", "PotPlayerMini.exe"),
		"PotPlayerMini64.exe",
		"PotPlayerMini.exe",
	}

	vlc := []string{
		under(pf, "VideoLAN", "VLC", "vlc.exe"),
		under(pf86, "VideoLAN", "VLC", "vlc.exe"),
		"vlc.exe",
		"vlc",
	}

	return []Player{
		{ID: "potplayer", Name: "PotPlayer", Args: "{playerinput}", candidates: pot},
		{ID: "vlc", Name: "VLC", Args: "--no-video-title-show", candidates: vlc},
	}
}

func linuxPlayers() []Player {
	return []Player{
		{
			ID:   "vlc",
			Name: "VLC",
			Args: "--no-video-title-show",
			candidates: []string{
				"/usr/bin/vlc",
				"/usr/local/bin/vlc",
				"/snap/bin/vlc",
				"vlc",
			},
		},
	}
}

func under(base string, elems ...string) string {
	if base == "" {
		return ""
	}
	return filepath.Join(append([]string{base}, elems...)...)
}

// Resolve 返回可用的播放器可执行文件路径。
func (p Player) Resolve() (string, error) {
	checked := make([]string, 0, len(p.candidates))
	for _, candidate := range p.candidates {
		if candidate == "" {
			continue
		}
		checked = append(checked, candidate)

		if filepath.IsAbs(candidate) {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, nil
			}
			continue
		}
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("未找到播放器 %s。已检查以下位置：\n  %s\n\n请安装该播放器，或在界面中选择其它播放器。",
		p.Name, strings.Join(checked, "\n  "))
}
