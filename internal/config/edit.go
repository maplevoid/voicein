package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type kind int

const (
	kindString kind = iota
	kindBool
	kindInt
	kindDuration
	kindArray
)

type spec struct {
	Name    string
	Section string
	Key     string
	Kind    kind
}

var voiceinKeys = []spec{
	{Name: "socket", Key: "socket", Kind: kindString},
	{Name: "sample_rate", Key: "sample_rate", Kind: kindInt},
	{Name: "silence", Key: "silence", Kind: kindDuration},
	{Name: "max_record", Key: "max_record", Kind: kindDuration},
	{Name: "notify", Key: "notify", Kind: kindBool},
	{Name: "mode", Key: "mode", Kind: kindString},
	{Name: "tap", Key: "tap", Kind: kindDuration},
	{Name: "hotkey", Key: "hotkey", Kind: kindString},
	{Name: "scribe.socket", Section: "scribe", Key: "socket", Kind: kindString},
	{Name: "record.command", Section: "record", Key: "command", Kind: kindArray},
	{Name: "inject.copy", Section: "inject", Key: "copy", Kind: kindArray},
	{Name: "inject.paste", Section: "inject", Key: "paste", Kind: kindArray},
	{Name: "inject.type", Section: "inject", Key: "type", Kind: kindArray},
	{Name: "inject.x_copy", Section: "inject", Key: "x_copy", Kind: kindArray},
	{Name: "inject.x_paste", Section: "inject", Key: "x_paste", Kind: kindArray},
	{Name: "inject.x_type", Section: "inject", Key: "x_type", Kind: kindArray},
	{Name: "inject.notify", Section: "inject", Key: "notify", Kind: kindArray},
	{Name: "hud.enabled", Section: "hud", Key: "enabled", Kind: kindBool},
	{Name: "hud.width", Section: "hud", Key: "width", Kind: kindInt},
	{Name: "hud.height", Section: "hud", Key: "height", Kind: kindInt},
	{Name: "hud.margin", Section: "hud", Key: "margin", Kind: kindInt},
	{Name: "hud.layer", Section: "hud", Key: "layer", Kind: kindString},
}

func canonicalVoicein(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	switch name {
	case "scribe":
		return "scribe.socket"
	default:
		return name
	}
}

func lookupVoicein(name string) (spec, bool) {
	name = canonicalVoicein(name)
	for _, s := range voiceinKeys {
		if s.Name == name {
			return s, true
		}
	}
	return spec{}, false
}

func Keys() []string {
	out := make([]string, len(voiceinKeys))
	for i, s := range voiceinKeys {
		out[i] = s.Name
	}
	return out
}

func Get(c Config, name string) (string, error) {
	sp, ok := lookupVoicein(name)
	if !ok {
		return "", unknownKey(name, Keys())
	}
	return formatVoicein(sp, normalize(c)), nil
}

func List(c Config) string {
	c = normalize(c)
	var b strings.Builder
	for _, sp := range voiceinKeys {
		fmt.Fprintf(&b, "%s = %s\n", sp.Name, formatVoicein(sp, c))
	}
	return b.String()
}

func Set(name, raw string) error {
	return SetPath(Path(), name, raw)
}

func SetPath(path, name, raw string) error {
	sp, ok := lookupVoicein(name)
	if !ok {
		return unknownKey(name, Keys())
	}
	raw = strings.TrimSpace(raw)
	cfg := Defaults()
	if err := assign(&cfg, sp.Section, sp.Key, raw); err != nil {
		return fmt.Errorf("%s: %w", sp.Name, err)
	}
	if sp.Name == "mode" {
		m := strings.ToLower(strings.TrimSpace(cfg.Mode))
		switch m {
		case "hybrid", "toggle", "hold", "ptt", "push", "both", "tap":
		default:
			return fmt.Errorf("mode must be hybrid, toggle, or hold")
		}
	}
	if sp.Name == "hud.layer" {
		l := strings.ToLower(strings.TrimSpace(cfg.HUD.Layer))
		switch l {
		case "overlay", "top":
		default:
			return fmt.Errorf("hud.layer must be overlay or top")
		}
	}
	encoded := encodeVoicein(sp, cfg)
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		data = nil
	}
	out := patchTOML(data, sp.Section, sp.Key, encoded)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func normalize(c Config) Config {
	if c.SampleRate <= 0 {
		c.SampleRate = 16000
	}
	if c.Silence <= 0 {
		c.Silence = 3 * time.Second
	}
	if c.MaxRecord <= 0 {
		c.MaxRecord = 60 * time.Second
	}
	if c.Tap <= 0 {
		c.Tap = 300 * time.Millisecond
	}
	if c.HUD.Width <= 0 {
		c.HUD.Width = 77
	}
	if c.HUD.Height <= 0 {
		c.HUD.Height = 36
	}
	if c.HUD.Layer == "" {
		c.HUD.Layer = "overlay"
	}
	if c.Socket == "" {
		c.Socket = Defaults().Socket
	}
	if c.Scribe.Socket != "" {
		c.ScribeSocket = c.Scribe.Socket
	}
	if c.ScribeSocket == "" {
		c.ScribeSocket = Defaults().ScribeSocket
	}
	return c
}

func formatVoicein(sp spec, c Config) string {
	switch sp.Name {
	case "socket":
		return strconv.Quote(c.Socket)
	case "sample_rate":
		return strconv.Itoa(c.SampleRate)
	case "silence":
		return strconv.Quote(formatDuration(c.Silence))
	case "max_record":
		return strconv.Quote(formatDuration(c.MaxRecord))
	case "notify":
		return strconv.FormatBool(c.Notify)
	case "mode":
		return strconv.Quote(c.RecordMode())
	case "tap":
		return strconv.Quote(formatDuration(c.Tap))
	case "hotkey":
		return strconv.Quote(c.Hotkey)
	case "scribe.socket":
		return strconv.Quote(c.ScribeSocket)
	case "record.command":
		return formatArray(c.Record.Command)
	case "inject.copy":
		return formatArray(c.Inject.Copy)
	case "inject.paste":
		return formatArray(c.Inject.Paste)
	case "inject.type":
		return formatArray(c.Inject.Type)
	case "inject.x_copy":
		return formatArray(c.Inject.XCopy)
	case "inject.x_paste":
		return formatArray(c.Inject.XPaste)
	case "inject.x_type":
		return formatArray(c.Inject.XType)
	case "inject.notify":
		return formatArray(c.Inject.Notify)
	case "hud.enabled":
		return strconv.FormatBool(c.HUD.Enabled)
	case "hud.width":
		return strconv.Itoa(c.HUD.Width)
	case "hud.height":
		return strconv.Itoa(c.HUD.Height)
	case "hud.margin":
		return strconv.Itoa(c.HUD.Margin)
	case "hud.layer":
		return strconv.Quote(c.HUD.Layer)
	default:
		return ""
	}
}

func encodeVoicein(sp spec, c Config) string {
	if sp.Name == "mode" {
		return strconv.Quote(c.Mode)
	}
	return formatVoicein(sp, c)
}

func formatArray(items []string) string {
	if items == nil {
		return "[]"
	}
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = strconv.Quote(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func unknownKey(name string, known []string) error {
	return fmt.Errorf("unknown key %q\nknown: %s", name, strings.Join(known, ", "))
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0"
	}
	if d%time.Hour == 0 {
		return strconv.FormatInt(int64(d/time.Hour), 10) + "h"
	}
	if d%time.Minute == 0 {
		return strconv.FormatInt(int64(d/time.Minute), 10) + "m"
	}
	if d%time.Second == 0 {
		return strconv.FormatInt(int64(d/time.Second), 10) + "s"
	}
	if d%time.Millisecond == 0 {
		return strconv.FormatInt(int64(d/time.Millisecond), 10) + "ms"
	}
	return d.String()
}

func patchTOML(data []byte, section, key, encoded string) []byte {
	text := string(data)
	lines := []string{}
	if text != "" {
		lines = strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	}
	current := ""
	sectionStart := -1
	lastInSection := -1
	found := -1
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			current = strings.TrimSpace(line[1 : len(line)-1])
			if current == section {
				sectionStart = i
				lastInSection = i
			}
			continue
		}
		k, _, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if current == section {
			lastInSection = i
			if k == key {
				found = i
			}
		}
	}
	newline := key + " = " + encoded
	if found >= 0 {
		lines[found] = replaceValue(lines[found], encoded)
		return joinTOML(lines)
	}
	if section == "" {
		insertAt := 0
		for insertAt < len(lines) {
			s := strings.TrimSpace(lines[insertAt])
			if s == "" || strings.HasPrefix(s, "#") {
				insertAt++
				continue
			}
			break
		}
		if lastInSection >= 0 {
			insertAt = lastInSection + 1
		}
		lines = insertLine(lines, insertAt, newline)
		return joinTOML(lines)
	}
	if sectionStart >= 0 {
		lines = insertLine(lines, lastInSection+1, newline)
		return joinTOML(lines)
	}
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		lines = append(lines, "")
	}
	lines = append(lines, "["+section+"]", newline)
	return joinTOML(lines)
}

func insertLine(lines []string, at int, line string) []string {
	if at < 0 {
		at = 0
	}
	if at >= len(lines) {
		return append(lines, line)
	}
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:at]...)
	out = append(out, line)
	out = append(out, lines[at:]...)
	return out
}

func replaceValue(line, encoded string) string {
	left := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(left)]
	keyPart, rest, ok := strings.Cut(left, "=")
	if !ok {
		return indent + left + " = " + encoded
	}
	_, pad, comment := splitValueComment(strings.TrimLeft(rest, " \t"))
	out := indent + keyPart + "= " + encoded
	if comment != "" {
		out += pad + comment
	}
	return out
}

func splitValueComment(rest string) (value, pad, comment string) {
	inQ := byte(0)
	hash := -1
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if inQ != 0 {
			if c == inQ && (i == 0 || rest[i-1] != '\\') {
				inQ = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			inQ = c
			continue
		}
		if c == '#' {
			hash = i
			break
		}
	}
	if hash < 0 {
		return strings.TrimSpace(rest), "", ""
	}
	before := rest[:hash]
	value = strings.TrimSpace(before)
	pad = before[len(strings.TrimRight(before, " \t")):]
	return value, pad, rest[hash:]
}

func joinTOML(lines []string) []byte {
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return []byte{}
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}
