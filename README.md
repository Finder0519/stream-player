# Stream Player

基于 Go + Fyne v2 的 streamlink 启动器，macOS / Windows / Linux 均可构建使用。用界面填写播放地址与代理，按当前平台选择播放器，一次点击交给 streamlink 拉起播放器。

## 功能

- 输入播放 URL，选择播放器后点「启动播放」。
- streamlink 路径可留空自动查找（常见安装位置 + `PATH`），也可手动指定。
- 可选代理：HTTP 或 SOCKS5，填写 `host:port`，可选用户名与密码。
- 播放器候选按平台生成：macOS 为 IINA / VLC，Windows 为 PotPlayer / VLC，Linux 为 VLC。
- 固定参数（不可配置）：
  - User-Agent：`Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36`
  - 质量：`best,720p,480p,worst`（streamlink 按顺序回退）
- 配置自动保存并在下次启动时回填。
- 关闭窗口后隐藏到系统托盘，播放不中断；左键点击托盘图标显示窗口，右键菜单提供「打开窗口」「停止播放」「退出」。
- streamlink 缺失、播放器缺失、代理参数非法、streamlink 异常退出时都会在界面弹窗给出可读错误，并附上 stderr 末尾内容。

## 环境要求

### 1. Go 与 cgo

Fyne 依赖 cgo，必须启用 CGO（`CGO_ENABLED=1`，默认即是）。所需最低 Go 版本以 `go.mod` 的 `go` 指令为准。

| 平台 | 编译前提 |
| --- | --- |
| macOS | 安装 Xcode Command Line Tools：`xcode-select --install` |
| Windows | 安装 MinGW-w64 的 gcc（推荐 MSYS2：`pacman -S mingw-w64-x86_64-gcc`），并把 `C:\msys64\mingw64\bin` 加入 `PATH`；在 PowerShell 里 `gcc --version` 能查到后再构建 |
| Linux | gcc + X11/OpenGL 开发库。Debian/Ubuntu：`sudo apt install gcc libgl1-mesa-dev xorg-dev`；Fedora：`sudo dnf install gcc libX11-devel libXcursor-devel libXrandr-devel libXinerama-devel libXi-devel mesa-libGL-devel` |

### 2. streamlink

程序按以下顺序查找 `streamlink`：

1. 界面「streamlink 路径」中填写的路径（绝对路径，或 `PATH` 中的可执行文件名）
2. 平台常见安装位置：
   - macOS：`/opt/homebrew/bin/streamlink`、`/usr/local/bin/streamlink`、`~/.local/bin/streamlink`
   - Linux：`/usr/bin/streamlink`、`/usr/local/bin/streamlink`、`/snap/bin/streamlink`、`~/.local/bin/streamlink`
3. `PATH` 中的 `streamlink`

注意：从 Finder/Dock 启动的 `.app` 不会继承终端的 `PATH`（launchd 默认只有 `/usr/bin:/bin:/usr/sbin:/sbin`），所以 Homebrew 安装的 streamlink 在终端里能查到、双击应用却找不到。此时上面的固定候选路径会自动兜底；若 streamlink 装在其它位置（如 pyenv/conda 环境），请在界面中填写绝对路径。

安装方式：

- macOS：`brew install streamlink`
- Windows：`pip install -U streamlink`（或用官方安装包）
- Linux：发行版包管理器，或 `pipx install streamlink`

### 3. 播放器

程序会按下面的顺序探测可执行文件；任一存在即可，找不到时会在弹窗中列出检查过的路径。

| 平台 | 播放器 | 默认可执行文件 | `--player-args` |
| --- | --- | --- | --- |
| macOS | IINA | `/Applications/IINA.app/Contents/MacOS/iina-cli`、`~/Applications/IINA.app/Contents/MacOS/iina-cli`，再退回 PATH 中的 `iina-cli` / `iina` | `{playerinput}-stdin`（stdin 模式下 `{playerinput}` 为 `-`，拼接后即 `--stdin`，IINA 从 stdin 管道播放） |
| macOS | VLC | `/Applications/VLC.app/Contents/MacOS/VLC`，再退回 PATH 中的 `vlc` | `--no-video-title-show` |
| Windows | PotPlayer | `%ProgramFiles%\DAUM\PotPlayer\PotPlayerMini64.exe`（或 `PotPlayerMini.exe`），以及 `%ProgramFiles(x86)%` 下的同名文件，再退回 PATH | `{playerinput}`（streamlink 在 stdin 模式下传入 `-`，PotPlayer 用 `-` 表示从标准输入播放） |
| Windows | VLC | `%ProgramFiles%\VideoLAN\VLC\vlc.exe`、`%ProgramFiles(x86)%\VideoLAN\VLC\vlc.exe`，再退回 PATH 中的 `vlc.exe` | `--no-video-title-show` |
| Linux | VLC | `/usr/bin/vlc`、`/usr/local/bin/vlc`、`/snap/bin/vlc`，再退回 PATH 中的 `vlc` | `--no-video-title-show` |

## 构建与运行

```bash
# macOS / Linux
go build -o stream-player .
./stream-player

# 开发时直接运行
go run .
```

```powershell
# Windows（-H windowsgui 避免额外控制台窗口）
go build -ldflags "-H windowsgui" -o stream-player.exe .
.\stream-player.exe
```

Fyne 需要 cgo，交叉编译必须准备目标平台的 C 工具链，因此建议在各自平台上原生构建；macOS 如需 `.app` 包可另装 `fyne` CLI 后执行 `fyne package -os darwin`（可选，非必需）。

## 实际执行的命令

```
streamlink --player <播放器路径> --player-args "<上面的参数>" \
  --ffmpeg-ffmpeg <ffmpeg路径> \
  --http-header "User-Agent=<固定 UA>" \
  [--http-proxy <代理URL>] \
  <URL> best,720p,480p,worst
```

- `--ffmpeg-ffmpeg` 传入程序自行探测到的 ffmpeg 绝对路径。streamlink 只按 `PATH` 查找 ffmpeg，而从 Finder/Dock 启动的 `.app` 不继承终端的 `PATH`；找不到 ffmpeg 时 streamlink 无法合并分离的音视频轨（HLS 会退化为 `hls` 而不是 `hls-multi`），表现为只有画面没有声音。
- 代理 URL：HTTP 为 `http://host:port`，SOCKS5 为 `socks5://host:port`；用户名非空时写成 `scheme://user:password@host:port`，用户名和密码由 Go 的 `net/url` 负责转义。凭据只用于拼这条命令，不会写入日志或配置文件之外的地方；错误弹窗中的密码会被替换为 `***`。
- streamlink 8.x 已弃用 `--https-proxy`（推荐只传 `--http-proxy`），当前实现按需求仍同时传入两个参数：功能正常，只会在 stderr 输出一条弃用警告。`socks5://` 表示 DNS 在本地解析，若需由代理服务器解析域名，streamlink 支持 `socks5h://`，但本程序按需求固定使用 `socks5://`。
- URL 未写协议时会自动补 `https://`。

## 配置文件

| 平台 | 路径 |
| --- | --- |
| macOS | `~/Library/Application Support/stream-player/config.json`（macOS 的用户配置目录，等价于 Linux 的 `~/.config`） |
| Linux | `~/.config/stream-player/config.json` |
| Windows | `%APPDATA%\stream-player\config.json` |

示例：

```json
{
  "url": "https://example.com/live",
  "use_proxy": true,
  "proxy_protocol": "socks5",
  "proxy_host": "127.0.0.1",
  "proxy_port": "7890",
  "proxy_username": "",
  "proxy_password": "",
  "player": "vlc",
  "streamlink_path": ""
}
```

配置在点击「启动播放」以及关闭窗口时写入，文件权限为 `0600`。代理密码以明文保存在该文件中，请自行注意目录权限。

## 日志

程序输出（包含 Fyne 自身的告警）以及每次 streamlink 的 stderr/stdout 末尾内容，都会追加写入配置目录下的 `stream-player.log`（权限 `0600`）：

| 平台 | 路径 |
| --- | --- |
| macOS | `~/Library/Application Support/stream-player/stream-player.log` |
| Linux | `~/.config/stream-player/stream-player.log` |
| Windows | `%APPDATA%\stream-player\stream-player.log` |

日志中的代理密码和完整代理 URL 会替换为 `***`；日志按追加方式写入，不做自动轮转。

## 关于控制台窗口

- Windows：Go 默认构建出的是控制台子系统程序，双击会带一个黑色控制台窗口，这与代码无关。请使用 `go build -ldflags "-H windowsgui" -o stream-player.exe .` 构建；用该参数构建后没有控制台窗口。streamlink 子进程的控制台窗口程序里已通过 `SysProcAttr{HideWindow: true}` 隐藏。
- macOS / Linux：从终端执行 `go run .` 或 `./stream-player` 时，终端窗口是启动它的那个终端，并不会额外弹出新窗口；用 `fyne package` 打成 `.app` 后从 Finder 启动就不会带终端。
- 由于程序输出已重定向到日志文件，终端和 Windows 控制台不再显示 Fyne/streamlink 的输出，排查问题请直接查看上面的日志文件。

## 行为说明

- 点击「启动播放」后该按钮禁用、「停止」按钮可用；点击「停止」会终止 streamlink 进程，streamlink 退出后播放器因 stdin 管道关闭而结束播放，状态栏显示「已停止播放」。
- 播放器关闭、streamlink 正常退出后，「启动播放」恢复可用，状态栏显示「播放结束」。
- 关闭窗口只是隐藏窗口并保存配置，streamlink 和播放器继续运行；左键点击托盘图标重新显示窗口。
- 托盘右键菜单：「打开窗口」显示主窗口，「停止播放」终止当前拉流（未播放时置灰），「退出」先结束 streamlink 再退出程序。程序通过其它方式退出时也会在生命周期回调里结束 streamlink，避免后台残留。
- Linux 桌面环境没有托盘宿主时（例如 GNOME 未安装 AppIndicator 扩展），图标可能不显示，此时关闭窗口后窗口会隐藏。
- macOS / Linux 按单实例运行：重复启动不会出现第二个进程，新进程通过本地 socket 通知已有实例显示窗口后退出；socket 位于 `$XDG_RUNTIME_DIR`（未设置时用 `os.TempDir()`，macOS 即用户私有临时目录），文件权限 0600。macOS 从 Finder 启动 `.app` 本来也会由系统复用已有实例。
- Windows 上按单实例运行：重复启动不会再出现第二个窗口、托盘图标或进程，新进程会通知已有实例显示窗口后立即退出。互斥体与通知事件都在 `Local\` 命名空间，只约束同一登录会话，不同 Windows 用户可以各开一个实例。
- streamlink 非零退出时会弹窗显示退出错误和 stderr 末尾内容（最多保留最近 8 KB），不会静默失败；同样的内容也会写入日志。
- Windows 下会隐藏 streamlink 子进程的控制台窗口。
