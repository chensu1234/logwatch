// Copyright (c) 2026 Cielavenir
// SPDX-License-Identifier: GPL-3.0-only

package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"logwatch/internal/config"
	"logwatch/internal/parser"
)

// Elasticsearch writes events to an Elasticsearch cluster using the bulk API.
type Elasticsearch struct {
	url        string
	indexPat   string
	username   string
	password   string
	bufferSize int
	flushInt   time.Duration
	client     *http.Client

	log      Logger
	buffer   []parser.Event
	mu       sync.Mutex
	closed   bool
	stopCh   chan struct{}
}

// NewElasticsearch creates an Elasticsearch bulk writer with buffering.
func NewElasticsearch(cfg config.OutputConfig, log Logger) (*Elasticsearch, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("elasticsearch output requires 'url'")
	}

	flushInt := cfg.FlushInterval
	if flushInt <= 0 {
		flushInt = 5 * time.Second
	}
	bufSize := cfg.BufferSize
	if bufSize <= 0 {
		bufSize = 1000
	}

	es := &Elasticsearch{
		url:        strings.TrimSuffix(cfg.URL, "/"),
		indexPat:   cfg.IndexPattern,
		username:   cfg.Username,
		password:   cfg.Password,
		bufferSize: bufSize,
		flushInt:   flushInt,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		log:     log,
		buffer:  make([]parser.Event, 0, bufSize),
		stopCh:  make(chan struct{}),
	}

	go es.periodicFlush()
	return es, nil
}

func (es *Elasticsearch) periodicFlush() {
	ticker := time.NewTicker(es.flushInt)
	defer ticker.Stop()
	for {
		select {
		case <-es.stopCh:
			return
		case <-ticker.C:
			es.Flush()
		}
	}
}

func (es *Elasticsearch) Write(ev parser.Event) error {
	es.mu.Lock()
	if es.closed {
		es.mu.Unlock()
		return nil
	}
	es.buffer = append(es.buffer, ev)
	shouldFlush := len(es.buffer) >= es.bufferSize
	es.mu.Unlock()

	if shouldFlush {
		select {
		case es.stopCh <- struct{}{}:
		default:
		}
	}
	return nil
}

func (es *Elasticsearch) Flush() error {
	es.mu.Lock()
	if len(es.buffer) == 0 {
		es.mu.Unlock()
		return nil
	}
	events := es.buffer
	es.buffer = make([]parser.Event, 0, es.bufferSize)
	es.mu.Unlock()

	var buf bytes.Buffer
	for _, ev := range events {
		indexName := es.currentIndex()
		meta := map[string]interface{}{
			"index": map[string]string{"_index": indexName},
		}
		metaLine, _ := json.Marshal(meta)
		buf.Write(metaLine)
		buf.WriteByte('\n')

		doc := ev.Clone()
		if _, ok := doc["@timestamp"]; !ok {
			if ts, ok := doc["timestamp"].(time.Time); ok {
				doc["@timestamp"] = ts.Format(time.RFC3339Nano)
			}
		}
		docLine, _ := json.Marshal(doc)
		buf.Write(docLine)
		buf.WriteByte('\n')
	}

	req, err := http.NewRequest(http.MethodPost, es.url+"/_bulk", &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	if es.username != "" {
		req.SetBasicAuth(es.username, es.password)
	}

	resp, err := es.client.Do(req)
	if err != nil {
		es.mu.Lock()
		es.buffer = append(events, es.buffer...)
		es.mu.Unlock()
		return fmt.Errorf("elasticsearch bulk: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("elasticsearch bulk: HTTP %d — %s", resp.StatusCode, string(body))
	}
	return nil
}

func (es *Elasticsearch) currentIndex() string {
	if es.indexPat == "" {
		return "logs"
	}
	return time.Now().UTC().Format(es.indexPat)
}

func (es *Elasticsearch) Close() error {
	es.mu.Lock()
	es.closed = true
	es.mu.Unlock()
	close(es.stopCh)
	return es.Flush()
}
