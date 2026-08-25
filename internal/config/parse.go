package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// applyTOML overlays a small TOML subset onto cfg.
// Enough for voicein's config: scalars, durations, string arrays, one-level tables.
func applyTOML(data []byte, cfg *Config) error {
	section := ""
	lines := strings.Split(string(data), "\n")
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("line %d: expected key = value", i+1)
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if hash := strings.Index(val, " #"); hash >= 0 {
			val = strings.TrimSpace(val[:hash])
		}
		if err := assign(cfg, section, key, val); err != nil {
			return fmt.Errorf("line %d: %w", i+1, err)
		}
	}
	return nil
}

func assign(cfg *Config, section, key, val string) error {
	switch section {
	case "":
		switch key {
		case "socket":
			cfg.Socket, _ = unquote(val)
		case "sample_rate":
			n, err := strconv.Atoi(val)
			if err != nil {
				return err
			}
			cfg.SampleRate = n
		case "silence":
			d, err := parseDuration(val)
			if err != nil {
				return err
			}
			cfg.Silence = d
		case "max_record":
			d, err := parseDuration(val)
			if err != nil {
				return err
			}
			cfg.MaxRecord = d
		case "language":
			cfg.Language, _ = unquote(val)
		case "itn":
			b, err := strconv.ParseBool(val)
			if err != nil {
				return err
			}
			cfg.ITN = b
		case "threads":
			n, err := strconv.Atoi(val)
			if err != nil {
				return err
			}
			cfg.Threads = n
		case "notify":
			b, err := strconv.ParseBool(val)
			if err != nil {
				return err
			}
			cfg.Notify = b
		}
	case "model":
		s, _ := unquote(val)
		switch key {
		case "dir":
			cfg.Model.Dir = s
		case "engine":
			cfg.Model.Engine = s
		case "onnx":
			cfg.Model.Onnx = s
		case "encoder":
			cfg.Model.Encoder = s
		case "decoder":
			cfg.Model.Decoder = s
		case "tokens":
			cfg.Model.Tokens = s
		case "vad":
			cfg.Model.VAD = s
		}
	case "record":
		if key == "command" {
			arr, err := parseStringArray(val)
			if err != nil {
				return err
			}
			cfg.Record.Command = arr
		}
	case "inject":
		arr, err := parseStringArray(val)
		if err != nil && (key == "copy" || key == "paste" || key == "type" || key == "x_copy" || key == "x_paste" || key == "x_type" || key == "notify") {
			return err
		}
		switch key {
		case "copy":
			cfg.Inject.Copy = arr
		case "paste":
			cfg.Inject.Paste = arr
		case "type":
			cfg.Inject.Type = arr
		case "x_copy":
			cfg.Inject.XCopy = arr
		case "x_paste":
			cfg.Inject.XPaste = arr
		case "x_type":
			cfg.Inject.XType = arr
		case "notify":
			cfg.Inject.Notify = arr
		}
	case "hud":
		switch key {
		case "enabled":
			b, err := strconv.ParseBool(val)
			if err != nil {
				return err
			}
			cfg.HUD.Enabled = b
		case "width", "height", "margin":
			n, err := strconv.Atoi(val)
			if err != nil {
				return err
			}
			switch key {
			case "width":
				cfg.HUD.Width = n
			case "height":
				cfg.HUD.Height = n
			case "margin":
				cfg.HUD.Margin = n
			}
		case "layer":
			cfg.HUD.Layer, _ = unquote(val)
		}
	}
	return nil
}

func parseDuration(val string) (time.Duration, error) {
	s, _ := unquote(val)
	return time.ParseDuration(s)
}

func unquote(val string) (string, bool) {
	if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
		return val[1 : len(val)-1], true
	}
	return val, false
}

func parseStringArray(val string) ([]string, error) {
	val = strings.TrimSpace(val)
	if !strings.HasPrefix(val, "[") || !strings.HasSuffix(val, "]") {
		return nil, fmt.Errorf("expected string array")
	}
	inner := strings.TrimSpace(val[1 : len(val)-1])
	if inner == "" {
		return []string{}, nil
	}
	var out []string
	var cur strings.Builder
	inQ := false
	for _, r := range inner {
		switch {
		case r == '"':
			inQ = !inQ
		case r == ',' && !inQ:
			s := strings.TrimSpace(cur.String())
			if s != "" {
				out = append(out, s)
			}
			cur.Reset()
		case unicode.IsSpace(r) && !inQ:
			// skip
		default:
			cur.WriteRune(r)
		}
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		out = append(out, s)
	}
	return out, nil
}
