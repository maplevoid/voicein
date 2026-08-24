# voicein

Niri 上的常驻语音输入 daemon。按一次开始听，再按一次（或静音 8 秒 / 满 60 秒）转写，注入当前焦点窗口。

- 无窗口 daemon，不是 TUI，也不是 Tauri
- 引擎默认 [SenseVoiceSmall](https://github.com/k2-fsa/sherpa-onnx) via sherpa-onnx，CPU，官方 Go 绑定
- 松手后再转。程序只暴露 `toggle` / `cancel` / `status`
- 注入当前焦点窗口。失败留剪贴板
- 登录就起 daemon，模型常驻
- 文本：标点 + ITN；口令以后再加
- 语言：Go
- 反馈：底部居中小波形胶囊，exclusive-zone 0，不抢键盘，不抢焦点
- HUD 只在录音 / 转写时出现

## Quick start

```bash
nix develop
go build -o voicein ./cmd/voicein

mkdir -p ~/.local/share/voicein/models ~/.config/voicein
# SenseVoiceSmall int8 + tokens.txt + silero_vad.onnx
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

热键需要写进 Niri 配置：

```kdl
spawn-at-startup "voicein" "daemon"

binds {
    Super+Shift+V { spawn "voicein" "toggle"; }
    Super+Shift+C { spawn "voicein" "cancel"; }
}
```

第二次 `toggle` 结束并转写。`cancel` 丢弃当前段，不注入。
没有热键时也可以在终端跑 `voicein toggle` / `voicein cancel`。

## 失败反馈

- 只有波形。失败就波形变红一下，不弹通知
- 注入失败：文本留剪贴板，`notify-send` 一声（可用 `notify = false` 关掉）

## 模型

放到 `$XDG_DATA_HOME/voicein/models`（默认 `~/.local/share/voicein/models`）：

- `model.int8.onnx` — SenseVoiceSmall
- `tokens.txt`
- `silero_vad.onnx` — 预留；当前录音用能量门限 + 8s 静音，VAD 文件暂未强制加载

## 依赖

运行时（NixOS / 用户环境）：

- `pw-record`（pipewire）
- `wl-copy`（wl-clipboard）
- `wtype`
- `notify-send`（libnotify / mako）

开发：

```bash
nix flake init --template "https://flakehub.com/f/the-nix-way/dev-templates/*#go"
nix develop
```

本仓库已经按这个模板初始化。测试和构建都走 `nix develop`。
