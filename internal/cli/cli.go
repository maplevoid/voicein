package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/zway/voicein/internal/config"
	"github.com/zway/voicein/internal/ipc"
)

type Deps struct {
	Load   func() (config.Config, error)
	Call   func(socket string, cmd ipc.Command) (ipc.Reply, error)
	Daemon func(config.Config) error
	Stdout io.Writer
}

func Run(args []string, d Deps) error {
	if d.Stdout == nil {
		d.Stdout = io.Discard
	}
	cmd := "help"
	if len(args) > 0 {
		cmd = args[0]
	}
	switch cmd {
	case "daemon":
		if d.Daemon == nil {
			return fmt.Errorf("daemon not wired")
		}
		cfg, err := load(d)
		if err != nil {
			return err
		}
		return d.Daemon(cfg)
	case "toggle", "down", "up", "cancel", "status", "quit":
		cfg, err := load(d)
		if err != nil {
			return err
		}
		reply, err := call(d, cfg.Socket, ipc.Command(cmd))
		if err != nil {
			return fmt.Errorf("daemon not running (%s): %w", cfg.Socket, err)
		}
		if !reply.OK {
			if reply.Error != "" {
				return fmt.Errorf("%s", reply.Error)
			}
			return fmt.Errorf("%s", ipc.Format(reply.Status))
		}
		fmt.Fprintln(d.Stdout, ipc.Format(reply.Status))
		return nil
	case "config":
		return configCmd(d, args[1:])
	case "help", "-h", "--help":
		fmt.Fprint(d.Stdout, Usage())
		return nil
	default:
		return fmt.Errorf("unknown command %q\n\n%s", cmd, Usage())
	}
}

func configCmd(d Deps, args []string) error {
	if len(args) == 0 {
		fmt.Fprint(d.Stdout, config.Example())
		return nil
	}
	switch args[0] {
	case "path":
		fmt.Fprintln(d.Stdout, config.Path())
		return nil
	case "get":
		cfg, err := load(d)
		if err != nil {
			return err
		}
		if len(args) == 1 {
			fmt.Fprint(d.Stdout, config.List(cfg))
			return nil
		}
		v, err := config.Get(cfg, args[1])
		if err != nil {
			return err
		}
		fmt.Fprintln(d.Stdout, v)
		return nil
	case "set":
		if len(args) < 3 {
			return fmt.Errorf("usage: voicein config set <key> <value>")
		}
		return config.Set(args[1], strings.Join(args[2:], " "))
	case "example":
		fmt.Fprint(d.Stdout, config.Example())
		return nil
	default:
		return fmt.Errorf("unknown config command %q\n\n%s", args[0], Usage())
	}
}

func load(d Deps) (config.Config, error) {
	if d.Load != nil {
		return d.Load()
	}
	return config.Load()
}

func call(d Deps, socket string, cmd ipc.Command) (ipc.Reply, error) {
	if d.Call != nil {
		return d.Call(socket, cmd)
	}
	return ipc.Call(socket, cmd)
}

func Usage() string {
	return `voicein - push-to-talk speech-to-text for Linux

Commands:
  daemon    stay resident, listen on the unix socket, talk to scribe
  toggle    start recording, or stop and transcribe
  down      start a hold take
  up        finish a hold take and transcribe
  cancel    discard the current take
  status    print idle | recording | transcribing
  quit      stop the daemon
  config    print an example config.toml
  config path
            print the config file path
  config get
            print effective keys
  config get K
            print one key
  config set K V
            write key into the toml (keeps comments)

Config: $XDG_CONFIG_HOME/voicein/config.toml
  mode = "hybrid"   tap latches toggle; hold releases
  mode = "toggle"   press the hotkey to start, press again to stop
  mode = "hold"     hold the hotkey; release to stop
  tap = "300ms"     hybrid only: shorter tap latches
  hotkey = "shift+alt+v"  daemon reads evdev; empty disables it
Socket: $XDG_RUNTIME_DIR/voicein.sock
scribe: $XDG_RUNTIME_DIR/scribe.sock
`
}
