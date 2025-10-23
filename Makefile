PROJECT := migrate
SHELL   := /bin/bash

.DEFAULT_GOAL := help
.PHONY: *

DBG_MAKEFILE ?=
ifeq ($(DBG_MAKEFILE),1)
    $(warning ***** starting Makefile for goal(s) "$(MAKECMDGOALS)")
    $(warning ***** $(shell date))
else
    # If we're not debugging the Makefile, don't echo recipes.
    MAKEFLAGS += -s
endif

# Configure bine.
export PATH := $(shell go tool bine path):$(PATH)

build: # @HELP Build migrate.
	env CGO_ENABLED=0 go build -trimpath -o $(CURDIR)/migrate ./cmd/migrate

deadcode: # @HELP Find unreachable functions.
deadcode: tool-deadcode
	@output=$$({ deadcode -test ./... || true; }); \
	if [[ -n "$$output" ]]; then \
	  echo "Unreachable code found:"; \
	  echo "$$output"; \
	  exit 1; \
	fi

deps: # @HELP List oudated dependencies.
deps: ARGS ?= -update -direct
deps: tool-go-mod-outdated
	go list -u -m -json all | go-mod-outdated $(ARGS)

fmt: # @HELP Format the project Go files with golangci-lint.
fmt: FMT_FLAGS ?=
fmt: tool-golangci-lint
	golangci-lint fmt $(FMT_FLAGS)

gen: # @HELP Generate code.
gen: tool-bobgen-sqlite
	bobgen-sqlite -c bobgen.yaml

help: # @HELP Print this message.
help:
	echo "TARGETS:"
	grep -E '^.*: *# *@HELP' Makefile             \
	    | awk '                                   \
	        BEGIN {FS = ": *# *@HELP"};           \
	        { printf "  %-30s %s\n", $$1, $$2 };  \
	    '

lint: # @HELP Lint the project Go files with golangci-lint (linters + formatters).
lint: LINT_FLAGS ?= --fix=1
lint: tool-golangci-lint
	golangci-lint run $(LINT_FLAGS)

tool-%:
	@go tool bine get $* 1> /dev/null

tools: # @HELP Install all tools managed by bine.
tools:
	go tool bine sync
