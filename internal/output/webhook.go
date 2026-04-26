// Copyright (c) 2026 Cielavenir
// SPDX-License-Identifier: GPL-3.0-only

package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"logwatch/internal/config"
	"logwatch/internal/parser"
)

// Webhook delivers events to an HTTP endpoint as JSON.
type Webhook struct {
	url       string
	headers   map[string]string
	batchSize int
	retryMax  int
	client    *http.Client
	log       Logger

	buffer []parser.Event
	mu     sync.Mutex
	closed bool
	stopCh chan struct{}
}

// NewWebhook creates an HTTP webhook sender with batching.
func NewWebhook(cfg config.OutputConfig, log Logger) (*Webhook, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("webhook output requires 'url'")
	}

	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 50
	}
	retryMax := cfg.RetryMax
	if retryMax <= 0 {
		retryMax = 3
	}

	return &Webhook{
		url:       cfg.URL,
		headers:   cfg.Headers,
		batchSize: batchSize,
		retryMax:  retryMax,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		log:     log,
		buffer:  make([]parser.Event, 0, batchSize),
		stopCh:  make(chan struct{}),
	}, nil
}

func (w *Webhook) Write(ev parser.Event) error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.buffer = append(w.buffer, ev)
	shouldFlush := len(w.buffer) >= w.batchSize
	w.mu.Unlock()

	if shouldFlush {
		return w.flush()
	}
	return nil
}

func (w *Webhook) flush() error {
	w.mu.Lock()
	if len(w.buffer) == 0 {
		w.mu.Unlock()
		return nil
	}
	events := w.buffer
	w.buffer = make([]parser.Event, 0, w.batchSize)
	w.mu.Unlock()

	return w.sendWithRetry(events)
}

func (w *Webhook) sendWithRetry(events []parser.Event) error {
	envelope := map[string]interface{}{
		"count":    len(events),
		"events":   events,
		"sent_at":  time.Now().UTC().Format(time.RFC3339),
	}

	payload, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= w.retryMax; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			time.Sleep(backoff)
		}

		err := w.post(payload)
		if err == nil {
			return nil
		}
		lastErr = err
		w.log.Warn("webhook attempt %d/%d failed: %s", attempt+1, w.retryMax+1, err)
	}
	return fmt.Errorf("webhook send (after %d retries): %w", w.retryMax, lastErr)
}

func (w *Webhook) post(payload []byte) error {
	req, err := http.NewRequest(http.MethodPost, w.url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.headers {
		req.Header.Set(k, v)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (w *Webhook) Flush() error { return w.flush() }

func (w *Webhook) Close() error {
	w.mu.Lock()
	w.closed = true
	w.mu.Unlock()
	close(w.stopCh)
	return w.Flush()
}
