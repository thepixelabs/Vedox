# Makefile for the Vedox CLI (apps/cli)
#
# Targets:
#   make build    — compile vedox binary to ./bin/vedox
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

.PHONY: build test vet lint clean tidy

build:
	@mkdir -p bin
	go build $(LDFLAGS) -o $(BIN) .
	@echo "Built: $(BIN) ($(VERSION))"

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
