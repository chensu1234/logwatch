// Copyright (c) 2026 Cielavenir
// SPDX-License-Identifier: GPL-3.0-only

package output

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"logwatch/internal/config"
	"logwatch/internal/parser"

	"github.com/IBM/sarama"
)

// Kafka delivers events to a Kafka topic using an async producer.
type Kafka struct {
	producer sarama.AsyncProducer
	topic    string
	log      Logger

	mu     sync.Mutex
	closed bool
	stopCh chan struct{}
}

// NewKafka creates an async Kafka producer.
func NewKafka(cfg config.OutputConfig, log Logger) (*Kafka, error) {
	brokers := strings.Split(cfg.URL, ",")
	if len(brokers) == 0 || brokers[0] == "" {
		return nil, fmt.Errorf("kafka output requires 'url' with broker address(es)")
	}
	topic := cfg.Topic
	if topic == "" {
		topic = "logs"
	}

	saramaConfig := sarama.NewConfig()
	saramaConfig.Producer.Return.Successes = false
	saramaConfig.Producer.Return.Errors = true
	saramaConfig.Producer.Retry.Max = cfg.RetryMax
	if saramaConfig.Producer.Retry.Max <= 0 {
		saramaConfig.Producer.Retry.Max = 3
	}

	switch strings.ToLower(cfg.Compression) {
	case "gzip":
		saramaConfig.Producer.Compression = sarama.CompressionGZIP
	case "snappy":
		saramaConfig.Producer.Compression = sarama.CompressionSnappy
	case "lz4":
		saramaConfig.Producer.Compression = sarama.CompressionLZ4
	case "zstd":
		saramaConfig.Producer.Compression = sarama.CompressionZSTD
	default:
		saramaConfig.Producer.Compression = sarama.CompressionNone
	}

	producer, err := sarama.NewAsyncProducer(brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("kafka producer: %w", err)
	}

	k := &Kafka{
		producer: producer,
		topic:    topic,
		log:      log,
		stopCh:   make(chan struct{}),
	}

	go k.drainErrors()
	return k, nil
}

func (k *Kafka) drainErrors() {
	for {
		select {
		case <-k.stopCh:
			return
		case err, ok := <-k.producer.Errors():
			if !ok {
				return
			}
			k.log.Error("kafka produce error: %s", err)
		}
	}
}

func (k *Kafka) Write(ev parser.Event) error {
	k.mu.Lock()
	if k.closed {
		k.mu.Unlock()
		return nil
	}
	k.mu.Unlock()

	data, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: k.topic,
		Key:   sarama.StringEncoder(ev.GetSource()),
		Value: sarama.ByteEncoder(data),
	}

	select {
	case k.producer.Input() <- msg:
	default:
		k.log.Warn("kafka input queue full — dropping message")
	}
	return nil
}

func (k *Kafka) Flush() error {
	k.producer.AsyncClose()
	return nil
}

func (k *Kafka) Close() error {
	k.mu.Lock()
	k.closed = true
	k.mu.Unlock()
	close(k.stopCh)
	return k.Flush()
}
