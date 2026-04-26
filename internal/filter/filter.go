// Copyright (c) 2026 Cielavenir
// SPDX-License-Identifier: GPL-3.0-only

// Package filter provides log-event filtering primitives: regex include/exclude
// rules, log-level threshold comparisons, and field-value drop rules.
package filter

import (
	"regexp"
	"strings"

	"logwatch/internal/parser"
)

// Level ordering for severity comparison. Higher numbers = more severe.
var levelOrder = map[string]int{
	"debug": 0, "trace": 0,
	"info": 1, "information": 1, "informational": 1,
	"notice": 2,
	"warn": 3, "warning": 3,
	"error": 4, "err": 4,
	"critical": 5, "crit": 5, "fatal": 5, "severe": 5,
	"panic": 6, "emergency": 6, "emerg": 6, "alert": 6,
}

// EventFilter applies include/exclude/minLevel/drop rules to a parsed Event.
type EventFilter struct {
	includeRe *regexp.Regexp
	excludeRe *regexp.Regexp
	minLevel   int
	dropField  string
	dropValue  string
}

// New constructs an EventFilter from the filter configuration.
func New(includeRegex, excludeRegex, minLevel, dropField, dropValue string) (*EventFilter, error) {
	f := &EventFilter{
		minLevel:  levelOrder[strings.ToLower(minLevel)],
		dropField: dropField,
		dropValue: dropValue,
	}
	if includeRegex != "" {
		re, err := regexp.Compile(includeRegex)
		if err != nil {
			return nil, &FilterError{Field: "includeRegex", Err: err}
		}
		f.includeRe = re
	}
	if excludeRegex != "" {
		re, err := regexp.Compile(excludeRegex)
		if err != nil {
			return nil, &FilterError{Field: "excludeRegex", Err: err}
		}
		f.excludeRe = re
	}
	// Default: if no minLevel specified, accept everything.
	if _, ok := levelOrder[strings.ToLower(minLevel)]; !ok && minLevel != "" {
		f.minLevel = 0
	}
	return f, nil
}

// Allow returns true if the event passes all filter rules. It also
// checks the raw message against include/exclude regex patterns.
func (f *EventFilter) Allow(ev parser.Event) bool {
	// Include regex: if set and raw doesn't match, drop.
	if f.includeRe != nil {
		raw := ev[parser.KeyRaw]
		var rawStr string
		if raw != nil {
			rawStr, _ = raw.(string)
		}
		if !f.includeRe.MatchString(rawStr) {
			return false
		}
	}

	// Exclude regex: if raw matches, drop immediately.
	if f.excludeRe != nil {
		raw := ev[parser.KeyRaw]
		var rawStr string
		if raw != nil {
			rawStr, _ = raw.(string)
		}
		if f.excludeRe.MatchString(rawStr) {
			return false
		}
	}

	// Log level threshold.
	if minLvl := f.minLevel; minLvl > 0 {
		levelStr, _ := ev[parser.KeyLevel].(string)
		if lvl := levelOrder[strings.ToLower(levelStr)]; lvl < minLvl {
			return false
		}
	}

	// Field drop rule.
	if f.dropField != "" {
		if v, ok := ev[f.dropField].(string); ok && v == f.dropValue {
			return false
		}
	}

	return true
}

// FilterError reports a configuration error in a filter rule.
type FilterError struct {
	Field string
	Err   error
}

func (e *FilterError) Error() string {
	return "filter." + e.Field + ": " + e.Err.Error()
}
