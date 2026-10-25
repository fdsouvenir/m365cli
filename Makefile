SHELL := /bin/bash
VERSION = $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT = $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo "")
DATE = $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
.DEFAULT_GOAL := build

.PHONY: build m365cli m365cli-help help fmt fmt-check test ci

BIN_DIR := $(CURDIR)/bin
BIN := $(BIN_DIR)/m365cli
CMD := ./cmd/m365cli
GO := go
GOFLAGS ?= -mod=readonly
TEST_PKGS := ./cmd/... ./internal/...

LDFLAGS = -X github.com/fdsouvenir/m365cli/internal/cmd.version=$(VERSION) -X github.com/fdsouvenir/m365cli/internal/cmd.commit=$(COMMIT) -X github.com/fdsouvenir/m365cli/internal/cmd.date=$(DATE)

ifneq ($(filter m365cli,$(MAKECMDGOALS)),)
RUN_ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
$(eval $(RUN_ARGS):;@:)
endif

build:
	@mkdir -p $(BIN_DIR)
	@$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN) $(CMD)

m365cli: build
	@if [ -n "$(RUN_ARGS)" ]; then \
		$(BIN) $(RUN_ARGS); \
	elif [ -z "$(ARGS)" ]; then \
		$(BIN) --help; \
	else \
		$(BIN) $(ARGS); \
	fi

m365cli-help: build
	@$(BIN) --help

help: m365cli-help

fmt:
	@gofmt -w cmd internal

fmt-check:
	@files="$$(gofmt -l cmd internal)"; \
	if [ -n "$$files" ]; then \
		echo "gofmt needs to be run on:"; \
		echo "$$files"; \
		echo "run: make fmt"; \
		exit 1; \
	fi

test:
	@$(GO) test $(GOFLAGS) $(TEST_PKGS)

ci: fmt-check test
