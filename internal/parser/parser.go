// Copyright (c) 2026 Cielavenir
// SPDX-License-Identifier: GPL-3.0-only

// Package parser provides format-specific log line parsers. The Router type
// selects the appropriate parser based on a format hint or content inspection.
package parser

import (
	"bytes"
	"encoding/json"
	"time"
)

// Event represents a parsed log event — a flat key/value map with a few
// well-known conventional keys.
type Event map[string]interface{}

// Well-known conventional keys. Not all parsers populate every field.
const (
	KeyTimestamp = "timestamp"
	KeyLevel     = "level"
	KeyMessage   = "message"
	KeySource    = "source"
	KeyHost      = "host"
	KeyRaw       = "raw"
)

// GetSource returns the source field, or "" if absent.
func (ev Event) GetSource() string {
	if v, ok := ev[KeySource].(string); ok {
		return v
	}
	return ""
}

// Clone returns a shallow copy of the event.
func (ev Event) Clone() Event {
	if ev == nil {
		return nil
	}
	out := make(Event, len(ev))
	for k, v := range ev {
		out[k] = v
	}
	return out
}

// Router dispatches raw input lines to the correct format-specific parser.
type Router struct {
	format string // forced format, or "auto"
}

// NewRouter creates a parser router with the given format hint.
func NewRouter(format string) *Router {
	return &Router{format: format}
}

// Parse parses a raw input line and returns an Event.
func (r *Router) Parse(raw []byte, sourceName string) Event {
	switch r.format {
	case "json":
		return parseJSON(raw, sourceName)
	case "syslog":
		return parseSyslog(raw, sourceName)
	case "nginx":
		return parseNginx(raw, sourceName)
	case "raw":
		return Event{KeyRaw: string(raw), KeySource: sourceName}
	case "auto":
		if len(raw) > 0 && raw[0] == '{' {
			if ev := parseJSON(raw, sourceName); ev != nil {
				return ev
			}
		}
		if ev := parseSyslog(raw, sourceName); ev != nil {
			return ev
		}
		return Event{KeyMessage: string(raw), KeySource: sourceName, KeyRaw: string(raw)}
	default:
		return Event{KeyMessage: string(raw), KeySource: sourceName, KeyRaw: string(raw)}
	}
}

// parseJSON decodes a JSON Lines record. Returns nil on parse failure.
func parseJSON(raw []byte, sourceName string) Event {
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	if m == nil {
		return nil
	}
	ev := Event(m)
	// Normalise common field name aliases.
	normaliseField(ev, "msg", KeyMessage)
	normaliseField(ev, "log", KeyMessage)
	normaliseField(ev, "level", KeyLevel)
	normaliseField(ev, "lvl", KeyLevel)
	normaliseField(ev, "ts", KeyTimestamp)
	normaliseField(ev, "time", KeyTimestamp)
	normaliseField(ev, "logger", KeySource)
	if _, ok := ev[KeySource]; !ok {
		ev[KeySource] = sourceName
	}
	if _, ok := ev[KeyRaw]; !ok {
		ev[KeyRaw] = string(raw)
	}
	return ev
}

// normaliseField copies src to dst if src exists and dst is absent.
func normaliseField(ev Event, src, dst string) {
	if _, ok := ev[dst]; ok {
		return
	}
	if v, ok := ev[src]; ok {
		ev[dst] = v
	}
}

// MustTimestamp parses s as a time string and returns a time.Time. It tries
// common layouts before returning the zero time.
func MustTimestamp(s string) time.Time {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		"2006-01-02T15:04:05.000Z07:00",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05",
		"Jan _2 15:04:05",
		"2006/01/02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// IsJSON reports whether b appears to be a JSON object.
func IsJSON(b []byte) bool {
	trimmed := bytes.TrimSpace(b)
	return len(trimmed) > 0 && trimmed[0] == '{'
}
