// Copyright (c) 2026 Cielavenir
// SPDX-License-Identifier: GPL-3.0-only

// Package output provides event sink implementations for various backends.
// All writers satisfy the io.WriterCloser interface.
package output

import (
	"logwatch/internal/config"
	"logwatch/internal/parser"
)

// Logger is the minimal logging interface used by output drivers.
type Logger interface {
	Info(format string, args ...any)
	Warn(format string, args ...any)
	Error(format string, args ...any)
}

// Writer is the interface implemented by all output drivers.
type Writer interface {
	Write(ev parser.Event) error
	Flush() error
	Close() error
}

// BuildAll creates a Writer instance for each output config entry.
func BuildAll(configs []config.OutputConfig, log Logger) ([]Writer, error) {
	var writers []Writer
	for _, cfg := range configs {
		w, err := buildOne(cfg, log)
		if err != nil {
			return nil, err
		}
		writers = append(writers, w)
	}
	return writers, nil
}

func buildOne(cfg config.OutputConfig, log Logger) (Writer, error) {
	switch cfg.Type {
	case "stdout":
		return NewStdout(cfg, log), nil
	case "file":
		return NewFile(cfg, log)
	case "elasticsearch":
		return NewElasticsearch(cfg, log)
	case "kafka":
		return NewKafka(cfg, log)
	case "webhook":
		return NewWebhook(cfg, log)
	default:
		return nil, &BuildError{Type: cfg.Type}
	}
}

// BuildError is returned when an output type cannot be instantiated.
type BuildError struct{ Type string }

func (e *BuildError) Error() string {
	return "output: unknown type " + e.Type
}
