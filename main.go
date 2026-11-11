package main

import (
	"log"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

func main() {
	logFile, logErr := setupLogging()
	if logFile != nil {
		defer logFile.Close()
	}
	if logErr != nil {
		log.Printf("无法写入日志文件，输出仅保留在控制台：%v", logErr)
	}

	a := app.NewWithID("io.github.stream-player")
	w := a.NewWindow("Stream Player")

	w.SetContent(newUI(w).content())
	w.Resize(fyne.NewSize(680, 660))
	w.CenterOnScreen()
	w.ShowAndRun()
}

// setupLogging 把程序与 Fyne 的输出追加写入配置目录下的 stream-player.log。
func setupLogging() (*os.File, error) {
	cfgPath, err := configPath()
	if err != nil {
		return nil, err
	}

	logPath := filepath.Join(filepath.Dir(cfgPath), "stream-player.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}

	log.SetOutput(f)
	log.SetFlags(log.LstdFlags)
	log.Printf("=== Stream Player 启动（日志：%s）===", logPath)
	return f, nil
}
