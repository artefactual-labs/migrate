package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	osexec "os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/artefactual-labs/migrate/internal/ssmock"
	"github.com/artefactual-labs/migrate/internal/testutil"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"migrate": main,
	})
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata",
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"temporal": temporalCmd,
			"worker":   workerCmd,
			"ssmock":   ssmock.TestScriptCmd,
			"esmock":   elasticCmd,
		},
	})
}

func workerCmd(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("worker: negation not supported")
	}

	// Parse flags
	fs := flag.NewFlagSet("worker", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	updateConfigFlag := fs.Bool("update-config", false, "update migrate config.json before starting worker")
	if err := fs.Parse(args); err != nil {
		ts.Fatalf("worker: %v", err)
	}

	// Optionally update the config file with Temporal address
	if *updateConfigFlag {
		temporalAddr := ts.Getenv("TEMPORAL_ADDRESS")
		if temporalAddr == "" {
			ts.Fatalf("worker: TEMPORAL_ADDRESS not set, start temporal server first")
		}
		if err := updateConfig(ts, temporalAddr); err != nil {
			ts.Fatalf("worker: unable to update config: %v", err)
		}
	}

	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Prepare to call exec with the "worker" subcommand
	workerArgs := []string{"worker"}
	if len(fs.Args()) > 0 {
		workerArgs = append(workerArgs, fs.Args()...)
	}

	// Run the worker in a goroutine
	errCh := make(chan error, 1)
	go func() {
		var stdout, stderr bytes.Buffer
		err := exec(ctx, workerArgs, os.Stdin, &stdout, &stderr)
		if stdout.Len() > 0 {
			_, _ = fmt.Fprint(ts.Stdout(), stdout.String())
		}
		if stderr.Len() > 0 {
			_, _ = fmt.Fprint(ts.Stderr(), stderr.String())
		}
		errCh <- err
	}()

	// Set up cleanup
	var errConsumed bool
	ts.Defer(func() {
		if errConsumed {
			return
		}
		cancel()
		select {
		case err := <-errCh:
			errConsumed = true
			if err != nil && ctx.Err() == nil {
				ts.Logf("worker: exit error: %v", err)
			}
		case <-time.After(2 * time.Second):
			errConsumed = true
			ts.Logf("worker: shutdown timeout")
		}
	})

	// Give the worker a moment to start
	time.Sleep(500 * time.Millisecond)

	// Check if it crashed immediately
	select {
	case err := <-errCh:
		errConsumed = true
		if err != nil {
			ts.Fatalf("worker: failed to start: %v", err)
		}
		ts.Fatalf("worker: exited unexpectedly")
	default:
		ts.Logf("worker: started")
	}
}

func temporalCmd(ts *testscript.TestScript, _ bool, args []string) {
	port, err := testutil.FreePort()
	if err != nil {
		ts.Fatalf("temporal: get free port: %v", err)
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ts.Setenv("TEMPORAL_ADDRESS", addr)

	if len(args) > 0 && args[0] == "--update-config" {
		if err := updateConfig(ts, addr); err != nil {
			ts.Fatalf("temporal: unable to update config: %v", err)
		}
	}

	stdout := &safeBuffer{}
	stderr := &safeBuffer{}
	cacheDir := ts.MkAbs(".cache")
	modCache := ts.MkAbs(".gomodcache")
	goCache := ts.MkAbs(".gocache")
	homeDir := ts.MkAbs(".home")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		ts.Fatalf("temporal: create cache dir: %v", err)
	}
	if err := os.MkdirAll(modCache, 0o755); err != nil {
		ts.Fatalf("temporal: create mod cache dir: %v", err)
	}
	if err := os.MkdirAll(goCache, 0o755); err != nil {
		ts.Fatalf("temporal: create go cache dir: %v", err)
	}
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		ts.Fatalf("temporal: create home dir: %v", err)
	}

	cmd := osexec.Command("go", []string{
		"tool",
		"bine",
		"run",
		"--",
		"temporal",
		"server",
		"start-dev",
		"--headless",
		"--port",
		strconv.Itoa(port),
	}...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = append(os.Environ(),
		"XDG_CACHE_HOME="+cacheDir,
		"GOMODCACHE="+modCache,
		"GOCACHE="+goCache,
		"HOME="+homeDir,
	)

	if err := cmd.Start(); err != nil {
		ts.Fatalf("temporal: start: %v", err)
	}

	waitCh := make(chan error, 1)
	var waitConsumed bool
	go func() {
		waitCh <- cmd.Wait()
	}()

	ts.Defer(func() {
		if waitConsumed {
			return
		}
		_ = cmd.Process.Signal(os.Interrupt)
		select {
		case err := <-waitCh:
			waitConsumed = true
			if err != nil {
				ts.Logf("temporal: exit error: %v", err)
				logTemporalOutput(ts, stdout, stderr)
			}
		case <-time.After(2 * time.Second):
			_ = cmd.Process.Kill()
			err := <-waitCh
			waitConsumed = true
			if err != nil {
				ts.Logf("temporal: exit error after kill: %v", err)
				logTemporalOutput(ts, stdout, stderr)
			}
		}
	})

	deadline := time.Now().Add(600 * time.Second)
	for {
		select {
		case err := <-waitCh:
			waitConsumed = true
			if err != nil {
				ts.Logf("temporal: exited before ready: %v", err)
				logTemporalOutput(ts, stdout, stderr)
				ts.Fatalf("temporal: exited before ready: %v", err)
			}
			ts.Fatalf("temporal: exited before ready")
		default:
		}
		conn, err := net.DialTimeout("tcp", addr, 250*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		if time.Now().After(deadline) {
			ts.Logf("temporal: server not ready: %v", err)
			logTemporalOutput(ts, stdout, stderr)
			ts.Fatalf("temporal: server not ready: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	ts.Logf("temporal dev server listening on %s", addr)
}

type elasticDocument struct {
	ID       string `json:"id"`
	UUID     string `json:"uuid"`
	Location string `json:"location"`
	FilePath string `json:"filePath"`
}

func elasticCmd(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("esmock: negation not supported")
	}

	fs := flag.NewFlagSet("esmock", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	updateConfigFlag := fs.Bool("update-config", false, "update migrate config.json before starting Elasticsearch mock")
	if err := fs.Parse(args); err != nil {
		ts.Fatalf("esmock: %v", err)
	}

	data, err := os.ReadFile(ts.MkAbs("esmock.json"))
	if err != nil {
		ts.Fatalf("esmock: read esmock.json: %v", err)
	}

	var cfg struct {
		Documents []elasticDocument `json:"documents"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		ts.Fatalf("esmock: decode esmock.json: %v", err)
	}

	docs := make(map[string]elasticDocument, len(cfg.Documents))
	for _, doc := range cfg.Documents {
		docs[doc.UUID] = doc
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		ts.Fatalf("esmock: listen: %v", err)
	}
	addr := "http://" + listener.Addr().String()
	ts.Setenv("ELASTIC_ADDRESS", addr)

	if *updateConfigFlag {
		temporalAddr := ts.Getenv("TEMPORAL_ADDRESS")
		if temporalAddr == "" {
			ts.Fatalf("esmock: TEMPORAL_ADDRESS not set, start temporal server first")
		}
		if err := updateConfig(ts, temporalAddr); err != nil {
			ts.Fatalf("esmock: unable to update config: %v", err)
		}
	}

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/aips/_search":
				var query struct {
					Query struct {
						Term struct {
							UUID string `json:"uuid"`
						} `json:"term"`
					} `json:"query"`
				}
				if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				doc, ok := docs[query.Query.Term.UUID]
				w.Header().Set("Content-Type", "application/json")
				if !ok {
					_, _ = fmt.Fprint(w, `{"hits":{"total":0,"hits":[]}}`)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{
					"hits": map[string]any{
						"total": 1,
						"hits": []map[string]any{{
							"_index": "aips",
							"_type":  "_doc",
							"_id":    doc.ID,
							"_score": 1,
							"_source": map[string]any{
								"uuid":     doc.UUID,
								"location": doc.Location,
								"filePath": doc.FilePath,
							},
						}},
					},
				})
			case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/aips/_update/"):
				http.Error(w, "unexpected update request", http.StatusInternalServerError)
			default:
				http.NotFound(w, r)
			}
		}),
	}

	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			ts.Logf("esmock: serve error: %v", err)
		}
	}()

	ts.Defer(func() {
		_ = srv.Shutdown(context.Background())
	})

	ts.Logf("elasticsearch mock listening on %s", addr)
}

func updateConfig(ts *testscript.TestScript, temporalAddr string) error {
	configPath := ts.MkAbs("config.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	if config["temporal"] == nil {
		config["temporal"] = make(map[string]any)
	}
	temporalConfig, ok := config["temporal"].(map[string]any)
	if !ok {
		// Handle case where "temporal" exists but is not a map.
		// For this implementation, we'll overwrite it.
		temporalConfig = make(map[string]any)
		config["temporal"] = temporalConfig
	}
	temporalConfig["address"] = temporalAddr

	if elasticAddr := ts.Getenv("ELASTIC_ADDRESS"); elasticAddr != "" {
		if config["elastic"] == nil {
			config["elastic"] = make(map[string]any)
		}
		elasticConfig, ok := config["elastic"].(map[string]any)
		if !ok {
			elasticConfig = make(map[string]any)
			config["elastic"] = elasticConfig
		}
		elasticConfig["host"] = elasticAddr
	}

	updatedData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, updatedData, 0o644)
}

func logTemporalOutput(ts *testscript.TestScript, stdout, stderr *safeBuffer) {
	if out := strings.TrimSpace(stdout.String()); out != "" {
		ts.Logf("temporal stdout:\n%s", out)
	}
	if errOut := strings.TrimSpace(stderr.String()); errOut != "" {
		ts.Logf("temporal stderr:\n%s", errOut)
	}
}

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (sb *safeBuffer) Write(p []byte) (int, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.Write(p)
}

func (sb *safeBuffer) String() string {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.String()
}
