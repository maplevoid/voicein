# voicein

Niri 上的常驻语音输入 daemon。按一次开始听，再按一次（或满 60 秒）转写，注入当前焦点窗口。思考停顿不会自动结束。

- 无窗口 daemon，不是 TUI，也不是 Tauri
- 引擎默认 SenseVoiceSmall；可切 [Whisper](https://github.com/k2-fsa/sherpa-onnx) small，CPU，官方 Go 绑定
- 松手后再转。程序只暴露 `toggle` / `cancel` / `status`
- 注入当前焦点窗口。失败留剪贴板
- 登录就起 daemon，模型常驻
- 文本：标点 + ITN；口令以后再加
- 语言：Go
- 反馈：底部正中很短一簇白色竖条，静音几乎贴底，说话才按音量此起彼伏；exclusive-zone 0，不抢键盘，不抢焦点
- HUD 只在录音 / 转写时出现

## Quick start

```bash
nix develop
go build -o voicein ./cmd/voicein

mkdir -p ~/.local/share/voicein/models ~/.config/voicein
# SenseVoiceSmall int8 + tokens.txt + silero_vad.onnx
# 或 Whisper small: small-encoder.int8.onnx + small-decoder.int8.onnx + small-tokens.txt
# https://github.com/k2-fsa/sherpa-onnx/releases

voicein config > ~/.config/voicein/config.toml   # optional
voicein daemon
```

另一终端：

```bash
voicein toggle    # start
voicein toggle    # stop + transcribe + inject
voicein cancel
voicein status
```

## Niri

登录自启走用户 systemd（`~/.config/niri/config.kdl` 由 home-manager 管，当前不可写）：

```bash
systemctl --user enable --now voicein.service
```

热键已经写在当前 Niri 配置里：

```kdl
binds {
    Shift+Alt+V hotkey-overlay-title="Voice input: toggle" { spawn "voicein" "toggle"; }
    Shift+Alt+C hotkey-overlay-title="Voice input: cancel" { spawn "voicein" "cancel"; }
}
```

第二次 `toggle` 结束并转写。`cancel` 丢弃当前段，不注入。
没有热键时也可以在终端跑 `voicein toggle` / `voicein cancel`。

NixOS 上 Niri 的 `spawn` 没有 `nix develop` 的 `LD_LIBRARY_PATH`。`sherpa-onnx` 需要 `libstdc++.so.6`，所以 `~/.local/bin/voicein` 必须是带库路径的包装脚本，真正的二进制放 `~/.local/libexec/voicein`。

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

运行时（NixOS / 用户环境）：

- `pw-record`（pipewire）
- `wl-copy`（wl-clipboard）
- `wtype`（Wayland 窗口，如 Ghostty）
- `xclip` + `xdotool`（XWayland 窗口，Flatpak QQ / 微信）
- `notify-send`（libnotify / mako）

注入看焦点窗口：Wayland 用 `wtype`，X11（`niri` 里 PID 是 `xwayland-satellite`）用 `xclip`/`xdotool`。
文本始终先复制到对应剪贴板；粘贴失败再尝试逐字输入。

NixOS / Home Manager 要把 X11 工具放进**用户环境**，不能只靠 `~/.local/bin` 临时 symlink。daemon 的 PATH 来自用户 systemd，登录后必须能直接找到 `xclip` 和 `xdotool`：

```nix
# home.packages / users/.shared/niri.nix
wtype
xclip
xdotool
```

然后 `home-manager switch` / `ug`。不要把这两个二进制手链到 nix store 路径。

开发：

```bash
nix flake init --template "https://flakehub.com/f/the-nix-way/dev-templates/*#go"
nix develop
```

本仓库已经按这个模板初始化。测试和构建都走 `nix develop`。
