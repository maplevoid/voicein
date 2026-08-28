package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Command string

const (
	CmdToggle Command = "toggle"
	CmdDown   Command = "down"
	CmdUp     Command = "up"
	CmdCancel Command = "cancel"
	CmdStatus Command = "status"
	CmdQuit   Command = "quit"
)

type Request struct {
	Cmd Command `json:"cmd"`
}

type State string

const (
	StateIdle         State = "idle"
	StateRecording    State = "recording"
	StateTranscribing State = "transcribing"
)

type Status struct {
	State   State  `json:"state"`
	Error   string `json:"error,omitempty"`
	Text    string `json:"text,omitempty"`
	Seconds int    `json:"seconds,omitempty"`
}

type Reply struct {
	OK     bool   `json:"ok"`
	Status Status `json:"status"`
	Error  string `json:"error,omitempty"`
}

func Listen(path string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if _, err := Call(path, CmdStatus); err == nil {
		return nil, fmt.Errorf("already running (%s)", path)
	}
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	return ln, nil
}

func Serve(ln net.Listener, handle func(Command) Reply) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go func(c net.Conn) {
			defer c.Close()
			_ = c.SetDeadline(time.Now().Add(2 * time.Second))
			req, err := readRequest(c)
			if err != nil {
				writeReply(c, Reply{OK: false, Error: err.Error()})
				return
			}
			writeReply(c, handle(req.Cmd))
		}(conn)
	}
}

func Call(path string, cmd Command) (Reply, error) {
	conn, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		return Reply{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if err := json.NewEncoder(conn).Encode(Request{Cmd: cmd}); err != nil {
		return Reply{}, err
	}
	var reply Reply
	if err := json.NewDecoder(conn).Decode(&reply); err != nil {
		return Reply{}, err
	}
	return reply, nil
}

func Format(s Status) string {
	switch s.State {
	case StateRecording:
		if s.Seconds > 0 {
			return fmt.Sprintf("recording %ds", s.Seconds)
		}
		return "recording"
	case StateTranscribing:
		return "transcribing"
	default:
		if s.Error != "" {
			return "idle error=" + s.Error
		}
		if s.Text != "" {
			return "idle last=" + s.Text
		}
		return "idle"
	}
}

func readRequest(r io.Reader) (Request, error) {
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && err != io.EOF {
		return Request{}, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return Request{}, fmt.Errorf("empty request")
	}
	var req Request
	if err := json.Unmarshal([]byte(line), &req); err == nil && req.Cmd != "" {
		return req, nil
	}
	req.Cmd = Command(strings.ToLower(line))
	switch req.Cmd {
	case CmdToggle, CmdDown, CmdUp, CmdCancel, CmdStatus, CmdQuit:
		return req, nil
	default:
		return Request{}, fmt.Errorf("unknown command %q", line)
	}
}

func writeReply(w io.Writer, reply Reply) {
	enc := json.NewEncoder(w)
	_ = enc.Encode(reply)
}
