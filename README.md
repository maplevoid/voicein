# voicein

Niri 上的常驻语音输入 daemon。按一次开始听，再按一次（或满 60 秒）转写，注入当前焦点窗口。思考停顿不会自动结束。

- 无窗口 daemon，不是 TUI，也不是 Tauri
- 引擎默认 SenseVoiceSmall；可切 [Whisper](https://github.com/k2-fsa/sherpa-onnx) small，CPU，官方 Go 绑定
- 松手后再转。程序只暴露 `toggle` / `cancel` / `status`
- 注入当前焦点窗口。失败留剪贴板
- 登录就起 daemon，模型常驻
- 文本：标点 + ITN；口令以后再加
- 语言：Go
- 反馈：底部正中一排细白竖条，1px 宽 1px 缝铺满 77px，静音一条淡基线，说话才按频段起伏
- HUD 只在录音 / 转写时出现

## Install

模型自己放，不进 Nix store。

```bash
mkdir -p ~/.local/share/voicein/models
# SenseVoiceSmall int8 + tokens.txt + silero_vad.onnx
# 或 Whisper small: small-encoder.int8.onnx + small-decoder.int8.onnx + small-tokens.txt
# https://github.com/k2-fsa/sherpa-onnx/releases
```

一次性跑：

```bash
nix run github:maplevoid/voicein -- daemon
```

Home Manager / NixOS 用户模块：

```nix
{
  inputs.voicein.url = "github:maplevoid/voicein";

  # home-manager.users.<name>.imports 或 standalone home.nix
  imports = [ inputs.voicein.homeManagerModules.default ];

  services.voicein.enable = true;
}
```

`enable` 会把包放进用户环境，并挂上 `graphical-session.target` 的 user unit。`pw-record` 和 `niri` 用系统里已有的，不要跟着这个 flake 再编一份。

可选配置：

```bash
nix run github:maplevoid/voicein -- config > ~/.config/voicein/config.toml
```

## Niri

热键自己写：

```kdl
binds {
    Shift+Alt+V hotkey-overlay-title="Voice input: toggle" { spawn "voicein" "toggle"; }
    Shift+Alt+C hotkey-overlay-title="Voice input: cancel" { spawn "voicein" "cancel"; }
}
```

第二次 `toggle` 结束并转写。`cancel` 丢弃当前段，不注入。没有热键也可以在终端跑同样的命令。

## 失败反馈

- 只有波形。失败就波形变红一下，不弹通知
- 注入失败：文本留剪贴板，`notify-send` 一声（可用 `notify = false` 关掉）

## 模型

放到 `$XDG_DATA_HOME/voicein/models`（默认 `~/.local/share/voicein/models`）：

- SenseVoice：`model.int8.onnx` + `tokens.txt`
- Whisper：`small-encoder.int8.onnx` + `small-decoder.int8.onnx` + `small-tokens.txt`
- `silero_vad.onnx` — HUD 说话检测；解码用整段（只裁首尾静音），结束靠第二次按键

默认 SenseVoice，第二次按完大约 0.3s 出字。中英夹杂再改 `engine = "whisper"` 并填 encoder/decoder/tokens。Whisper 大约慢 4 倍。

## 依赖

包已经 wrap 了这些命令：

- `wl-copy`（wl-clipboard）
- `wtype`（Wayland 窗口，如 Ghostty）
- `xclip` + `xdotool`（XWayland 窗口，Flatpak QQ / 微信）
- `notify-send`（libnotify / mako）

还需要系统里已有的：

- `pw-record`（PipeWire）
- `niri`（探测焦点窗口是不是 XWayland）

注入看焦点窗口：Wayland 用 `wtype`，X11（`niri` 里 PID 是 `xwayland-satellite`）用 `xclip`/`xdotool`。文本始终先复制到对应剪贴板；粘贴失败再尝试逐字输入。

## 开发

```bash
nix develop
go test ./...
go build -o voicein ./cmd/voicein
```
