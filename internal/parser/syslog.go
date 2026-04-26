// Copyright (c) 2026 Cielavenir
// SPDX-License-Identifier: GPL-3.0-only

package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Syslog severity names mapped to their numeric values.
var severityNames = map[string]int{
	"emerg": 0, "emergency": 0, "panic": 0,
	"alert": 1,
	"crit": 2, "critical": 2,
	"err": 3, "error": 3,
	"warn": 4, "warning": 4,
	"notice": 5,
	"info": 6, "information": 6, "informational": 6,
	"debug": 7,
}

// Syslog facility names mapped to their numeric values.
var facilityNames = map[string]int{
	"kern": 0, "kernel": 0,
	"user": 1,
	"mail": 2,
	"daemon": 3,
	"auth": 4, "security": 4,
	"syslog": 9,
	"local0": 16, "local1": 17, "local2": 18, "local3": 19,
	"local4": 20, "local5": 21, "local6": 22, "local7": 23,
}

// RFC 3164 syslog pattern: <pri>timestamp host program: message
// Example: <34>Oct 11 22:14:15 mymachine su: 'su root' failed
var rfc3164Re = regexp.MustCompile(
	`^<(?P<pri>\d+)>(?P<time>\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})\s+` +
		`(?P<host>\S+)\s+(?P<prog>\S+?)(?:\[(?P<pid>\d+)\])?:\s+(?P<msg>.*)$`,
)

// RFC 5424 syslog pattern (without structured data):
// <pri>version timestamp host program pid mid sdata message
// Example: <165>1 2026-04-26T12:00:00.000Z myhost app 12345 - message
var rfc5424Re = regexp.MustCompile(
	`^<(?P<pri>\d+)>(?P<ver>\d+)\s+` +
		`(?P<ts>\S+)\s+` +
		`(?P<host>\S+)\s+` +
		`(?P<prog>\S+?)\s+` +
		`(?P<pid>\d+)\s+` +
		`(?P<mid>\S+)\s+` +
		`(?:\[(?P<sdata>.*?)\]\s+)?` +
		`(?P<msg>.*)$`,
)

// parseSyslog attempts to parse raw as a syslog message (RFC 3164 or RFC 5424).
// Returns nil if the line doesn't match either pattern.
func parseSyslog(raw []byte, sourceName string) Event {
	s := string(raw)
	if len(s) < 4 || s[0] != '<' {
		return nil
	}

	// Try RFC 5424 first (newer standard).
	if ev := parseRFC5424(s, sourceName); ev != nil {
		return ev
	}
	return parseRFC3164(s, sourceName)
}

func parseRFC5424(s, sourceName string) Event {
	m := rfc5424Re.FindStringSubmatch(s)
	if m == nil {
		return nil
	}

	pri, _ := strconv.Atoi(m[1])
	facility := pri / 8
	severity := pri % 8
	ts := MustTimestamp(m[2])

	ev := Event{
		KeyTimestamp: ts,
		KeySource:    sourceName,
		KeyHost:      m[3],
		"program":    m[4],
		"pid":        m[5],
		"message":    m[8],
		"severity":   severity,
		"facility":   facility,
		KeyRaw:       s,
	}
	if msg := m[8]; msg != "" {
		ev[KeyMessage] = msg
	}
	if l := severityName(severity); l != "" {
		ev[KeyLevel] = l
	}
	return ev
}

func parseRFC3164(s, sourceName string) Event {
	m := rfc3164Re.FindStringSubmatch(s)
	if m == nil {
		return nil
	}

	pri, _ := strconv.Atoi(m[1])
	facility := pri / 8
	severity := pri % 8

	// RFC 3164 has no year; use current year.
	ts := parseRFC3164Time(m[2])
	year := time.Now().Year()
	ts = time.Date(year, ts.Month(), ts.Day(), ts.Hour(), ts.Minute(), ts.Second(), 0, ts.Location())

	ev := Event{
		KeyTimestamp: ts,
		KeySource:    sourceName,
		KeyHost:      m[3],
		"program":    m[4],
		KeyMessage:   m[7],
		"severity":   severity,
		"facility":   facility,
		KeyRaw:       s,
	}
	if m[5] != "" {
		ev["pid"] = m[5]
	}
	if l := severityName(severity); l != "" {
		ev[KeyLevel] = l
	}
	return ev
}

// parseRFC3164Time parses "Oct  1 14:05:30" (note variable spacing on day).
func parseRFC3164Time(s string) time.Time {
	// Normalise variable spaces: "Oct  1" -> "Oct  1"
	parts := strings.Fields(s)
	if len(parts) < 3 {
		return time.Time{}
	}
	monthStr := parts[0]
	dayStr := parts[1]
	timeStr := parts[2]

	// Parse day (may be 1 or 2 digits, left-padded with space).
	day := 1
	if d, err := strconv.Atoi(strings.TrimSpace(dayStr)); err == nil {
		day = d
	}

	// Parse time.
	var hour, min, sec int
	n, _ := fmt.Sscanf(timeStr, "%d:%d:%d", &hour, &min, &sec)
	if n < 3 {
		return time.Time{}
	}

	month, ok := monthIndex(monthStr)
	if !ok {
		return time.Time{}
	}

	year := time.Now().Year()
	return time.Date(year, month, day, hour, min, sec, 0, time.Local)
}

func monthIndex(s string) (time.Month, bool) {
	months := map[string]time.Month{
		"Jan": 1, "Feb": 2, "Mar": 3, "Apr": 4, "May": 5, "Jun": 6,
		"Jul": 7, "Aug": 8, "Sep": 9, "Oct": 10, "Nov": 11, "Dec": 12,
	}
	m, ok := months[s[:3]]
	return m, ok
}

func severityName(n int) string {
	names := []string{"emerg", "alert", "crit", "err", "warn", "notice", "info", "debug"}
	if n >= 0 && n < len(names) {
		return names[n]
	}
	return ""
}
