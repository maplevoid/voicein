# voicein

Push-to-talk speech-to-text for [Niri](https://github.com/YaLTeR/niri) on
Linux. The daemon owns the hotkey via `/dev/input/event*`. Niri does not
need to spawn `voicein` on press.

Three record modes:

- `hybrid` (default): press starts recording. A tap shorter than `tap`
  (300ms) latches like toggle; hold past that threshold stops on release.
  A later press stops a latched take.
- `toggle`: press the hotkey to start, press again (or hit 60s) to transcribe.
- `hold`: hold the hotkey; release (or hit 60s) to transcribe.

Thinking pauses do not end the take.

- Headless daemon. Not a TUI, not a window.
- Default engine: [SenseVoiceSmall](https://github.com/k2-fsa/sherpa-onnx) int8 on CPU. Whisper small is optional.
- Commands: `daemon` / `toggle` / `down` / `up` / `cancel` / `status` / `quit` / `config`.
- Injects into the focused window. Paste failure leaves the text on the clipboard.
- Models stay in `$XDG_DATA_HOME/voicein/models`. They are never copied into the Nix store.
- HUD is a 77px row of 1px bars at the bottom center. Silent = one faint baseline. Speech moves the bands. Visible only while recording or transcribing.

Linux only (`x86_64` / `aarch64`). Needs PipeWire (`pw-record`). The user
must be in group `input` so the daemon can read evdev.


## 1. Models

Models are not part of the package. Put them here before starting the daemon:

```text
~/.local/share/voicein/models/
```

SenseVoice (default, ~229MB) plus Silero VAD (~0.6MB):

```bash
mkdir -p ~/.local/share/voicein/models
cd /tmp

curl -L -o sensevoice.tar.bz2 \
  https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-sense-voice-zh-en-ja-ko-yue-int8-2024-07-17.tar.bz2
tar xf sensevoice.tar.bz2
cp sherpa-onnx-sense-voice-zh-en-ja-ko-yue-int8-2024-07-17/model.int8.onnx \
   sherpa-onnx-sense-voice-zh-en-ja-ko-yue-int8-2024-07-17/tokens.txt \
   ~/.local/share/voicein/models/

curl -L -o ~/.local/share/voicein/models/silero_vad.onnx \
  https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/silero_vad.onnx
```

VAD is only for the HUD. Missing `silero_vad.onnx` still starts the daemon
(energy-gate fallback). Missing `model.int8.onnx` or `tokens.txt` does not.

Optional Whisper small (~360MB). Copy the int8 files as-is:

```bash
curl -L -o whisper.tar.bz2 \
  https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-whisper-small.tar.bz2
tar xf whisper.tar.bz2
cp sherpa-onnx-whisper-small/small-encoder.int8.onnx ~/.local/share/voicein/models/
cp sherpa-onnx-whisper-small/small-decoder.int8.onnx ~/.local/share/voicein/models/
cp sherpa-onnx-whisper-small/small-tokens.txt        ~/.local/share/voicein/models/
```

Then in `~/.config/voicein/config.toml`:

```toml
[model]
engine = "whisper"
encoder = "small-encoder.int8.onnx"
decoder = "small-decoder.int8.onnx"
tokens = "small-tokens.txt"
```

Whisper is about 4× slower than SenseVoice.

## 2. Install

### Home Manager on NixOS

```nix
{
  inputs.voicein.url = "github:maplevoid/voicein";

  # inside nixosSystem { modules = [ ... ]; }
  home-manager.users.<name>.imports = [
    inputs.voicein.homeManagerModules.default
  ];
  home-manager.users.<name>.services.voicein.enable = true;
}
```

### Standalone `home.nix`

```nix
{
  inputs,
  ...
}: {
  imports = [ inputs.voicein.homeManagerModules.default ];
  services.voicein.enable = true;
}
```

`enable` puts `voicein` on `PATH`, creates
`~/.local/share/voicein/models`, and starts a user unit on
`graphical-session.target`. The package wraps `wl-copy`, `wtype`, `xclip`,
`xdotool`, and `notify-send`. Use the host's `pw-record` and `niri`; do not
rebuild those from this flake.

After `home-manager switch` / `nixos-rebuild switch`, if this graphical
session was already running:

```bash
systemctl --user daemon-reload
systemctl --user start voicein
voicein status   # idle
```

### One-shot without Home Manager

```bash
nix run github:maplevoid/voicein -- daemon
```

## 3. Hotkey

The daemon reads `/dev/input/event*` and needs group `input`.
Niri is optional. Bind the same chord to an empty action so the letter
does not type into the focused window.

```kdl
binds {
    Shift+Alt+V repeat=false hotkey-overlay-title="Voice input" { spawn "true"; }
    Shift+Alt+C hotkey-overlay-title="Voice input: cancel" { spawn "voicein" "cancel"; }
}
```

`hotkey = ""` disables the evdev listener. `toggle` / `down` / `up` still
work from a terminal or a compositor bind. `cancel` drops the take.

## 4. Check it

```bash
voicein status          # idle | recording | transcribing
journalctl --user -u voicein -e
```

A healthy start looks like:

```text
engine sensevoice model=/home/you/.local/share/voicein/models/model.int8.onnx
vad ready .../silero_vad.onnx; tap <300ms latches, hold releases
listening on /run/user/UID/voicein.sock
hotkey shift+alt+v via evdev; niri bind optional (swallow key)
hud: wayland ready 77x36 layer=overlay
```

| Symptom | Cause |
| --- | --- |
| `model ...: no such file or directory` | files not copied into `~/.local/share/voicein/models` |
| `daemon not running (.../voicein.sock)` | unit not started in this session |
| HUD flashes red, no notification | decode / record failed |
| text on clipboard, one `notify-send` | inject failed (`notify = false` silences that) |
| `pw-record: command not found` | PipeWire not on `PATH` |
| hotkey does nothing | user not in group `input`; check `hotkey listen:` in the journal |
| letter still types into the window | add a Niri bind that swallows the chord |



## Config

Optional. Missing file uses built-in defaults.

```bash
mkdir -p ~/.config/voicein
nix run github:maplevoid/voicein -- config > ~/.config/voicein/config.toml
```

```toml
socket = ""                 # default: $XDG_RUNTIME_DIR/voicein.sock
sample_rate = 16000
max_record = "60s"
language = "auto"           # auto | zh | en | yue | ja | ko
itn = true
threads = 4
notify = true
mode = "hybrid"             # hybrid | toggle | hold
tap = "300ms"               # hybrid: shorter tap latches; hold releases
hotkey = "shift+alt+v"      # evdev; empty disables

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
