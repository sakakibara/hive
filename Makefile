BINARY := hive
MODULE := github.com/sakakibara/hive
VERSION ?= dev
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -X $(MODULE)/internal/cli.version=$(VERSION) \
           -X $(MODULE)/internal/cli.commit=$(COMMIT) \
           -X $(MODULE)/internal/cli.date=$(DATE)

.PHONY: build test clean install

build:
	go build -ldflags '$(LDFLAGS)' -o $(BINARY) ./cmd/hive/

test:
	go test ./...

clean:
	rm -f $(BINARY)

install:
	go install -ldflags '$(LDFLAGS)' ./cmd/hive/
