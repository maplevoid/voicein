[English](README.md) · [中文](README.zh.md)

# voicein

Push-to-talk speech-to-text for Linux.

The daemon owns the hotkey via `/dev/input/event*`. Recognition is
[scribe](https://github.com/maplevoid/scribe). This process records,
shows a HUD, and injects text into the focused window.

```bash
scribe fetch-model
scribe serve &
voicein daemon
```

Hold `Shift+Alt+V` and speak. Tap (<300ms) latches; press again to
stop. Hold past 300ms and release to stop. `voicein cancel` discards
the take.

## Requirements

- Linux (`x86_64` / `aarch64`)
- PipeWire (`pw-record`)
- group `input` (hotkey reads `/dev/input/event*`)
- [scribe](https://github.com/maplevoid/scribe) on the same machine
- Wayland: `wl-copy`, `wtype` (HUD needs layer-shell)
- X11: `xclip`, `xdotool` (no HUD)

Not tied to niri or any compositor. A compositor bind is optional:
only needed if the chord would otherwise type into the focused window.

## Install

### Binary

Tag a `v*` release and GitHub Actions uploads:

| Arch | Archive |
| --- | --- |
| `x86_64` | `voicein-linux-amd64.tar.gz` |
| `aarch64` | `voicein-linux-arm64.tar.gz` |

```bash
curl -fsSL -o voicein.tar.gz \
  https://github.com/maplevoid/voicein/releases/latest/download/voicein-linux-amd64.tar.gz
tar -xzf voicein.tar.gz
sudo install -m 755 voicein-linux-amd64/voicein /usr/local/bin/voicein
```

voicein is a static Go binary. It still needs [scribe](https://github.com/maplevoid/scribe)
on the same machine (binary or source), PipeWire, group `input`, and
the inject tools below.

### From source

```bash
# 1. scribe (transcriber) — binary or:
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

Packages to have on `PATH`:

| Distro | Packages |
| --- | --- |
| Debian / Ubuntu | `pipewire-bin wl-clipboard wtype xclip xdotool libnotify-bin` |
| Fedora | `pipewire-utils wl-clipboard wtype xclip xdotool libnotify` |
| Arch | `pipewire wl-clipboard wtype xclip xdotool libnotify` |

Add yourself to group `input`, then log out and back in:

```bash
sudo usermod -aG input "$USER"
id   # must list input
```

On Debian/Ubuntu the device nodes are often `root:input` with mode
`660`. On some setups they stay `root:root` `600` — then the hotkey
will not work even after joining the group. Check:

```bash
ls -l /dev/input/event0
```

## Run

```bash
scribe fetch-model          # SenseVoice into ~/.local/share/scribe/models
scribe serve &              # or the systemd socket below
voicein daemon
voicein status              # idle
```

Without a local model, use Groq or OpenAI in scribe's config instead
of `fetch-model`. Without `scribe serve` (or `scribe.socket`), a take
ends with a HUD flash and no text.

systemd user units, if you are not using Nix:

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

If this graphical session was already running, start the units by
hand after enabling them.

## Nix

```bash
nix run github:maplevoid/scribe -- fetch-model
nix run github:maplevoid/voicein -- daemon
```

Home Manager (NixOS or standalone):

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

On NixOS, still add group `input` yourself:

```nix
users.users.<name>.extraGroups = [ "input" ];
```

Then rebuild **and log out**. `id` must show `input`. The Home Manager
modules do not add the group, do not download models, and wrap
`wl-copy` / `wtype` / `xclip` / `xdotool` / `notify-send`. They use
the host's `pw-record`.

## Record modes

- `hybrid` (default): press starts recording. A tap shorter than
  `tap` (300ms) latches like toggle; hold past that threshold stops
  on release. A later press stops a latched take.
- `toggle`: press to start, press again (or hit 60s) to transcribe.
- `hold`: hold; release (or hit 60s) to transcribe.

Thinking pauses do not end the take. The first keypress also warms
scribe (empty PCM). The hotkey does nothing while transcribing; use
`cancel`.

`hotkey = ""` disables evdev. `toggle` / `down` / `up` still work
from a terminal or a compositor bind.

## Optional: keep the chord out of the window

The compositor does not start recording. The daemon reads the chord
from evdev with no `EVIOCGRAB`, so an unbound `Shift+Alt+V` still
reaches the focused window (Ghostty / terminals often paste). Bind the
same keys to a no-op to swallow them.

Do not bind V to `voicein toggle`: evdev already latches, and a
compositor spawn would fire twice. Bind C to `voicein cancel`; the
daemon has no evdev cancel chord.

Hyprland:

```conf
bind = SHIFT ALT, V, exec, true
bind = SHIFT ALT, C, exec, voicein cancel
```

niri (default `mod-key` Super):

```kdl
binds {
    Shift+Alt+V repeat=false { spawn "true"; }
    Shift+Alt+C { spawn "voicein" "cancel"; }
}
```

If `mod-key "Alt"`, physical `Shift+Alt+V` / `Shift+Alt+C` are
`Mod+Shift+V` / `Mod+Shift+C`. Write `Mod+`, not `Shift+Alt+`:

```kdl
binds {
    Mod+Shift+V repeat=false { spawn "true"; }
    Mod+Shift+C { spawn "voicein" "cancel"; }
}
```

Sway / i3: bind the same keys to `true` / `voicein cancel`.

## Config

Optional. Missing files use built-in defaults.

voicein reads `~/.config/voicein/config.toml`. Engine, language, and
idle live in `~/.config/scribe/config.toml`.

```bash
voicein config > ~/.config/voicein/config.toml
scribe config  > ~/.config/scribe/config.toml
voicein config set hotkey "shift+alt+v"
scribe config set engine sensevoice
```

```toml
socket = ""                 # default: $XDG_RUNTIME_DIR/voicein.sock
sample_rate = 16000
max_record = "60s"
notify = true
mode = "hybrid"             # hybrid | toggle | hold
tap = "300ms"
hotkey = "shift+alt+v"      # evdev; empty disables

[scribe]
socket = ""                 # default: $XDG_RUNTIME_DIR/scribe.sock
```

## Online transcription

Same daemons. In `~/.config/scribe/config.toml`:

```toml
[model]
engine = "groq"             # or "openai"
```

```bash
export GROQ_API_KEY=gsk_...
# already-running user unit:
# systemctl --user edit scribe
# [Service]
# Environment=GROQ_API_KEY=gsk_...
systemctl --user restart scribe.socket
```

`openai` uses `OPENAI_API_KEY`. Either engine also accepts
`SCRIBE_API_KEY`. Do not put keys in the toml.

## Inject

Session type picks the tools, not the focused window:

- Wayland (`wl-copy` + `wtype`): copy, then Ctrl+V; fallback is
  type-from-stdin.
- X11 (`xclip` + `xdotool`): `XDG_SESSION_TYPE=x11`, or `DISPLAY`
  set and `WAYLAND_DISPLAY` empty.

Wayland sessions with an XWayland focused window still use `wtype`.
Text is always copied first. Paste failure leaves it on the clipboard.

HUD needs a Wayland layer-shell compositor. Plain X11 can inject, with
no bar.

## Check it

```bash
id                          # must list input
voicein status              # idle
journalctl --user -u scribe -u voicein -e
```

A healthy start looks like:

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

| Symptom | Cause |
| --- | --- |
| `model .../tokens.txt: no such file or directory` | no `scribe fetch-model` |
| `daemon not running (.../voicein.sock)` | daemon / unit not started |
| hotkey does nothing, journal `hotkey listen:` | not in group `input`, or no logout |
| letter still types into the window | no compositor no-op bind |
| `pw-record: command not found` | PipeWire not on `PATH` |
| HUD flashes red, no notification | decode / record failed; check scribe logs |
| text on clipboard, one `notify-send` | inject failed |
| take ends, no text, scribe not listening | `scribe serve` / `scribe.socket` not running |

## Develop

```bash
go test ./...
go build -o voicein ./cmd/voicein
```

Or `nix develop`.
