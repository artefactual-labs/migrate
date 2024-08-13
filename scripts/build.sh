#!/usr/bin/env bash

# Options to build a statically linked Go binary with CGO enabled (need CGO for sqlite access). Critical step is to use musl as the libc.
CGO_ENABLED=1 CC=musl-gcc go build -ldflags="-extldflags=-static -linkmode external" -o=./migrate ./cmd/migrate/main.go
