// Copyright (c) 2026 Cielavenir
// SPDX-License-Identifier: GPL-3.0-only

package util

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Logger is a structured logger compatible with the internal output.Logger interface.
type Logger struct {
	h     slog.Handler
	minLv slog.Level
	mu    sync.Mutex
	attrs []any
}

// NewLogger creates a Logger that writes to stderr with the given minimum level.
func NewLogger(level string) *Logger {
	var lv slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lv = slog.LevelDebug
	case "info":
		lv = slog.LevelInfo
	case "warn", "warning":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}

	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: lv,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{Key: a.Key, Value: slog.StringValue(
					a.Value.Time().Format("15:04:05.000"))}
			}
			return a
		},
	})
	return &Logger{h: h, minLv: lv}
}

// WithFields returns a new Logger that includes the given key-value pairs
// in every subsequent log call.
func (l *Logger) WithFields(fields ...any) *Logger {
	l.mu.Lock()
	attrs := append(l.attrs[:len(l.attrs):len(l.attrs)], fields...)
	l.mu.Unlock()
	return &Logger{h: l.h, minLv: l.minLv, attrs: attrs}
}

// Debug logs at debug level.
func (l *Logger) Debug(format string, args ...any) {
	l.log(slog.LevelDebug, format, args...)
}

// Info logs at info level.
func (l *Logger) Info(format string, args ...any) {
	l.log(slog.LevelInfo, format, args...)
}

// Warn logs at warning level.
func (l *Logger) Warn(format string, args ...any) {
	l.log(slog.LevelWarn, format, args...)
}

// Error logs at error level.
func (l *Logger) Error(format string, args ...any) {
	l.log(slog.LevelError, format, args...)
}

// log formats the message and emits it through the underlying handler.
func (l *Logger) log(lv slog.Level, format string, args ...any) {
	msg := sprintf(format, args)
	l.mu.Lock()
	h := l.h
	attrs := l.attrs
	l.mu.Unlock()

	var pcs [1]uintptr
	runtime.Callers(3, pcs[:])
	r := slog.Record{Time: time.Now(), Level: lv, Message: msg, PC: pcs[0]}
	r.AddAttrs(attrsToAttrValues(attrs)...)
	h.Handle(context.Background(), r)
}

// contextBg returns a background context.

// AttrsToAttrValues converts a flat key-value slice to slog.Attr values.
func AttrsToAttrValues(attrs []any) []slog.Attr {
	var out []slog.Attr
	for i := 0; i < len(attrs); i += 2 {
		if i+1 < len(attrs) {
			key, _ := attrs[i].(string)
			out = append(out, slog.Any(key, attrs[i+1]))
		}
	}
	return out
}

func attrsToAttrValues(attrs []any) []slog.Attr { return AttrsToAttrValues(attrs) }

// sprintf formats a message with args using simple printf-style replacement.
func sprintf(format string, args []any) string {
	if len(args) == 0 {
		return format
	}
	var b strings.Builder
	b.Grow(len(format) + len(args)*16)
	argIdx := 0
	for i := 0; i < len(format); {
		c := format[i]
		if c != '%' || i+1 >= len(format) {
			b.WriteByte(c)
			i++
			continue
		}
		verb := format[i+1]
		if argIdx < len(args) {
			switch verb {
			case 's':
				if s, ok := args[argIdx].(string); ok {
					b.WriteString(s)
				} else {
					b.WriteString(toString(args[argIdx]))
				}
			case 'd', 'i', 'c':
				b.WriteString(toString(args[argIdx]))
			case 'v':
				b.WriteString(toString(args[argIdx]))
			default:
				b.WriteString(toString(args[argIdx]))
			}
			argIdx++
		}
		i += 2
	}
	return b.String()
}

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int:
		return intToString(x)
	case int64:
		return int64ToString(x)
	case int32:
		return intToString(int(x))
	case uint64:
		return uint64ToString(x)
	case error:
		return x.Error()
	default:
		return "<val>"
	}
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits [20]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		digits[i] = '-'
	}
	return string(digits[i:])
}

func int64ToString(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits [24]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + int(n%10))
		n /= 10
	}
	if neg {
		i--
		digits[i] = '-'
	}
	return string(digits[i:])
}

func uint64ToString(n uint64) string {
	if n == 0 {
		return "0"
	}
	var digits [24]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + int(n%10))
		n /= 10
	}
	return string(digits[i:])
}
