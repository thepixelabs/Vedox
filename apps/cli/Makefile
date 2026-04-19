# Makefile for the Vedox CLI (apps/cli)
#
# Targets:
#   make build          — compile vedox binary to ./bin/vedox (stub transcriber)
#   make build-whisper  — compile vedox with real Whisper.cpp transcriber
#                         (requires WHISPER_ROOT env var and libwhisper.a built;
#                          see docs/how-to/build-whisper.md for the full guide)
#   make test     — run all tests with race detector
#   make vet      — run go vet
#   make lint     — run staticcheck (install: go install honnef.co/go/tools/cmd/staticcheck@latest)
#   make clean    — remove build artifacts

MODULE  := github.com/vedox/vedox
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -ldflags "\
  -X $(MODULE)/cmd.version=$(VERSION) \
  -X $(MODULE)/cmd.commit=$(COMMIT) \
  -X $(MODULE)/cmd.buildDate=$(DATE)"

BIN := bin/vedox

# go.sum must be populated before building. Run `make tidy` first on a fresh
# checkout: go mod tidy will fetch and hash all dependencies declared in go.mod.

# ---------------------------------------------------------------------------
# Whisper.cpp integration (optional — requires CGO + libwhisper.a)
# ---------------------------------------------------------------------------
#
# Usage:
#   export WHISPER_ROOT=/path/to/whisper.cpp     # repo root after cmake build
#   make build-whisper
#
# The binary produced is bin/vedox-whisper.  Pass --model ~/.vedox/models/ggml-base.en.bin
# when starting the server with voice enabled.
#
# See docs/how-to/build-whisper.md for the full step-by-step walkthrough.

WHISPER_ROOT ?= $(HOME)/whisper.cpp
WHISPER_BIN  := bin/vedox-whisper

# macOS: link Accelerate + CoreML frameworks; adjust for Linux (see docs).
WHISPER_CGO_CPPFLAGS := -I$(WHISPER_ROOT)/include
WHISPER_CGO_LDFLAGS  := \
  -L$(WHISPER_ROOT)/build/src \
  -L$(WHISPER_ROOT)/build/ggml/src \
  -lwhisper -lggml -lggml-base -lggml-cpu \
  -framework Accelerate -framework CoreML -framework Foundation

.PHONY: build test vet lint clean tidy build-whisper

build:
	@mkdir -p bin
	go build $(LDFLAGS) -o $(BIN) .
	@echo "Built: $(BIN) ($(VERSION))"

build-whisper:
	@mkdir -p bin
	@if [ ! -f "$(WHISPER_ROOT)/build/src/libwhisper.a" ]; then \
	  echo "ERROR: libwhisper.a not found at $(WHISPER_ROOT)/build/src/"; \
	  echo "       Set WHISPER_ROOT and build whisper.cpp first."; \
	  echo "       See docs/how-to/build-whisper.md for instructions."; \
	  exit 1; \
	fi
	CGO_ENABLED=1 \
	CGO_CPPFLAGS="$(WHISPER_CGO_CPPFLAGS)" \
	CGO_LDFLAGS="$(WHISPER_CGO_LDFLAGS)" \
	  go build -tags whisper $(LDFLAGS) -o $(WHISPER_BIN) .
	@echo "Built: $(WHISPER_BIN) ($(VERSION)) [whisper]"

test:
	go test -race -count=1 ./...

vet:
	go vet ./...

lint:
	staticcheck ./...

tidy:
	go mod tidy

clean:
	rm -rf bin/
