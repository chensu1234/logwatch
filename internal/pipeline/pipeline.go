// Copyright (c) 2026 Cielavenir
// SPDX-License-Identifier: GPL-3.0-only

// Package pipeline orchestrates the processing flow: raw line → parser → filter
// → transforms → outputs. It is safe for concurrent use by multiple tailers.
package pipeline

import (
	"encoding/json"
	"runtime"
	"strings"
	"sync/atomic"

	"logwatch/internal/config"
	"logwatch/internal/filter"
	"logwatch/internal/output"
	"logwatch/internal/parser"
	"logwatch/internal/util"
)

// Pipeline processes raw log lines through parsing, filtering, and transforms
// before forwarding the result to all configured outputs.
type Pipeline struct {
	cfg     *config.Config
	outputs []output.Writer
	log     *util.Logger

	router     *parser.Router
	eventFilter *filter.EventFilter

	processed uint64
	dropped   uint64
	errored   uint64
}

// Stats holds throughput statistics for the pipeline.
type Stats struct {
	Processed uint64
	Dropped   uint64
	Errored   uint64
	RSSMB     int
}

// New creates a new Pipeline with the given configuration and output writers.
func New(cfg *config.Config, writers []output.Writer, log *util.Logger) *Pipeline {
	f, _ := filter.New(
		cfg.Pipeline.IncludeRegex,
		cfg.Pipeline.ExcludeRegex,
		cfg.Pipeline.MinLevel,
		"", "",
	)

	route := cfg.Pipeline.ParseMode()
	if route == "" {
		route = "auto"
	}

	return &Pipeline{
		cfg:        cfg,
		outputs:    writers,
		log:        log,
		router:     parser.NewRouter(route),
		eventFilter: f,
	}
}

// Feed is the main entry point — called by tailers with each raw line and its
// source name.
func (p *Pipeline) Feed(raw []byte, sourceName string) {
	if len(raw) == 0 {
		return
	}

	atomic.AddUint64(&p.processed, 1)

	ev := p.router.Parse(raw, sourceName)
	if ev == nil {
		ev = parser.Event{parser.KeyRaw: string(raw), parser.KeySource: sourceName}
	}

	if p.eventFilter != nil && !p.eventFilter.Allow(ev) {
		atomic.AddUint64(&p.dropped, 1)
		return
	}

	p.applyTransforms(ev)

	for _, out := range p.outputs {
		if err := out.Write(ev); err != nil {
			atomic.AddUint64(&p.errored, 1)
			p.log.Error("output write: %s", err)
		}
	}
}

// applyTransforms executes the configured pipeline transforms on ev.
func (p *Pipeline) applyTransforms(ev parser.Event) {
	if len(p.cfg.Pipeline.Transforms) == 0 {
		return
	}
	for _, t := range p.cfg.Pipeline.Transforms {
		switch t.Action {
		case "add", "set":
			ev[t.Field] = t.Value
		case "rename":
			if v, ok := ev[t.Field]; ok {
				delete(ev, t.Field)
				ev[t.To] = v
			}
		case "remove":
			delete(ev, t.Field)
		}
	}
}

// RecordDropped increments the dropped counter for lines that were skipped
// before reaching the pipeline (e.g. too long, binary data).
func (p *Pipeline) RecordDropped() {
	atomic.AddUint64(&p.dropped, 1)
}

// Stats returns a snapshot of current pipeline statistics.
func (p *Pipeline) Stats() Stats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return Stats{
		Processed: atomic.LoadUint64(&p.processed),
		Dropped:   atomic.LoadUint64(&p.dropped),
		Errored:   atomic.LoadUint64(&p.errored),
		RSSMB:     int(m.Alloc / 1024 / 1024),
	}
}

// Reload rebuilds the pipeline with a new configuration.
func (p *Pipeline) Reload(newCfg *config.Config) {
	f, _ := filter.New(
		newCfg.Pipeline.IncludeRegex,
		newCfg.Pipeline.ExcludeRegex,
		newCfg.Pipeline.MinLevel,
		"", "",
	)
	p.cfg = newCfg
	p.eventFilter = f
	p.router = parser.NewRouter(newCfg.Pipeline.ParseMode())

	for _, out := range p.outputs {
		out.Flush()
	}
}

// Close flushes all outputs and waits for delivery.
func (p *Pipeline) Close() {
	for _, out := range p.outputs {
		if err := out.Flush(); err != nil {
			p.log.Warn("output flush: %s", err)
		}
		if err := out.Close(); err != nil {
			p.log.Warn("output close: %s", err)
		}
	}
}

// MarshalJSON serialises an Event to JSON bytes.
func MarshalJSON(ev parser.Event) ([]byte, error) {
	return json.Marshal(ev)
}

// getField extracts a dot-notation field from an event.
func getField(ev parser.Event, path string) interface{} {
	parts := strings.SplitN(path, ".", 2)
	v, ok := ev[parts[0]]
	if !ok {
		return nil
	}
	if len(parts) == 1 {
		return v
	}
	if m, ok := v.(map[string]interface{}); ok {
		return getField(parser.Event(m), parts[1])
	}
	return nil
}

