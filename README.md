# voicein

Push-to-toggle speech-to-text for [Niri](https://github.com/YaLTeR/niri).
Press once to start listening, press again (or hit 60s) to transcribe into
the focused window. Thinking pauses do not end the take.

- Headless daemon. Not a TUI, not a window.
- Default engine: [SenseVoiceSmall](https://github.com/k2-fsa/sherpa-onnx) int8 on CPU. Whisper small is optional.
- Commands: `toggle` / `cancel` / `status` / `quit`.
- Injects into the focused window. Paste failure leaves the text on the clipboard.
- Models stay in `$XDG_DATA_HOME/voicein/models`. They are never copied into the Nix store.
- HUD is a 77px row of 1px bars at the bottom center. Silent = one faint baseline. Speech moves the bands. Visible only while recording or transcribing.

## Install (Home Manager)

```nix
{
  inputs.voicein.url = "git+ssh://git@github.com/maplevoid/voicein.git";

  # NixOS + home-manager as a NixOS module:
  home-manager.users.<name>.imports = [
    inputs.voicein.homeManagerModules.default
  ];

  # or a standalone home.nix:
  # imports = [ inputs.voicein.homeManagerModules.default ];

  services.voicein.enable = true;
}
```

`enable` puts `voicein` on `PATH` and starts a user unit on
`graphical-session.target`. The package wraps `wl-copy`, `wtype`, `xclip`,
`xdotool`, and `notify-send`. Use the host's `pw-record` and `niri`; do not
rebuild those from this flake.

Models are still manual:

```bash
mkdir -p ~/.local/share/voicein/models
# SenseVoiceSmall int8 + tokens.txt + silero_vad.onnx
# or Whisper small: small-encoder.int8.onnx + small-decoder.int8.onnx + small-tokens.txt
# https://github.com/k2-fsa/sherpa-onnx/releases
```

One-shot without Home Manager:

```bash
nix run 'git+ssh://git@github.com/maplevoid/voicein.git' -- daemon
```

Print a starter config (optional; missing file uses built-in defaults):

```bash
mkdir -p ~/.config/voicein
nix run 'git+ssh://git@github.com/maplevoid/voicein.git' -- config > ~/.config/voicein/config.toml
```

## Niri

The module does not bind keys. Add them yourself:

```kdl
binds {
    Shift+Alt+V hotkey-overlay-title="Voice input: toggle" { spawn "voicein" "toggle"; }
    Shift+Alt+C hotkey-overlay-title="Voice input: cancel" { spawn "voicein" "cancel"; }
}
```

Second `toggle` stops and transcribes. `cancel` drops the take. The same
commands work from a terminal.

## Failure feedback

- Decode / record failure: HUD flashes red. No notification.
- Inject failure: text stays on the clipboard; `notify-send` once. Set `notify = false` to silence that.

## Models

Default directory: `$XDG_DATA_HOME/voicein/models` (`~/.local/share/voicein/models`).

| Engine | Files |
| --- | --- |
| SenseVoice (default) | `model.int8.onnx`, `tokens.txt` |
| Whisper | `small-encoder.int8.onnx`, `small-decoder.int8.onnx`, `small-tokens.txt` |
| HUD VAD | `silero_vad.onnx` |

SenseVoice is ~0.3s after the second press. Mixed Chinese/English: set
`engine = "whisper"` and fill encoder/decoder/tokens. Whisper is about 4× slower.

VAD is only for the HUD. Decode uses the whole take (leading/trailing silence
trimmed). Stop is always the second keypress or `max_record`.

## Config

`$XDG_CONFIG_HOME/voicein/config.toml`. Empty / missing file = defaults.

```toml
socket = ""                 # default: $XDG_RUNTIME_DIR/voicein.sock
sample_rate = 16000
max_record = "60s"
language = "auto"           # auto | zh | en | yue | ja | ko
itn = true
threads = 4
notify = true

[model]
dir = ""                    # default: $XDG_DATA_HOME/voicein/models
engine = "sensevoice"       # sensevoice | whisper
onnx = "model.int8.onnx"
tokens = "tokens.txt"
vad = "silero_vad.onnx"
```

## Inject

Focus decides the path:

- Wayland window (`wtype`): copy, then Ctrl+V; fallback is type-from-stdin.
- XWayland (`niri` focused PID is `xwayland-satellite`): `xclip` + `xdotool`.

Text is always copied first.

## Develop

```bash
nix develop
go test ./...
go build -o voicein ./cmd/voicein
```
