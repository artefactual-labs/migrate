package ssmock

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/rogpeppe/go-internal/testscript"

	"github.com/artefactual-labs/migrate/internal/testutil"
)

func TestScriptCmd(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) == 0 {
		ts.Fatalf("ssmock: missing subcommand")
	}
	sub := args[0]
	switch sub {
	case "start":
		ssmockStart(ts, neg, args[1:])
	case "snapshot":
		ssmockSnapshot(ts, neg, args[1:])
	default:
		ts.Fatalf("ssmock: unknown subcommand %q", sub)
	}
}

func ssmockStart(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("ssmock start: negation not supported")
	}
	if _, ok := getSsmockInstance(ts); ok {
		ts.Fatalf("ssmock start: simulator already running")
	}

	fs := flag.NewFlagSet("ssmock start", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", "", "path to the simulator TOML configuration")
	portFlag := fs.Int("port", 0, "port to listen on (default random)")
	updateConfigFlag := fs.Bool("update-config", false, "update migrate config.json with simulator settings")
	moveDelay := fs.Duration("move-delay", 0, "duration packages remain MOVING before completing a move")
	if err := fs.Parse(args); err != nil {
		ts.Fatalf("ssmock start: %v", err)
	}
	if *configPath == "" {
		ts.Fatalf("ssmock start: -config is required")
	}

	absConfig := ts.MkAbs(*configPath)
	port := *portFlag
	if port == 0 {
		p, err := testutil.FreePort()
		if err != nil {
			ts.Fatalf("ssmock start: acquire port: %v", err)
		}
		port = p
	}
	listen := fmt.Sprintf("127.0.0.1:%d", port)

	var (
		cfg *Config
		err error
	)
	if *updateConfigFlag {
		cfg, err = loadConfigAllowMissingListen(absConfig)
	} else {
		cfg, err = LoadConfig(absConfig)
	}
	if err != nil {
		ts.Fatalf("ssmock start: load config: %v", err)
	}

	cfg.Server.Listen = listen
	if err := cfg.Validate(); err != nil {
		ts.Fatalf("ssmock start: validate config: %v", err)
	}

	options := []Option{}
	if *moveDelay > 0 {
		options = append(options, WithMoveDelay(*moveDelay))
	}

	srv, stop, err := StartServer(context.Background(), cfg, options...)
	if err != nil {
		ts.Fatalf("ssmock start: %v", err)
	}

	baseURL := fmt.Sprintf("http://%s", listen)
	success := false
	defer func() {
		if success {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := stop(ctx); err != nil {
			ts.Logf("ssmock: cleanup error: %v", err)
		}
	}()

	ts.Setenv("SSMOCK_URL", baseURL)
	if *updateConfigFlag {
		if err := updateStorageServiceConfig(ts, baseURL); err != nil {
			ts.Fatalf("ssmock start: update config: %v", err)
		}
	}

	inst := &ssmockInstance{srv: srv, stop: stop, baseURL: baseURL}
	setSsmockInstance(ts, inst)
	ts.Defer(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := inst.stop(ctx); err != nil {
			ts.Logf("ssmock: shutdown error: %v", err)
		}
		clearSsmockInstance(ts)
	})

	success = true
	ts.Logf("ssmock simulator listening on %s", baseURL)
}

func ssmockSnapshot(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("ssmock snapshot: negation not supported")
	}
	if len(args) != 0 {
		ts.Fatalf("ssmock snapshot: unexpected arguments: %v", args)
	}
	inst, ok := getSsmockInstance(ts)
	if !ok {
		ts.Fatalf("ssmock snapshot: simulator not running")
	}

	snap := inst.srv.Snapshot()
	data, err := snap.MarshalTOML()
	if err != nil {
		ts.Fatalf("ssmock snapshot: marshal: %v", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	if _, err := ts.Stdout().Write(data); err != nil {
		ts.Fatalf("ssmock snapshot: write stdout: %v", err)
	}
}

func setSsmockInstance(ts *testscript.TestScript, inst *ssmockInstance) {
	ssmockMu.Lock()
	defer ssmockMu.Unlock()
	ssmockInstances[ts] = inst
}

func getSsmockInstance(ts *testscript.TestScript) (*ssmockInstance, bool) {
	ssmockMu.Lock()
	defer ssmockMu.Unlock()
	inst, ok := ssmockInstances[ts]
	return inst, ok
}

func clearSsmockInstance(ts *testscript.TestScript) {
	ssmockMu.Lock()
	defer ssmockMu.Unlock()
	delete(ssmockInstances, ts)
}

type ssmockInstance struct {
	srv     *Server
	stop    func(context.Context) error
	baseURL string
}

var (
	ssmockMu        sync.Mutex
	ssmockInstances = make(map[*testscript.TestScript]*ssmockInstance)
)

func updateStorageServiceConfig(ts *testscript.TestScript, baseURL string) error {
	configPath := ts.MkAbs("config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			data = []byte("{}")
		} else {
			return err
		}
	}
	if len(bytes.TrimSpace(data)) == 0 {
		data = []byte("{}")
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	config["ss_url"] = baseURL
	config["ss_user_name"] = "test-user"
	config["ss_api_key"] = "test-key"
	config["docker"] = false

	if _, ok := config["python_path"]; !ok {
		config["python_path"] = "python3"
	}
	wd, _ := os.Getwd()
	repoManage := filepath.Join(wd, "internal", "ssmock", "manage.py")
	if _, statErr := os.Stat(repoManage); statErr == nil {
		config["ss_manage_path"] = repoManage
	}

	envMap, _ := config["environment"].(map[string]any)
	if envMap == nil {
		envMap = make(map[string]any)
	}
	envMap["SSMOCK_URL"] = baseURL
	config["environment"] = envMap

	updatedData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, updatedData, 0o644)
}

func loadConfigAllowMissingListen(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	return &cfg, nil
}
