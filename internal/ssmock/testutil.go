package ssmock

import (
	"context"
	"errors"
	"net"
	"syscall"
	"testing"
	"time"
)

// StartTestServer starts the simulator for use in tests and registers a cleanup
// hook on t to ensure the server stops when the test finishes.
func StartTestServer(t *testing.T, cfg *Config, opts ...Option) *Server {
	t.Helper()

	srv, stop, err := StartServer(t.Context(), cfg, opts...)
	if err != nil {
		var opErr *net.OpError
		if errors.As(err, &opErr) && errors.Is(opErr.Err, syscall.EPERM) {
			t.Skipf("skipping simulator tests: %v", err)
		}
		t.Fatalf("start simulator: %v", err)
	}

	t.Cleanup(func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(t.Context(), time.Second)
		defer shutdownCancel()
		if err := stop(shutdownCtx); err != nil {
			t.Fatalf("shutdown simulator: %v", err)
		}
	})

	return srv
}

// StartServer bootstraps the simulator without requiring a testing dependency.
// The caller is responsible for invoking the returned stop function when done.
func StartServer(ctx context.Context, cfg *Config, opts ...Option) (*Server, func(context.Context) error, error) {
	srv, err := NewServer(cfg, opts...)
	if err != nil {
		return nil, nil, err
	}
	if err := srv.Start(); err != nil {
		return nil, nil, err
	}

	readyCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := srv.WaitReady(readyCtx); err != nil {
		_ = srv.Shutdown(ctx)
		return nil, nil, err
	}

	stop := func(shutdownCtx context.Context) error {
		return srv.Shutdown(shutdownCtx)
	}
	return srv, stop, nil
}
