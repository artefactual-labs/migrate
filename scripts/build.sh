#!/usr/bin/env bash

# Build the migrate binary without CGO; modernc's SQLite driver is pure Go.
CGO_ENABLED=0 go build -o ./migrate ./cmd/migrate/main.go
