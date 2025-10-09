package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/tailscale/hujson"
)

func LoadConfig() (*Config, error) {
	cfg := &Config{}

	path, err := findConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config.json: %w", err)
	}

	standard, err := hujson.Standardize(data)
	if err != nil {
		return nil, fmt.Errorf("parse config.json: %w", err)
	}

	if err := json.Unmarshal(standard, cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config.json: %w", err)
	}

	// Set defaults.
	if cfg.Temporal.TaskQueue == "" {
		cfg.Temporal.TaskQueue = "default"
	}

	return cfg, nil
}

func findConfigPath() (string, error) {
	const name = "config.json"

	var candidates []string

	seen := map[string]struct{}{}
	add := func(path string) {
		if path == "" {
			return
		}
		if !filepath.IsAbs(path) {
			path = filepath.Clean(path)
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		candidates = append(candidates, path)
	}

	if wd, err := os.Getwd(); err == nil {
		add(filepath.Join(wd, name))
	}

	if exe, err := os.Executable(); err == nil {
		add(filepath.Join(filepath.Dir(exe), name))
	}

	if cfgDir, err := os.UserConfigDir(); err == nil {
		add(filepath.Join(cfgDir, "migrate", name))
	}

	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, name))
	}

	var errs []error
	for _, path := range candidates {
		info, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			errs = append(errs, err)
			continue
		}
		if info.IsDir() {
			continue
		}
		return path, nil
	}

	if len(errs) > 0 {
		return "", fmt.Errorf("locate config.json: %w", errs[len(errs)-1])
	}

	return "", fmt.Errorf("config.json not found in standard locations")
}

// Config represents the application configuration loaded from config.json.
// Check out `config.json.example` for more details.
type Config struct {
	// Temporal workflow engine connection details.
	Temporal TemporalConfig `json:"temporal"`

	// Storage service API connection details.
	SSURL      string `json:"ss_url"`
	SSUserName string `json:"ss_user_name"`
	SSAPIKey   string `json:"ss_api_key"`

	// Source location used for move and replicate workflows.
	LocationUUID string `json:"location_uuid"`

	// Location used for move workflow operations.
	MoveLocationUUID string `json:"move_location_uuid"`

	// Replication targets available to the replicate workflow.
	ReplicationLocations []Location `json:"replication_locations"`

	// Whether we'll use `docker exec` or `exec.Command` to run SS management commands.
	Docker          bool   `json:"docker"`
	SSContainerName string `json:"ss_container_name"`

	// Paths to the SS management command and Python interpreter.
	SSManagePath string `json:"ss_manage_path"`
	PythonPath   string `json:"python_path"`

	// Only used by exec.Command, not Docker.
	Environment map[string]string `json:"environment"`
}

type Location struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

type TemporalConfig struct {
	Namespace string `json:"namespace"`
	Address   string `json:"address"`
	TaskQueue string `json:"task_queue"`
}
