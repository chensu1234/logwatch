# Makefile for logwatch
# Targets:
#   build       – compile for the current OS/arch
#   build/all   – cross-compile for all targets
#   test        – run unit tests
#   test/cover  – run tests with coverage
#   install     – install binary to $(PREFIX)/bin
#   clean       – remove build artifacts
#   lint        – run golangci-lint
#   fmt         – format code
#   tidy        – tidy go.mod

.PHONY: build build/all test test/cover install clean lint fmt tidy

# Binary name and version
BIN      := logwatch
VERSION  := 0.3.0
BUILD    := $(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
LDFLAGS  := -X main.version=$(VERSION) -X main.build=$(BUILD)
PREFIX   ?= /usr/local
GO       := go
GOFLAGS  := -ldflags "$(LDFLAGS)"

# Cross-compilation targets.
TARGETS  := \
	darwin-arm64   GOOS=darwin  GOARCH=arm64   CGO_ENABLED=0 \
	darwin-amd64   GOOS=darwin  GOARCH=amd64   CGO_ENABLED=0 \
	linux-arm64    GOOS=linux   GOARCH=arm64   CGO_ENABLED=0 \
	linux-amd64    GOOS=linux   GOARCH=amd64   CGO_ENABLED=0 \
	linux-x86      GOOS=linux   GOARCH=386     CGO_ENABLED=0 \
	windows-amd64  GOOS=windows GOARCH=amd64   CGO_ENABLED=0

# ── Build ──────────────────────────────────────────────────────────────────────

build:
	$(GO) build $(GOFLAGS) -o bin/$(BIN) ./cmd/logwatch

build/all: $(addprefix build-,,$(TARGETS))

build-%:
	$(GO) build $(GOFLAGS) -o bin/$(BIN)-$* ./cmd/logwatch

# ── Test ──────────────────────────────────────────────────────────────────────

test:
	$(GO) test ./...

test/cover:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# ── Install ───────────────────────────────────────────────────────────────────

install: build
	install -Dm755 bin/$(BIN) $(PREFIX)/bin/$(BIN)

# ── Clean ─────────────────────────────────────────────────────────────────────

clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# ── Lint & Format ──────────────────────────────────────────────────────────────

fmt:
	$(GO) fmt ./...

tidy:
	$(GO) mod tidy

lint:
	golangci-lint run ./... || true

# ── Docker ─────────────────────────────────────────────────────────────────────

docker/build:
	docker build -t cielavenir/logwatch:$(VERSION) -t cielavenir/logwatch:latest .

# ── Release ───────────────────────────────────────────────────────────────────

release: build/all
	mkdir -p dist/
	@for target in $(TARGETS); do \
		name=$${target%% *}; \
		os=$${target#*GOOS=}; os=$${os%% *}; \
		arch=$${target#*GOARCH=}; arch=$${arch%% *}; \
		ext=$$([ "$$os" = "windows" ] && echo ".exe" || echo ""); \
		tarball=logwatch-$$os-$$arch.tar.gz; \
		if [ "$$os" = "windows" ]; then \
			zip -j dist/$$tarball bin/$(BIN)-$$name.exe 2>/dev/null || true; \
		else \
			tar -czf dist/$$tarball -C bin/ $(BIN)-$$name; \
		fi \
	done
