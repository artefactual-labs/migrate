package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestExecRequiresConfig(t *testing.T) {
	t.Parallel()

	var (
		ctx    = context.Background()
		stdin  = strings.NewReader("")
		stdout = io.Discard
		stderr = &bytes.Buffer{}
	)

	err := exec(ctx, []string{"migrate", "replicate"}, stdin, stdout, stderr)
	assert.Error(t, err, "config.json not found in standard locations")
}
