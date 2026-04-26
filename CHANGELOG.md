# Changelog

All notable changes to **logwatch** are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.0] – 2026-04-26

### Added
- **Kafka output driver** with gzip/snappy/lz4/zstd compression support
- **Ring buffer** with disk spill for output buffering — survives transient backend outages
- **`--stats-interval`** flag for periodic throughput monitoring
- **`--buffer-size`** and **`--flush-interval`** CLI overrides

### Improved
- Elasticsearch bulk indexing now uses exponential backoff retry on network errors
- Syslog parser now handles RFC 5424 structured data fields
- Auto-detection now falls back through JSON → syslog → raw in order

### Fixed
- File watcher goroutine leak on rapid file rotation
- Race condition between fsnotify events and reader seeking to EOF

---

## [0.3.0-rc.1] – 2026-04-10

### Added
- Webhook output driver with batch sending and retry
- `pipeline.transforms` with `add`, `rename`, `remove`, and `set` field actions
- `LOGWATCH_WEBHOOK_TOKEN` and `LOGWATCH_ES_PASSWORD` environment variable support

---

## [0.2.0] – 2026-03-15

### Added
- **Nginx/Apache combined log parser** — parses access logs with method, URI, status, referrer, and user-agent fields
- **Error log parser** for Nginx error log format
- **`--include-regex`** and **`--exclude-regex`** CLI flags
- **SIGUSR2 graceful reload** — hot-reloads config without restart
- Automatic `level` field extraction for Nginx access log (inferred from HTTP status code)

### Improved
- Auto-detection now inspects line prefix (`{` → JSON, `<` → syslog, `"` → Nginx)
- File rotation detection uses 200ms debounce to avoid reading partial files

### Fixed
- Stdin reader now properly handles binary / null bytes in log lines

---

## [0.1.0] – 2026-02-01

### Added
- Initial release
- Multi-file glob tailing with `fsnotify`
- JSON Lines parser with auto-detection
- Syslog parser (RFC 3164 + RFC 5424)
- Raw text passthrough mode
- Stdout, rotating file, and Elasticsearch outputs
- Structured internal logging with `terse`
- Full YAML configuration with environment variable expansion
- Graceful shutdown on SIGINT / SIGTERM
