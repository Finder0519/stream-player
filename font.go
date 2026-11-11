package main

import (
	"bytes"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	gotextfont "github.com/go-text/typesetting/font"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

const preferredFontName = "Noto Sans SC"

// fontTheme 仅替换常规字体，粗体、斜体、等宽和符号仍使用 Fyne 默认字体。
type fontTheme struct {
	fyne.Theme
	font fyne.Resource
}

func (t fontTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Bold || style.Italic || style.Monospace || style.Symbol {
		return t.Theme.Font(style)
	}
	return t.font
}

// applyPreferredFont 在系统已安装 Noto Sans SC 时用它显示界面文字。
func applyPreferredFont(a fyne.App) {
	font := findNotoSansSC()
	if font == nil {
		log.Printf("系统未安装 %s，使用默认字体", preferredFontName)
		return
	}
	log.Printf("使用系统字体 %s：%s", preferredFontName, font.Name())
	a.Settings().SetTheme(fontTheme{Theme: theme.DefaultTheme(), font: font})
}

func findNotoSansSC() fyne.Resource {
	path := findFontFile("notosanssc")
	if path == "" {
		return nil
	}

	res, err := fyne.LoadResourceFromPath(path)
	if err != nil {
		log.Printf("读取字体 %s 失败：%v", path, err)
		return nil
	}
	// Fyne 不支持字体集合（ttc），先确认文件能作为单字体加载。
	if _, err := gotextfont.ParseTTF(bytes.NewReader(res.Content())); err != nil {
		log.Printf("字体 %s 无法被 Fyne 加载，改用默认字体：%v", path, err)
		return nil
	}
	return res
}

// findFontFile 在系统字体目录中查找文件名以 family 开头的字体，优先选择常规字重。
func findFontFile(family string) string {
	name := strings.NewReplacer("-", "", "_", "", " ", "").Replace(family)

	var matches []string
	for _, dir := range fontDirectories() {
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			base := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
			key := strings.ToLower(strings.NewReplacer("-", "", "_", "", " ", "").Replace(base))
			if strings.HasPrefix(key, name) {
				matches = append(matches, path)
			}
			return nil
		})
	}

	for _, path := range matches {
		if strings.Contains(strings.ToLower(filepath.Base(path)), "regular") {
			return path
		}
	}
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

func fontDirectories() []string {
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "darwin":
		return []string{
			filepath.Join(home, "Library", "Fonts"),
			"/Library/Fonts",
			"/System/Library/Fonts",
			"/System/Library/Fonts/Supplemental",
		}
	case "windows":
		windir := os.Getenv("WINDIR")
		if windir == "" {
			windir = `C:\Windows`
		}
		dirs := []string{filepath.Join(windir, "Fonts")}
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			dirs = append(dirs, filepath.Join(local, "Microsoft", "Windows", "Fonts"))
		}
		return dirs
	default:
		return []string{
			filepath.Join(home, ".local", "share", "fonts"),
			filepath.Join(home, ".fonts"),
			"/usr/share/fonts",
			"/usr/local/share/fonts",
		}
	}
}
