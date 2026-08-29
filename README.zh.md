[English](README.md) · [中文](README.zh.md)

# voicein

Linux 按住说话转写。

daemon 通过 `/dev/input/event*` 占用热键。识别是
[scribe](https://github.com/maplevoid/scribe)。这个进程负责录音、HUD、
把文字注入当前窗口。

```bash
scribe fetch-model
scribe serve &
voicein daemon
```

按住 `Shift+Alt+V` 说话。轻点（<300ms）闩住，再按停止。按住超过 300ms
松手停止。`voicein cancel` 丢掉这一句。

## 依赖

- Linux（`x86_64` / `aarch64`）
- PipeWire（`pw-record`）
- `input` 组（热键读 `/dev/input/event*`）
- 同一台机器上的 [scribe](https://github.com/maplevoid/scribe)
- Wayland：`wl-copy`、`wtype`（HUD 需要 layer-shell）
- X11：`xclip`、`xdotool`（没有 HUD）

不绑 niri，也不绑任何合成器。合成器 bind 是可选的：只有组合键会打进
当前窗口时才需要。

## 安装

### 二进制

打 `v*` tag 之后，GitHub Actions 会上传：

| 架构 | 压缩包 |
| --- | --- |
| `x86_64` | `voicein-linux-amd64.tar.gz` |
| `aarch64` | `voicein-linux-arm64.tar.gz` |

```bash
curl -fsSL -o voicein.tar.gz \
  https://github.com/maplevoid/voicein/releases/latest/download/voicein-linux-amd64.tar.gz
tar -xzf voicein.tar.gz
sudo install -m 755 voicein-linux-amd64/voicein /usr/local/bin/voicein
```

voicein 是静态 Go 二进制。同一台机器上仍需要
[scribe](https://github.com/maplevoid/scribe)（二进制或源码）、PipeWire、
`input` 组，以及下面的注入工具。

### 从源码

```bash
# 1. scribe（转写）— 二进制，或：
git clone https://github.com/maplevoid/scribe.git
cd scribe
scripts/fetch-sherpa.sh x86_64-unknown-linux-gnu
go build -o scribe ./cmd/scribe
sudo install -m 755 scribe /usr/local/bin/scribe

# 2. voicein
git clone https://github.com/maplevoid/voicein.git
cd voicein
go build -mod=vendor -o voicein ./cmd/voicein
sudo install -m 755 voicein /usr/local/bin/voicein
```

`PATH` 上需要这些包：

| 发行版 | 包 |
| --- | --- |
| Debian / Ubuntu | `pipewire-bin wl-clipboard wtype xclip xdotool libnotify-bin` |
| Fedora | `pipewire-utils wl-clipboard wtype xclip xdotool libnotify` |
| Arch | `pipewire wl-clipboard wtype xclip xdotool libnotify` |

把自己加进 `input` 组，然后注销再登录：

```bash
sudo usermod -aG input "$USER"
id   # 必须列出 input
```

Debian/Ubuntu 上设备节点通常是 `root:input`、模式 `660`。有些环境仍是
`root:root` `600`——那加组之后热键还是没反应。检查：

```bash
ls -l /dev/input/event0
```

## 运行

```bash
scribe fetch-model          # SenseVoice 放到 ~/.local/share/scribe/models
scribe serve &              # 或下面的 systemd socket
voicein daemon
voicein status              # idle
```

不用本地模型的话，在 scribe 的配置里改成 Groq / OpenAI，不必
`fetch-model`。没有 `scribe serve`（或 `scribe.socket`），一句录音结束
会闪一下 HUD，没有文字。

不用 Nix 时的 systemd 用户 unit：

```ini
# ~/.config/systemd/user/scribe.socket
[Socket]
ListenStream=%t/scribe.sock
SocketMode=0600

[Install]
WantedBy=sockets.target
```

```ini
# ~/.config/systemd/user/scribe.service
[Service]
ExecStart=/usr/local/bin/scribe serve
Type=simple
```

```ini
# ~/.config/systemd/user/voicein.service
[Unit]
After=graphical-session.target scribe.socket
Wants=scribe.socket
PartOf=graphical-session.target

[Service]
ExecStart=/usr/local/bin/voicein daemon
Restart=on-failure

[Install]
WantedBy=graphical-session.target
```

```bash
systemctl --user daemon-reload
systemctl --user enable --now scribe.socket
systemctl --user enable --now voicein
```

如果这个图形会话已经在跑，enable 之后还要手动 start。

## Nix

```bash
nix run github:maplevoid/scribe -- fetch-model
nix run github:maplevoid/voicein -- daemon
```

Home Manager（NixOS 或独立）：

```nix
{
  inputs.scribe.url = "github:maplevoid/scribe";
  inputs.scribe.inputs.nixpkgs.follows = "nixpkgs";
  inputs.voicein.url = "github:maplevoid/voicein";
  inputs.voicein.inputs.nixpkgs.follows = "nixpkgs";
}
```

```nix
{
  imports = [
    inputs.scribe.homeManagerModules.default
    inputs.voicein.homeManagerModules.default
  ];
  services.scribe.enable = true;
  services.voicein.enable = true;
}
```

NixOS 上仍然要自己加 `input` 组：

```nix
users.users.<name>.extraGroups = [ "input" ];
```

然后重建 **并且注销**。`id` 必须能看到 `input`。Home Manager 模块不加
这个组，不下载模型，会 wrap `wl-copy` / `wtype` / `xclip` / `xdotool` /
`notify-send`。它们用主机上的 `pw-record`。

## 录音模式

- `hybrid`（默认）：按下开始录。短于 `tap`（300ms）的轻点会像 toggle
  一样闩住；按住超过阈值则松手停止。闩住后再按一次结束。
- `toggle`：按下开始，再按一次（或到 60s）转写。
- `hold`：按住；松开（或到 60s）转写。

思考停顿不会结束这一句。第一次按键也会预热 scribe（空 PCM）。转写过程中
热键暂时无效，用 `cancel`。

`hotkey = ""` 关掉 evdev。`toggle` / `down` / `up` 仍可从终端或合成器
bind 调用。

## 可选：别让组合键打进窗口

合成器不会开始录音。daemon 读 evdev，没有 `EVIOCGRAB`，所以未绑定的
`Shift+Alt+V` 仍会进当前窗口（Ghostty / 终端常常当成粘贴）。把同样的
键绑到空操作才能吞掉。

不要把 V 绑成 `voicein toggle`：evdev 已经 latch，合成器再 spawn 会打
两次。C 要绑 `voicein cancel`；daemon 没有 evdev 取消键。

Hyprland：

```conf
bind = SHIFT ALT, V, exec, true
bind = SHIFT ALT, C, exec, voicein cancel
```

niri（默认 `mod-key` Super）：

```kdl
binds {
    Shift+Alt+V repeat=false { spawn "true"; }
    Shift+Alt+C { spawn "voicein" "cancel"; }
}
```

若 `mod-key "Alt"`，物理 `Shift+Alt+V` / `Shift+Alt+C` 是
`Mod+Shift+V` / `Mod+Shift+C`。写 `Mod+`，不要写 `Shift+Alt+`：

```kdl
binds {
    Mod+Shift+V repeat=false { spawn "true"; }
    Mod+Shift+C { spawn "voicein" "cancel"; }
}
```

Sway / i3：把同样的键绑到 `true` / `voicein cancel`。

## 配置

可选。缺文件用内置默认值。

voicein 读 `~/.config/voicein/config.toml`。引擎、语言、空闲退出在
`~/.config/scribe/config.toml`。

```bash
voicein config > ~/.config/voicein/config.toml
scribe config  > ~/.config/scribe/config.toml
voicein config set hotkey "shift+alt+v"
scribe config set engine sensevoice
```

```toml
socket = ""                 # 默认: $XDG_RUNTIME_DIR/voicein.sock
sample_rate = 16000
max_record = "60s"
notify = true
mode = "hybrid"             # hybrid | toggle | hold
tap = "300ms"
hotkey = "shift+alt+v"      # evdev；空字符串关掉

[scribe]
socket = ""                 # 默认: $XDG_RUNTIME_DIR/scribe.sock
```

## 在线转写

还是这两个 daemon。在 `~/.config/scribe/config.toml` 里：

```toml
[model]
engine = "groq"             # 或 "openai"
```

```bash
export GROQ_API_KEY=gsk_...
# 已经在跑的用户 unit:
# systemctl --user edit scribe
# [Service]
# Environment=GROQ_API_KEY=gsk_...
systemctl --user restart scribe.socket
```

`openai` 用 `OPENAI_API_KEY`。两个引擎都接受 `SCRIBE_API_KEY`。不要把
key 写进 toml。

## 注入

会话类型决定工具，不是焦点窗口：

- Wayland（`wl-copy` + `wtype`）：先复制，再 Ctrl+V；失败则从 stdin 打字。
- X11（`xclip` + `xdotool`）：`XDG_SESSION_TYPE=x11`，或设置了 `DISPLAY`
  且没有 `WAYLAND_DISPLAY`。

Wayland 会话里焦点在 XWayland 窗口时仍走 `wtype`。文字一定先复制。粘贴
失败时留在剪贴板。

HUD 需要 Wayland layer-shell。纯 X11 能注入，没有条。

## 检查

```bash
id                          # 必须列出 input
voicein status              # idle
journalctl --user -u scribe -u voicein -e
```

正常启动类似：

```text
# scribe
engine sensevoice model=/home/you/.local/share/scribe/models/model.int8.onnx
listening on /run/user/UID/scribe.sock idle=10m0s

# voicein
scribe socket /run/user/UID/scribe.sock
listening on /run/user/UID/voicein.sock
hotkey shift+alt+v via evdev; compositor bind optional (swallow key)
hud: wayland ready 77x36 layer=overlay
```

| 现象 | 原因 |
| --- | --- |
| `model .../tokens.txt: no such file or directory` | 没跑 `scribe fetch-model` |
| `daemon not running (.../voicein.sock)` | daemon / unit 没启动 |
| 热键没反应，journal `hotkey listen:` | 不在 `input` 组，或加组后没注销 |
| 字母仍打进窗口 | 没有合成器空操作 bind |
| `pw-record: command not found` | `PATH` 上没有 PipeWire |
| HUD 闪红，没有通知 | 解码 / 录音失败；看 scribe 日志 |
| 文字在剪贴板，一次 `notify-send` | 注入失败 |
| 录音结束没字，scribe 没在听 | `scribe serve` / `scribe.socket` 没在跑 |

## 开发

```bash
go test ./...
go build -o voicein ./cmd/voicein
```

或 `nix develop`。
