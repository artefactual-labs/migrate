# Agent Development Guide

A file for [guiding coding agents](https://agents.md/).

## Commands

- **Build:** `go build -x ./cmd/...`
- **Test:** `go test ./...`
- **Test filter:**
  - `go test -run regexp ./...`
  - `go test ./internal/application`
- **Formatting and linting:** `make lint`

## Testing conventions

Prefer using assertion helpers from `gotest.tools/v3/assert` for Go tests.
For richer checks (errors, slices, maps, golden files), prefer the helpers in
`gotest.tools/v3/assert` and `gotest.tools/v3/golden` instead of handwritten
assertions or low-level `testing` helpers.
