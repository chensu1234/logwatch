// Copyright (c) 2026 Cielavenir
// SPDX-License-Identifier: GPL-3.0-only

// Package config handles loading, merging, and validating the logwatch YAML
// configuration. All configuration is optional; sensible defaults are applied
// where fields are omitted.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the root of the configuration tree.
type Config struct {
	Sources  []SourceConfig  `yaml:"sources"`
	Pipeline PipelineConfig  `yaml:"pipeline"`
	Outputs  []OutputConfig  `yaml:"outputs"`
}

// SourceConfig describes a single log source to watch.
type SourceConfig struct {
	// Path is the file path or glob pattern (e.g. /var/log/**/*.log).
	Path string `yaml:"path"`

	// Name is an optional human-readable label for this source.
	Name string `yaml:"name"`

	// ParseFormat forces a specific parse format; "auto" (default) attempts
	// auto-detection.
	ParseFormat string `yaml:"parse"`

	// MaxLineLength skips lines exceeding this byte count (default 65536).
	MaxLineLength int `yaml:"maxLineLength"`

	// Priority is an optional log level threshold — lines below this level
	// are dropped before the pipeline runs.
	Priority string `yaml:"priority"`
}

// PipelineConfig describes how each log line is processed.
type PipelineConfig struct {
	// ParseFormat is the parse mode: auto, json, syslog, nginx, raw.
	ParseFormat string `yaml:"parse"`

	// MinLevel discards events below this log level.
	MinLevel string `yaml:"minLevel"`

	// IncludeRegex forwards only lines matching this regex (empty = accept all).
	IncludeRegex string `yaml:"includeRegex"`

	// ExcludeRegex drops lines matching this regex (empty = drop none).
	ExcludeRegex string `yaml:"excludeRegex"`

	// Drop conditionally drops events based on a field comparison.
	Drop *DropRule `yaml:"drop"`

	// Transforms applies a sequence of field operations.
	Transforms []Transform `yaml:"transforms"`
}

// DropRule describes a field-based drop condition.
type DropRule struct {
	Field string `yaml:"field"` // dot-notation field path
	Value string `yaml:"value"` // drop if field equals this value
}

// Transform describes a single field transformation.
type Transform struct {
	Action string `yaml:"action"` // add | rename | remove | set
	Field  string `yaml:"field"`  // dot-notation field path
	To     string `yaml:"to"`     // target name (for rename)
	Value  string `yaml:"value"`  // static value or expression (for add/set)
}

// OutputConfig describes a single output destination.
type OutputConfig struct {
	// Type is the output driver: stdout, file, elasticsearch, kafka, webhook.
	Type string `yaml:"type"`

	// URL / path / broker list — meaning depends on Type.
	URL string `yaml:"url"`

	// IndexPattern is used by Elasticsearch outputs (strftime pattern).
	IndexPattern string `yaml:"index"`

	// Topic is used by Kafka outputs.
	Topic string `yaml:"topic"`

	// MaxSize / MaxBackups / Compress control file rotation for file outputs.
	MaxSize    int  `yaml:"maxSize"`
	MaxBackups int  `yaml:"maxBackups"`
	Compress   bool `yaml:"compress"`

	// BufferSize / FlushInterval control the output ring buffer.
	BufferSize    int           `yaml:"bufferSize"`
	FlushInterval time.Duration `yaml:"flushInterval"`
	// BatchSize is used by webhook outputs.
	BatchSize     int           `yaml:"batchSize"`

	// Headers for webhook outputs (key: value pairs).
	Headers map[string]string `yaml:"headers"`

	// Username / Password for authenticated outputs.
	Username string `yaml:"username"`
	Password string `yaml:"password"`

	// Compression codec for Kafka: none, gzip, snappy, lz4.
	Compression string `yaml:"compression"`

	// Color enables ANSI colour output for the stdout driver.
	Color bool `yaml:"color"`

	// RetryMax is the number of retries on transient errors.
	RetryMax int `yaml:"retryMax"`
}

// Load reads and parses the YAML configuration file at path. If path is empty
// or the file does not exist, a minimal default config is returned so that
// logwatch can still run with CLI flags alone.
func Load(path string) (*Config, error) {
	cfg := Default()

	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	// Expand environment variables in string fields.
	cfg.expandEnv()

	return cfg, nil
}

// Default returns a Config with sensible zero-value defaults.
func Default() *Config {
	return &Config{
		Pipeline: PipelineConfig{
			ParseFormat: "auto",
		},
		Outputs: []OutputConfig{
			{Type: "stdout"},
		},
	}
}

// Validate checks the configuration for common errors.
func (c *Config) Validate() error {
	// At least one source must be configured when not reading from stdin.
	if len(c.Sources) == 0 {
		// This is allowed — logwatch reads from stdin in that case.
	}
	seenNames := make(map[string]bool)
	for _, src := range c.Sources {
		if src.Path == "" {
			return fmt.Errorf("source missing required field: path")
		}
		if src.Name != "" {
			if seenNames[src.Name] {
				return fmt.Errorf("duplicate source name: %q", src.Name)
			}
			seenNames[src.Name] = true
		}
		if src.MaxLineLength <= 0 {
			src.MaxLineLength = 65536
		}
	}
	if len(c.Outputs) == 0 {
		return fmt.Errorf("at least one output must be configured")
	}
	validTypes := map[string]bool{
		"stdout": true, "file": true, "elasticsearch": true,
		"kafka": true, "webhook": true,
	}
	for i, out := range c.Outputs {
		if out.Type == "" {
			return fmt.Errorf("output[%d]: missing required field 'type'", i)
		}
		if !validTypes[out.Type] {
			return fmt.Errorf("output[%d]: unknown type %q (valid: stdout|file|elasticsearch|kafka|webhook)", i, out.Type)
		}
		if out.Type == "file" && out.URL == "" {
			return fmt.Errorf("output[%d]: file output requires 'url' path", i)
		}
	}
	return nil
}

// expandEnv replaces ${VAR} and $VAR tokens with environment variable values.
func (c *Config) expandEnv() {
	for i := range c.Sources {
		c.Sources[i].Path = expandEnvString(c.Sources[i].Path)
		c.Sources[i].Name = expandEnvString(c.Sources[i].Name)
	}
	for i := range c.Outputs {
		c.Outputs[i].URL = expandEnvString(c.Outputs[i].URL)
		c.Outputs[i].Topic = expandEnvString(c.Outputs[i].Topic)
		c.Outputs[i].Username = expandEnvString(c.Outputs[i].Username)
		c.Outputs[i].Password = expandEnvString(c.Outputs[i].Password)
		for k, v := range c.Outputs[i].Headers {
			c.Outputs[i].Headers[k] = expandEnvString(v)
		}
	}
}

func expandEnvString(s string) string {
	return os.Expand(s, func(k string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		// Return the original token if the env var is not set.
		return "${" + k + "}"
	})
}

// ParseMode returns a normalised parse format string.
func (p *PipelineConfig) ParseMode() string {
	switch strings.ToLower(p.ParseFormat) {
	case "json", "syslog", "nginx", "raw":
		return strings.ToLower(p.ParseFormat)
	default:
		return "auto"
	}
}
