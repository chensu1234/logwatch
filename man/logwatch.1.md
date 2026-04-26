% LOGWATCH(1) logwatch 0.3.0
% Cielavenir
% April 2026

# NAME

logwatch — real-time log file tailing, parsing, and forwarding

# SYNOPSIS

**logwatch** [_options_]

**logwatch** `--from-stdin` [_options_]

# DESCRIPTION

**logwatch** monitors one or more log files in real time, parses each line
according to a configurable format, optionally filters and transforms the
resulting fields, and delivers the events to one or more output destinations.

When no `--config` file is found, **logwatch** uses safe defaults so that it can
operate solely from CLI flags.

# OPTIONS

**-c**, **`--config`** _path_
:   Path to the YAML configuration file. (default: `logwatch.yaml`)

**`--from-stdin`**
:   Read log lines from standard input instead of tailing files.

**-p**, **`--parse`** _mode_
:   Force a parse format for all sources. Valid values: `auto`, `json`,
    `syslog`, `nginx`, `raw`. (default: `auto`)

**`--include-regex`** _regex_
:   Forward only lines matching this regular expression.

**`--exclude-regex`** _regex_
:   Skip lines matching this regular expression.

**`--min-level`** _level_
:   Minimum log severity to forward. Valid: `debug`, `info`, `warn`,
    `error`, `fatal`. (default: forward all levels)

**-o**, **`--output`** _type_
:   Output driver. Valid: `stdout`, `file`, `elasticsearch`, `kafka`,
    `webhook`. (default: `stdout`)

**`--output-url`** _url_
:   URL, file path, or broker list for the output driver.

**`--index`** _pattern_
:   Elasticsearch index name pattern (strftime format). (default: `logs-%Y%m%d`)

**`--topic`** _name_
:   Kafka topic name. (default: `logs`)

**`--buffer-size`** _n_
:   Number of events held in the output buffer before flushing.
    (default: `1000`)

**`--flush-interval`** _duration_
:   Force flush the output buffer after this interval. (default: `5s`)

**`--max-line-length`** _n_
:   Skip lines exceeding this byte length. (default: `65536`)

**`--log-level`** _level_
:   logwatch's own logging level. Valid: `debug`, `info`, `warn`, `error`.
    (default: `info`)

**`--stats-interval`** _duration_
:   Print throughput statistics every N seconds. Set to `0` to disable.
    (default: `30s`)

**-v**, **`--version`**
:   Print version and exit.

**-h**, **`--help`**
:   Show this help text.

# CONFIGURATION

Configuration is in YAML format. See **logwatch.yaml**(5) or the examples in
`/usr/share/logwatch/examples/` for the full reference.

# SIGNALS

**SIGINT**, **SIGTERM**
:   Graceful shutdown — flushes all output buffers before exiting.

**SIGUSR2**
:   Hot-reload configuration from the config file without restarting.

# ENVIRONMENT

`LOGWATCH_CONFIG`
:   Default config file path.

`LOGWATCH_WEBHOOK_TOKEN`
:   Token inserted into the `Authorization` header for webhook outputs.

`LOGWATCH_ES_PASSWORD`
:   Elasticsearch password.

`LOGWATCH_LOG_LEVEL`
:   Override log level (`debug`, `info`, `warn`, `error`).

# EXIT CODES

0
:   Normal shutdown.

1
:   Configuration error or fatal startup error.

2
:   Runtime error during tailing or output.

# FILES

`/etc/logwatch/config.yaml`
:   System-wide configuration file.

`~/.config/logwatch/config.yaml`
:   Per-user configuration file.

# SEE ALSO

**logwatch.yaml**(5), the full documentation at
<https://github.com/cielavenir/logwatch>
