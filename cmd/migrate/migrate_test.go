package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// TODO: migrate uses globals (slog.SetDefault), avoid t.Parallel() for now.

func TestExecRequiresConfig(t *testing.T) {
	var (
		ctx    = context.Background()
		stdin  = strings.NewReader("")
		stdout = io.Discard
		stderr = &bytes.Buffer{}
	)

	err := exec(ctx, []string{"migrate", "replicate"}, stdin, stdout, stderr)
	assert.Error(t, err, "open config.json: no such file or directory")
}
