package cli

import (
	"fmt"
	"io"

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
	if d.Load == nil {
		d.Load = config.Load
	}
	if d.Call == nil {
		d.Call = ipc.Call
	}
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
		cfg, err := d.Load()
		if err != nil {
			return err
		}
		return d.Daemon(cfg)
	case "toggle", "cancel", "status", "quit":
		cfg, err := d.Load()
		if err != nil {
			return err
		}
		reply, err := d.Call(cfg.Socket, ipc.Command(cmd))
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
		fmt.Fprint(d.Stdout, config.Example())
		return nil
	case "help", "-h", "--help":
		fmt.Fprint(d.Stdout, Usage())
		return nil
	default:
		return fmt.Errorf("unknown command %q\n\n%s", cmd, Usage())
	}
}

func Usage() string {
	return `voicein - push-to-toggle speech-to-text for Niri

Commands:
  daemon    stay resident, load SenseVoice, listen on the unix socket
  toggle    start recording, or stop and transcribe
  cancel    discard the current take
  status    print idle | recording | transcribing
  quit      stop the daemon
  config    print an example config.toml

Config: $XDG_CONFIG_HOME/voicein/config.toml
Models: $XDG_DATA_HOME/voicein/models
Socket: $XDG_RUNTIME_DIR/voicein.sock
`
}
