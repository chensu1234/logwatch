// Copyright (c) 2026 Cielavenir
// SPDX-License-Identifier: GPL-3.0-only

package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"logwatch/internal/config"
	"logwatch/internal/parser"
)

// File writes JSON Lines to a rotating output file.
type File struct {
	path       string
	maxSize    int64
	maxBackups int
	compress   bool
	log        Logger

	mu      sync.Mutex
	closed  bool
	current *os.File
	size    int64
}

// NewFile creates a rotating file writer.
func NewFile(cfg config.OutputConfig, log Logger) (*File, error) {
	path := cfg.URL
	if path == "" {
		return nil, fmt.Errorf("file output requires 'url' path")
	}

	// Ensure the parent directory exists.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create dir %s: %w", dir, err)
	}

	maxSize := int64(cfg.MaxSize)
	if maxSize <= 0 {
		maxSize = 100 * 1 << 20 // 100 MB default
	}
	maxBackups := cfg.MaxBackups
	if maxBackups <= 0 {
		maxBackups = 5
	}

	f := &File{
		path:       path,
		maxSize:    maxSize,
		maxBackups: maxBackups,
		compress:   cfg.Compress,
		log:        log,
	}

	if err := f.openCurrent(); err != nil {
		return nil, err
	}

	return f, nil
}

// openCurrent opens the main output file, creating it if necessary.
func (f *File) openCurrent() error {
	file, err := os.OpenFile(f.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open %s: %w", f.path, err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("stat %s: %w", f.path, err)
	}
	f.current = file
	f.size = info.Size()
	return nil
}

// Write serialises ev as a JSON line and writes it to the output file.
// If the write would exceed maxSize, it triggers rotation first.
func (f *File) Write(ev parser.Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil
	}

	line, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	line = append(line, '\n')
	ln := int64(len(line))

	// Rotate if this write would exceed the size limit.
	if f.size+ln > f.maxSize {
		if err := f.rotateLocked(); err != nil {
			f.log.Error("file rotate: %v", err)
			return err
		}
	}

	n, err := f.current.Write(line)
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}
	f.size += int64(n)
	return nil
}

// rotateLocked closes the current file, archives it with a timestamp, and
// opens a new file. Caller must hold f.mu.
func (f *File) rotateLocked() error {
	if f.current != nil {
		f.current.Close()
	}

	// Rename current file to timestamped backup.
	backup := f.path + "." + time.Now().Format("2006-01-02T15-04-05")
	if err := os.Rename(f.path, backup); err != nil {
		f.log.Warn("rotate rename: %v", err)
	}

	// Compress the backup asynchronously (best-effort).
	if f.compress {
		go compressFile(backup)
	}

	// Purge old backups.
	f.purgeBackups()

	// Open a fresh file.
	if err := f.openCurrent(); err != nil {
		return err
	}

	f.log.Info("rotated log file → %s", backup)
	return nil
}

// purgeBackups deletes the oldest backup files beyond maxBackups.
func (f *File) purgeBackups() {
	dir := filepath.Dir(f.path)
	globPat := filepath.Join(dir, filepath.Base(f.path)+".*")
	matches, err := filepath.Glob(globPat)
	if err != nil {
		return
	}
	if len(matches) <= f.maxBackups {
		return
	}
	// Sort by modification time (oldest first).
	sort.Slice(matches, func(i, j int) bool {
		ti, _ := os.Stat(matches[i]); tj, _ := os.Stat(matches[j])
		if ti == nil || tj == nil {
			return false
		}
		return ti.ModTime().Before(tj.ModTime())
	})
	// Delete everything beyond maxBackups.
	for _, m := range matches[:len(matches)-f.maxBackups] {
		os.Remove(m)
	}
}

// compressFile gzip-compresses a file and removes the original.
// Best-effort: errors are silently ignored.
func compressFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	// Write to .gz (downstream tools handle it); remove original.
	gzPath := path + ".gz"
	if err := os.WriteFile(gzPath, data, 0644); err != nil {
		return
	}
	os.Remove(path)
}

// Flush syncs data to disk.
func (f *File) Flush() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.current == nil {
		return nil
	}
	return f.current.Sync()
}

// Close closes the file and frees resources.
func (f *File) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	if f.current == nil {
		return nil
	}
	err := f.current.Close()
	f.current = nil
	return err
}
