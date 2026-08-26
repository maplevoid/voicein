# voicein

Push-to-talk speech-to-text for Linux. The daemon owns the hotkey via
`/dev/input/event*`. A compositor bind is only there to swallow the chord
so `v` does not type into the focused window.

Three record modes:

- `hybrid` (default): press starts recording. A tap shorter than `tap`
  (300ms) latches like toggle; hold past that threshold stops on release.
  A later press stops a latched take.
- `toggle`: press to start, press again (or hit 60s) to transcribe.
- `hold`: hold; release (or hit 60s) to transcribe.

Thinking pauses do not end the take.

- Headless daemon. Not a TUI, not a window.
- Default engine: [SenseVoiceSmall](https://github.com/k2-fsa/sherpa-onnx) int8 on CPU. Whisper small is optional.
- Commands: `daemon` / `toggle` / `down` / `up` / `cancel` / `status` / `quit` / `config`.
- Injects into the focused window. Paste failure leaves the text on the clipboard.
- Models stay in `$XDG_DATA_HOME/voicein/models`. They are never copied into the Nix store.
- HUD is a 77px row of 1px bars at the bottom center. Visible only while recording or transcribing.

Linux only (`x86_64` / `aarch64`). Needs PipeWire (`pw-record`).

`nix run` / the Home Manager module is not enough. Without models the
daemon exits. Without group `input` it starts but the hotkey does nothing.

## 1. Models

Do this first. The package does not download them.

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

Required: `model.int8.onnx` + `tokens.txt`.
`silero_vad.onnx` is HUD only; missing it still starts (energy-gate fallback).

Optional Whisper small (~360MB):

```bash
curl -L -o whisper.tar.bz2 \
  https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-whisper-small.tar.bz2
tar xf whisper.tar.bz2
cp sherpa-onnx-whisper-small/small-encoder.int8.onnx \
   sherpa-onnx-whisper-small/small-decoder.int8.onnx \
   sherpa-onnx-whisper-small/small-tokens.txt \
   ~/.local/share/voicein/models/
```

```toml
[model]
engine = "whisper"
encoder = "small-encoder.int8.onnx"
decoder = "small-decoder.int8.onnx"
tokens = "small-tokens.txt"
```

Whisper is about 4× slower than SenseVoice.

## 2. Group `input`

The daemon reads `/dev/input/event*` (`crw-rw---- root input`).
The Home Manager module does **not** add this group.

```nix
# NixOS
users.users.<name>.extraGroups = [ "input" ];
```

Then `nixos-rebuild switch` **and log out / back in**. `id` must show
`input` before the hotkey can work. `groups` in an old terminal lies.

## 3. Install

### Home Manager on NixOS

```nix
{
  inputs.voicein.url = "github:maplevoid/voicein";
  inputs.voicein.inputs.nixpkgs.follows = "nixpkgs";

  # inside nixosSystem { modules = [ ... ]; }
  home-manager.users.<name>.imports = [
    inputs.voicein.homeManagerModules.default
  ];
  home-manager.users.<name>.services.voicein.enable = true;
}
```

`follows` keeps the friend on their own nixpkgs. Without it this flake
pulls `nixos-unstable` as well.

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
`~/.local/share/voicein/models`, and installs a user unit on
`graphical-session.target`. It does not download models and does not
add group `input`.

The package wraps `wl-copy`, `wtype`, `xclip`, `xdotool`, and
`notify-send`. Use the host's `pw-record`. Do not rebuild PipeWire
from this flake.

After `home-manager switch` / `nixos-rebuild switch`, if this graphical
session was already running the unit is installed but not started:

```bash
systemctl --user daemon-reload
systemctl --user start voicein
voicein status   # idle
```

A path pin (`voicein.url = "path:/…/voicein"`) needs
`nix flake update voicein` after you pull, then rebuild. Otherwise
the live unit keeps the old store path.

### One-shot without Home Manager

Models and group `input` still required.

```bash
nix run github:maplevoid/voicein -- daemon
```

Without models this exits immediately:

```text
model ~/.local/share/voicein/models/tokens.txt: no such file or directory
```

## 4. Swallow the chord

Niri (or any compositor) does not start recording. Bind the same chord
to a no-op so the key is not typed. `cancel` is still a CLI spawn.

```kdl
binds {
    Shift+Alt+V repeat=false hotkey-overlay-title="Voice input" { spawn "true"; }
    Shift+Alt+C hotkey-overlay-title="Voice input: cancel" { spawn "voicein" "cancel"; }
}
```

`hotkey = ""` disables evdev. `toggle` / `down` / `up` still work from
a terminal or a compositor bind.

## 5. Check it

```bash
id                 # must list input
voicein status     # idle
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
| `model .../tokens.txt: no such file or directory` | step 1 skipped |
| `daemon not running (.../voicein.sock)` | unit not started in this session |
| hotkey does nothing, journal `hotkey listen:` | not in group `input`, or no logout since adding it |
| letter still types into the window | no swallow bind |
| `pw-record: command not found` | PipeWire not on `PATH` |
| HUD flashes red, no notification | decode / record failed |
| text on clipboard, one `notify-send` | inject failed (`notify = false` silences that) |
| `unknown command "down"` | old binary; rebuild / `nix flake update voicein` |

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
