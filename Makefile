# ruxx — build, test, and release helpers.
#
# `make` with no target builds the binary into bin/ruxx with the version
# stamped in from git (the values live in internal/version).

MODULE  := github.com/ppreeper/ruxx
BIN     := bin/ruxx
GO      ?= go

# Version stamp: the nearest release tag when there is one, else the dev default.
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null | sed -e 's/^v//')
ifeq ($(strip $(VERSION)),)
VERSION := 0.1.0-dev
endif
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -X $(MODULE)/internal/version.Version=$(VERSION) \
           -X $(MODULE)/internal/version.Commit=$(COMMIT) \
           -X $(MODULE)/internal/version.Date=$(DATE)

.PHONY: all build install test race lint fmt vet bench clean

all: build

build: ## Build bin/ruxx with the version stamped in.
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/ruxx

install: ## Install ruxx into GOBIN.
	$(GO) install -ldflags "$(LDFLAGS)" ./cmd/ruxx

test: ## Run the test suite.
	$(GO) test ./...

race: ## Run the test suite under the race detector.
	$(GO) test -race ./...

lint: fmt-check vet ## Fail if the tree is not gofmt-clean or fails go vet.

fmt: ## Rewrite the Go sources with gofmt.
	gofmt -w cmd internal

fmt-check:
	@out="$$(gofmt -l cmd internal)"; \
	if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

vet:
	$(GO) vet ./...

bench: ## Run the micro-benchmarks (and the corpus sweep when available).
	./scripts/bench.sh

clean: ## Remove build output and caches.
	rm -rf bin .ruxx_cache
