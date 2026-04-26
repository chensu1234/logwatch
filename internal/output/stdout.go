// Copyright (c) 2026 Cielavenir
// SPDX-License-Identifier: GPL-3.0-only

package output

import (
	"encoding/json"
	"os"
	"strings"

	"logwatch/internal/config"
	"logwatch/internal/parser"
)

// Stdout writes parsed events as coloured JSON or human-readable lines.
type Stdout struct {
	colored bool
}

// NewStdout creates a stdout writer.
func NewStdout(cfg config.OutputConfig, _ Logger) *Stdout {
	return &Stdout{colored: cfg.Color}
}

// Write emits a single event to stdout.
func (s *Stdout) Write(ev parser.Event) error {
	var b []byte
	var err error
	if s.colored {
		b, err = s.colouredJSON(ev)
	} else {
		b, err = json.MarshalIndent(ev, "", "  ")
	}
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(append(b, '\n'))
	return err
}

// colouredJSON returns a pretty-printed, ANSI-coloured JSON representation.
func (s *Stdout) colouredJSON(ev parser.Event) ([]byte, error) {
	level, _ := ev["level"].(string)
	var esc string
	switch strings.ToLower(level) {
	case "error", "err", "critical", "crit", "fatal", "panic", "emergency", "emerg":
		esc = "\x1b[31m" // red
	case "warn", "warning":
		esc = "\x1b[33m" // yellow
	case "info", "notice":
		esc = "\x1b[36m" // cyan
	case "debug", "trace":
		esc = "\x1b[34m" // blue
	default:
		esc = "\x1b[0m" // reset
	}

	plain, err := json.MarshalIndent(ev, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(append([]byte(esc), plain...), '\x1b', '0', '\n'), nil
}

func (s *Stdout) Flush() error { return nil }
func (s *Stdout) Close() error { return nil }
