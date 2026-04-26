// Copyright (c) 2026 Cielavenir
// SPDX-License-Identifier: GPL-3.0-only

package util

import "strings"

// Level represents a log severity level.
type Level int

const (
	LevelDebug     Level = 0
	LevelInfo      Level = 1
	LevelNotice    Level = 2
	LevelWarn      Level = 3
	LevelError     Level = 4
	LevelCritical  Level = 5
	LevelFatal     Level = 6
	LevelEmergency Level = 7
)

// ParseLevel converts a level string to a Level value. Unknown strings
// default to LevelInfo.
func ParseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "debug", "trace":
		return LevelDebug
	case "info", "information", "informational":
		return LevelInfo
	case "notice":
		return LevelNotice
	case "warn", "warning":
		return LevelWarn
	case "error", "err":
		return LevelError
	case "critical", "crit", "fatal", "severe":
		return LevelCritical
	case "panic":
		return LevelFatal
	case "emergency", "emerg", "alert":
		return LevelEmergency
	default:
		return LevelInfo
	}
}

// String returns the canonical string representation of l.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelNotice:
		return "notice"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	case LevelCritical:
		return "critical"
	case LevelFatal:
		return "fatal"
	case LevelEmergency:
		return "emergency"
	default:
		return "unknown"
	}
}

// Less returns true if a is less severe than b.
func (a Level) Less(b Level) bool {
	return int(a) < int(b)
}
