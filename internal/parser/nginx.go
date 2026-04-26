// Copyright (c) 2026 Cielavenir
// SPDX-License-Identifier: GPL-3.0-only

package parser

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Combined Nginx / Apache access log pattern.
// Format: host ident user [timestamp +timezone] "method uri proto" status size "referer" "user_agent"
var accessLogRe = regexp.MustCompile(
	`^(?P<host>\S+)\s+` +
		`(?P<ident>\S+)\s+` +
		`(?P<user>\S+)\s+` +
		`\[(?P<ts>[^\]]+)\]\s+` +
		`"(?P<method>\S+)\s+(?P<uri>\S+)\s+(?P<proto>\S+)"\s+` +
		`(?P<status>\d+)\s+` +
		`(?P<size>\S+)\s+` +
		`"(?P<referer>[^\"]*)"\s+` +
		`"(?P<user_agent>[^\"]*)"` +
		`.*$`,
)

// Nginx error log pattern.
// Format: timestamp [level] pid tid#cid log_message
var errorLogRe = regexp.MustCompile(
	`^(?P<ts>\d{4}/\d{2}/\d{2}\s+\d{2}:\d{2}:\d{2})\s+` +
		`\[(?P<level>\w+)\]\s+` +
		`(?P<pid>\d+).*?:\s+` +
		`(?P<msg>.*)$`,
)

// parseNginx attempts to parse raw as an Nginx access log or error log entry.
// Returns nil if the line doesn't match either pattern.
func parseNginx(raw []byte, sourceName string) Event {
	s := string(raw)

	// Access log: starts with an IP or hostname, contains a timestamp in brackets.
	if strings.HasPrefix(s, "\"") || strings.Count(s, "\"") >= 2 {
		if ev := parseAccessLog(s, sourceName); ev != nil {
			return ev
		}
	}

	// Error log: starts with a date like 2026/04/26.
	if strings.Contains(s, "error") || strings.Contains(s, "warn") ||
		strings.Contains(s, "notice") || strings.Contains(s, "debug") {
		if ev := parseErrorLog(s, sourceName); ev != nil {
			return ev
		}
	}

	return nil
}

func parseAccessLog(s, sourceName string) Event {
	m := accessLogRe.FindStringSubmatch(s)
	if m == nil {
		return nil
	}

	ts := parseAccessLogTime(m[3])

	ev := Event{
		KeyTimestamp: ts,
		KeySource:    sourceName,
		"host":       m[1],
		"ident":      m[2],
		"user":       m[3],
		"method":     m[4],
		"uri":        m[5],
		"proto":      m[6],
		"status":     toInt(m[7]),
		"size":       toSize(m[8]),
		"referer":    m[9],
		"user_agent": m[10],
		KeyRaw:       s,
	}
	ev[KeyMessage] = ev["method"].(string) + " " + ev["uri"].(string)

	status := ev["status"].(int)
	if status >= 500 {
		ev[KeyLevel] = "error"
	} else if status >= 400 {
		ev[KeyLevel] = "warn"
	} else {
		ev[KeyLevel] = "info"
	}
	return ev
}

func parseErrorLog(s, sourceName string) Event {
	m := errorLogRe.FindStringSubmatch(s)
	if m == nil {
		return nil
	}

	ts := parseErrorLogTime(m[1])
	level := m[2]
	if level == "error" {
		level = "err"
	}

	ev := Event{
		KeyTimestamp: ts,
		KeySource:    sourceName,
		KeyLevel:     level,
		"pid":        toInt(m[3]),
		KeyMessage:   m[4],
		KeyRaw:       s,
	}
	return ev
}

// parseAccessLogTime parses Nginx combined log timestamp, e.g.
// "26/Apr/2026:12:00:00 +0000"
func parseAccessLogTime(s string) time.Time {
	// Remove the timezone part to parse with standard layout.
	parts := strings.Split(s, " ")
	if len(parts) < 1 {
		return time.Time{}
	}
	ts := strings.TrimSpace(parts[0])
	layouts := []string{
		"02/Jan/2006:15:04:05 -0700",
		"02/Jan/2006:15:04:05",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, ts); err == nil {
			return t
		}
	}
	return time.Time{}
}

// parseErrorLogTime parses "2026/04/26 12:00:00"
func parseErrorLogTime(s string) time.Time {
	t, err := time.Parse("2006/01/02 15:04:05", s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func toInt(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

func toSize(s string) int {
	s = strings.TrimSpace(s)
	if s == "-" || s == "" {
		return 0
	}
	v, _ := strconv.ParseInt(s, 10, 64)
	return int(v)
}
