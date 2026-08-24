package main

import (
	"fmt"
	"os"

	"github.com/zway/voicein/internal/config"
	"github.com/zway/voicein/internal/daemon"
	"github.com/zway/voicein/internal/ipc"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cmd := "help"
	if len(args) > 0 {
		cmd = args[0]
	}
	switch cmd {
	case "daemon":
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		return daemon.Run(cfg)
	case "toggle", "cancel", "status", "quit":
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		reply, err := ipc.Call(cfg.Socket, ipc.Command(cmd))
		if err != nil {
			return fmt.Errorf("daemon not running (%s): %w", cfg.Socket, err)
		}
		if !reply.OK {
			if reply.Error != "" {
				return fmt.Errorf("%s", reply.Error)
			}
			return fmt.Errorf("%s", ipc.Format(reply.Status))
		}
		fmt.Println(ipc.Format(reply.Status))
		return nil
	case "config":
		fmt.Print(config.Example())
		return nil
	case "help", "-h", "--help":
		fmt.Print(usage())
		return nil
	default:
		return fmt.Errorf("unknown command %q\n\n%s", cmd, usage())
	}
}

func usage() string {
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
