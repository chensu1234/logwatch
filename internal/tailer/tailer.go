// Copyright (c) 2026 Cielavenir
// SPDX-License-Identifier: GPL-3.0-only

// Package tailer provides file-watching functionality using fsnotify. It
// follows log files across rotation events (rename / truncate) and delivers
// raw line-by-line input to the pipeline.
package tailer

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"logwatch/internal/config"
	"logwatch/internal/pipeline"
	"logwatch/internal/util"

	"github.com/fsnotify/fsnotify"
)

// Tailer watches one or more files matching a glob pattern and forwards raw
// lines to the processing pipeline. It automatically handles file rotation.
type Tailer struct {
	cfg   *config.SourceConfig
	pipe  *pipeline.Pipeline
	log   *util.Logger
	watch *fsnotify.Watcher

	mu     sync.RWMutex
	closed bool
	stopCh chan struct{}
}

// New creates a Tailer for the given source config. The glob pattern in
// cfg.Path is expanded immediately to discover matching files.
func New(cfg *config.SourceConfig, pipe *pipeline.Pipeline, log *util.Logger) (*Tailer, error) {
	abs, err := filepath.Abs(cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("abs path: %w", err)
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("fsnotify: %w", err)
	}

	t := &Tailer{
		cfg:   cfg,
		pipe:  pipe,
		log:   log,
		watch: w,
		stopCh: make(chan struct{}),
	}

	// Watch the directory so we see new files created by rotation.
	dir := filepath.Dir(abs)
	if err := w.Add(dir); err != nil {
		w.Close()
		return nil, fmt.Errorf("watch dir %s: %w", dir, err)
	}

	return t, nil
}

// NewStdin creates a Tailer that reads from stdin instead of a file.
func NewStdin(pipe *pipeline.Pipeline, log *util.Logger) (*Tailer, error) {
	return &Tailer{
		cfg: &config.SourceConfig{
			Name:         "stdin",
			Path:         "/dev/stdin",
			MaxLineLength: 65536,
		},
		pipe:  pipe,
		log:   log,
		stopCh: make(chan struct{}),
	}, nil
}

// Start begins watching the configured file(s) and streams lines to the
// pipeline. It blocks until Stop is called or a fatal error occurs.
func (t *Tailer) Start() error {
	if t.cfg.Path == "/dev/stdin" {
		return t.watchStdin()
	}
	return t.watchLoop()
}

// watchLoop monitors the filesystem and processes file content.
func (t *Tailer) watchLoop() error {
	globPattern := t.cfg.Path
	files, err := filepath.Glob(globPattern)
	if err != nil {
		return fmt.Errorf("glob %s: %w", globPattern, err)
	}
	if len(files) == 0 {
		t.log.Warn("glob %q matched no files", globPattern)
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	for _, path := range files {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			t.tailFile(ctx, p)
		}(path)
	}

	go func() {
		for {
			select {
			case <-t.stopCh:
				cancel()
				return
			case <-ctx.Done():
				return
			case evt := <-t.watch.Events:
				t.handleEvent(ctx, evt, globPattern, &wg)
			case err := <-t.watch.Errors:
				if err != nil {
					t.log.Warn("fsnotify error: %s", err)
				}
			}
		}
	}()

	wg.Wait()
	return nil
}

// handleEvent processes fsnotify events.
func (t *Tailer) handleEvent(ctx context.Context, evt fsnotify.Event, globPattern string, wg *sync.WaitGroup) {
	matched, err := filepath.Match(globPattern, filepath.Base(evt.Name))
	if err != nil || !matched {
		return
	}

	switch {
	case evt.Has(fsnotify.Create):
		time.Sleep(200 * time.Millisecond)
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			t.tailFile(ctx, p)
		}(evt.Name)

	case evt.Has(fsnotify.Remove), evt.Has(fsnotify.Rename):
		t.log.Info("file removed: %s", evt.Name)
	}
}

// tailFile opens a file, seeks to the end, and reads new lines as they appear.
func (t *Tailer) tailFile(ctx context.Context, path string) {
	maxLen := t.cfg.MaxLineLength
	if maxLen <= 0 {
		maxLen = 65536
	}

	file, err := os.Open(path)
	if err != nil {
		t.log.Error("open %s: %s", path, err)
		return
	}
	defer file.Close()

	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		t.log.Error("seek %s: %s", path, err)
		return
	}

	reader := bufio.NewReaderSize(file, maxLen)
	sourceName := t.cfg.Name
	if sourceName == "" {
		sourceName = filepath.Base(path)
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			line = stripNewline(line)
			if len(line) > maxLen {
				t.log.Debug("line too long (%d > %d) from %s — skipping", len(line), maxLen, path)
				t.pipe.RecordDropped()
				continue
			}
			if len(line) == 0 {
				continue
			}
			t.pipe.Feed(line, sourceName)
		}
		if err != nil {
			if err == io.EOF {
				time.Sleep(250 * time.Millisecond)
				continue
			}
			return
		}
	}
}

// watchStdin reads from standard input until EOF or shutdown signal.
func (t *Tailer) watchStdin() error {
	maxLen := t.cfg.MaxLineLength
	if maxLen <= 0 {
		maxLen = 65536
	}

	reader := bufio.NewReaderSize(os.Stdin, maxLen)
	for {
		select {
		case <-t.stopCh:
			return nil
		default:
		}

		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			line = stripNewline(line)
			if len(line) == 0 {
				continue
			}
			t.pipe.Feed(line, "stdin")
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("stdin read: %w", err)
		}
	}
}

// Close releases all resources held by the Tailer.
func (t *Tailer) Close() {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	t.closed = true
	close(t.stopCh)
	t.mu.Unlock()

	if t.watch != nil {
		t.watch.Close()
	}
}

// stripNewline removes trailing \n and optional \r from a byte slice.
func stripNewline(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}

// parseName returns the source name from the config, falling back to the
// file basename. It collapses sequences of dots and underscores.
func parseName(path string) string {
	base := filepath.Base(path)
	return strings.Trim(base, "._")
}
