package main

import (
	"fmt"
	"os"

	"github.com/zway/voicein/internal/cli"
	"github.com/zway/voicein/internal/daemon"
)

func main() {
	err := cli.Run(os.Args[1:], cli.Deps{
		Daemon: daemon.Run,
		Stdout: os.Stdout,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
