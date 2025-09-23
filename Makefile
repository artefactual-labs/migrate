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

fmt: # @HELP Format the project Go files with golangci-lint.
fmt: FMT_FLAGS ?=
fmt: tool-golangci-lint
	golangci-lint fmt $(FMT_FLAGS)

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
