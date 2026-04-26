# Logwatch

**A powerful, real-time log file tailing, parsing, and forwarding tool for DevOps and SREs.**

[![Build Status](https://img.shields.io/github/actions/workflow/status/cielavenir/logwatch/ci.yml?branch=main&logo=github)](https://github.com/cielavenir/logwatch/actions)
[![Go Version](https://img.shields.io/github/go-mod/go-version/cielavenir/logwatch?logo=go)](https://golang.org/)
[![License](https://img.shields.io/github/license/cielavenir/logwatch?logo=gpl-3.0)](LICENSE)
[![Releases](https://img.shields.io/github/v/release/cielavenir/logwatch?logo=tag&include_prereleases)](https://github.com/cielavenir/logwatch/releases)
[![Stars](https://img.shields.io/github/stars/cielavenir/logwatch?logo=star)](https://github.com/cielavenir/logwatch/stargazers)

---

`logwatch` watches log files in real time, parses structured or unstructured entries, filters and transforms them through a configurable pipeline, and streams the results to one or more destinations — stdout, files, Elasticsearch, Kafka, or a webhook endpoint.

If you need to aggregate logs from multiple sources, build a lightweight log pipeline, monitor log patterns, or forward logs to a SIEM, `logwatch` gets out of your way and just works.

---

## ✨ Features

- **Multi-source tailing** — Watch multiple files simultaneously with glob pattern support (`/var/log/**/*.log`)
- **Structured parsing** — Auto-detect and parse JSON Lines, Syslog (RFC 3164 / RFC 5424), Apache/Nginx combined log, and raw text formats
- **Powerful filtering** — Include/exclude by regex, field comparison, log level threshold, or arbitrary expression
- **Field transformation** — Add, rename, remove, or reformat fields using a dot-notation pipeline
- **Multiple outputs** — Fan out to stdout, rotating file, Elasticsearch, Kafka, or HTTP webhook
- **Buffered writes** — In-memory ring buffer with optional disk spill to survive transient output outages
- **Graceful reload** — Config changes picked up without restart (SIGUSR2)
- **Zero-dependency binary** — Single static Go binary, no runtime needed on the target host
- **Production-ready** — Structured JSON logging, signal handling, graceful shutdown, exit codes

---

## 🏃 Quick Start

### Installation

**Binary (recommended)**

```bash
# macOS (Apple Silicon)
curl -sSL https://github.com/cielavenir/logwatch/releases/latest/download/logwatch-darwin-arm64.tar.gz | tar -xz
sudo mv logwatch /usr/local/bin/

# Linux (x86_64)
curl -sSL https://github.com/cielavenir/logwatch/releases/latest/download/logwatch-linux-amd64.tar.gz | tar -xz
sudo mv logwatch /usr/local/bin/
```

**From source**

```bash
git clone https://github.com/cielavenir/logwatch.git
cd logwatch
make build
sudo make install
```

**Docker**

```bash
docker pull ghcr.io/cielavenir/logwatch:latest
docker run --rm \
  -v /var/log/myapp.log:/var/log/myapp.log:ro \
  ghcr.io/cielavenir/logwatch:latest \
  --config /etc/logwatch/config.yaml
```

### First run

```bash
# Create a minimal config
cat > logwatch.yaml << 'EOF'
sources:
  - path: /var/log/myapp.log
    name: myapp

pipeline:
  parse: json

outputs:
  - type: stdout
EOF

# Run
logwatch --config logwatch.yaml
```

### Try it without a config

```bash
# Pipe logs directly — reads from stdin, outputs parsed JSON to stdout
tail -f /var/log/syslog | logwatch --from-stdin --parse syslog
```

---

## ⚙️ Configuration

Configuration is in YAML format. All options can be overridden via CLI flags.

### Minimal config

```yaml
# Watch a single file, print parsed JSON to stdout
sources:
  - path: /var/log/myapp.log

outputs:
  - type: stdout
```

### Full config

```yaml
# ── Sources ──────────────────────────────────────────────
sources:
  - path: /var/log/myapp/*.log
    name: myapp
    priority: info          # only forward if log level >= info
    maxLineLength: 65536    # truncate lines longer than this

  - path: /var/log/nginx/access.log
    name: nginx
    parse: nginx           # explicit format parser

  - path: /var/log/syslog
    name: syslog
    parse: syslog

# ── Pipeline ─────────────────────────────────────────────
pipeline:
  # Parse format: auto | json | syslog | nginx | raw
  parse: json

  # Drop logs below this level (debug | info | warn | error | fatal)
  minLevel: info

  # Include only lines matching this regex
  includeRegex: "^\\["

  # Exclude lines matching this regex
  excludeRegex: "healthcheck"

  # Drop events where field matches value
  drop:
    field: level
    value: debug

  # Transform fields
  transforms:
    - action: add           # add | rename | remove | set
      field: service        # target field (dot-notation supported)
      value: myapp          # static value or expression

    - action: rename
      field: ts
      to: timestamp

    - action: remove
      field: .env           # . prefix = regex match

# ── Outputs ──────────────────────────────────────────────
outputs:
  # Pretty-print to terminal
  - type: stdout
    color: true

  # Write to rotating files
  - type: file
    path: /var/log/collected/app.log
    maxSize: 100MB
    maxBackups: 5
    compress: true

  # Forward to Elasticsearch
  - type: elasticsearch
    url: http://elasticsearch:9200
    index: logs-%Y%m%d
    username: elastic
    password: changeme
    bufferSize: 1000
    flushInterval: 5s

  # Stream to Kafka
  - type: kafka
    brokers:
      - kafka:9092
    topic: logs
    compression: gzip

  # POST to a webhook
  - type: webhook
    url: https://logs.example.com/ingest
    headers:
      Authorization: "Bearer $WEBHOOK_TOKEN"
    batchSize: 50
    retryMax: 3
```

### Environment variables

| Variable | Description |
|---|---|
| `LOGWATCH_CONFIG` | Path to config file (default: `./logwatch.yaml`) |
| `LOGWATCH_WEBHOOK_TOKEN` | Token used in webhook `Authorization` header |
| `LOGWATCH_ES_PASSWORD` | Elasticsearch password |
| `LOGWATCH_LOG_LEVEL` | Override log level (`debug`, `info`, `warn`, `error`) |

---

## 📋 Command Line Options

| Flag | Short | Default | Description |
|---|---|---|---|
| `--config` | `-c` | `logwatch.yaml` | Path to YAML config file |
| `--from-stdin` | | `false` | Read input from stdin instead of tailing files |
| `--parse` | `-p` | `auto` | Parse format: `auto`, `json`, `syslog`, `nginx`, `raw` |
| `--include-regex` | | | Only process lines matching this regex |
| `--exclude-regex` | | | Skip lines matching this regex |
| `--min-level` | | | Minimum log level to forward |
| `--output` | `-o` | `stdout` | Output type: `stdout`, `file`, `elasticsearch`, `kafka`, `webhook` |
| `--output-url` | | | URL / path / broker list for the output |
| `--index` | | `logs-%Y%m%d` | Elasticsearch index pattern |
| `--topic` | | `logs` | Kafka topic name |
| `--buffer-size` | | `1000` | Number of events to buffer in memory |
| `--flush-interval` | | `5s` | Force flush buffer after this duration |
| `--max-line-length` | | `65536` | Skip lines longer than this |
| `--log-level` | | `info` | logwatch's own log level |
| `--stats-interval` | | `30s` | Print stats every N seconds (`0` to disable) |
| `--version` | `-v` | | Print version and exit |
| `--help` | `-h` | | Show help |

---

## 📁 Project Structure

```
logwatch/
├── cmd/
│   └── logwatch/
│       └── main.go          # Entry point, CLI flag parsing
├── internal/
│   ├── config/
│   │   └── config.go        # Config loading and validation
│   ├── tailer/
│   │   └── tailer.go        # File tailing with fsnotify
│   ├── parser/
│   │   ├── parser.go        # Auto-detection + router
│   │   ├── json.go          # JSON Lines parser
│   │   ├── syslog.go        # Syslog (RFC 3164/5424) parser
│   │   └── nginx.go         # Apache/Nginx log parser
│   ├── filter/
│   │   └── filter.go        # Include/exclude/level filtering
│   ├── output/
│   │   ├── stdout.go        # Stdout writer
│   │   ├── file.go          # Rotating file writer
│   │   ├── elasticsearch.go # Elasticsearch bulk writer
│   │   ├── kafka.go         # Kafka async producer
│   │   └── webhook.go       # HTTP webhook sender
│   ├── pipeline/
│   │   └── pipeline.go      # Event processing pipeline
│   └── util/
│       ├── ring.go          # Lock-free ring buffer
│       └── level.go         # Log level comparison
├── config/
│   └── examples/
│       └── full.yaml        # Full configuration example
├── man/
│   └── logwatch.1.md        # Man page source
├── scripts/
│   └── install.sh           # Quick install script
├── Makefile
├── LICENSE
└── README.md
```

---

## 📝 CHANGELOG

All notable changes are documented in [CHANGELOG.md](CHANGELOG.md).

### v0.3.0 (2026-04-26)

- **Added** Kafka output driver with gzip compression
- **Added** Ring buffer with disk spill for output buffering
- **Added** `--stats-interval` flag for throughput monitoring
- **Improved** Elasticsearch bulk indexing with exponential backoff retry
- **Improved** Syslog parser now handles RFC 5424 with structured data
- **Fixed** File watcher memory leak on rapid rotation

### v0.2.0 (2026-03-15)

- **Added** Nginx/Apache combined log parser
- **Added** Pipeline field transforms (add, rename, remove, set)
- **Added** SIGUSR2 graceful config reload
- **Improved** Auto-detection of JSON vs plain text

### v0.1.0 (2026-02-01)

- Initial release
- File tailing with fsnotify
- JSON, Syslog, raw text parsers
- Stdout, file, Elasticsearch, webhook outputs

---

## 📄 License

This project is licensed under the **GNU General Public License v3.0** — see the [LICENSE](LICENSE) file for details.

```
Copyright (C) 2026  Cielavenir

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.
```
