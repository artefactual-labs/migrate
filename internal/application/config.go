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

func LoadConfig() (*Config, string, error) {
	path, err := FindConfigPath()
	if err != nil {
		return nil, "", err
	}

	cfg, err := LoadConfigAt(path)
	if err != nil {
		return nil, "", err
	}

	return cfg, path, nil
}

func LoadConfigAt(path string) (*Config, error) {
	cfg := &Config{}

	if err := decodeConfigFile(path, cfg); err != nil {
		return nil, err
	}

	if err := normalizeConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func decodeConfigFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config.json: %w", err)
	}

	standard, err := hujson.Standardize(data)
	if err != nil {
		return fmt.Errorf("parse config.json: %w", err)
	}

	if err := json.Unmarshal(standard, cfg); err != nil {
		return fmt.Errorf("unmarshal config.json: %w", err)
	}

	return nil
}

func normalizeConfig(cfg *Config) error {
	if cfg.Temporal.TaskQueue == "" {
		cfg.Temporal.TaskQueue = "default"
	}

	if err := cfg.StorageService.applyDefaults(); err != nil {
		return err
	}

	if cfg.Database.Engine == "" {
		cfg.Database.Engine = "sqlite"
	}
	switch cfg.Database.Engine {
	case "sqlite":
		if cfg.Database.SQLite.Path == "" {
			cfg.Database.SQLite.Path = DefaultSQLitePath()
		}
	default:
		return fmt.Errorf("unsupported database engine %q", cfg.Database.Engine)
	}

	return nil
}

func FindConfigPath() (string, error) {
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

func DefaultConfig() *Config {
	cfg := &Config{
		Database: DatabaseConfig{
			Engine: "sqlite",
			SQLite: SQLiteConfig{
				Path: DefaultSQLitePath(),
			},
		},
		Temporal: TemporalConfig{
			Namespace: "default",
			Address:   "127.0.0.1:7233",
			TaskQueue: "default",
		},
	}

	_ = cfg.StorageService.applyDefaults()

	return cfg
}

func ApplyDefaults(cfg *Config) error {
	if cfg == nil {
		return errors.New("nil config")
	}
	return normalizeConfig(cfg)
}

// Config represents the application configuration loaded from config.json.
// Check out `config.json.example` for more details.
type Config struct {
	// Database configuration.
	Database DatabaseConfig `json:"database"`

	// Temporal workflow engine connection details.
	Temporal TemporalConfig `json:"temporal"`

	// Storage Service configuration.
	StorageService StorageServiceConfig `json:"storage_service"`

	// Workflow-level configuration toggles.
	Workflows WorkflowConfig `json:"workflows"`
}

type TemporalConfig struct {
	Namespace string `json:"namespace"`
	Address   string `json:"address"`
	TaskQueue string `json:"task_queue"`
}

type StorageServiceConfig struct {
	API        StorageServiceAPIConfig        `json:"api"`
	Management StorageServiceManagementConfig `json:"management"`
	Locations  StorageServiceLocationConfig   `json:"locations"`
}

func (c *StorageServiceConfig) applyDefaults() error {
	if c.Management.Mode == "" {
		if c.Management.Docker.Container != "" {
			c.Management.Mode = "docker"
		} else {
			c.Management.Mode = "host"
		}
	}

	switch c.Management.Mode {
	case "docker":
	case "host":
		if c.Management.Host.PythonPath == "" {
			c.Management.Host.PythonPath = "python3"
		}
	default:
		return fmt.Errorf("unsupported storage_service.management.mode %q", c.Management.Mode)
	}

	return nil
}

type StorageServiceAPIConfig struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	APIKey   string `json:"api_key"`
}

type StorageServiceManagementConfig struct {
	Mode string `json:"mode"`

	Docker StorageServiceDockerConfig `json:"docker"`
	Host   StorageServiceHostConfig   `json:"host"`
}

type StorageServiceDockerConfig struct {
	Container  string `json:"container"`
	ManagePath string `json:"manage_path"`
}

type StorageServiceHostConfig struct {
	PythonPath  string            `json:"python_path"`
	ManagePath  string            `json:"manage_path"`
	Environment map[string]string `json:"environment"`
}

type StorageServiceLocationConfig struct {
	SourceLocationID     string              `json:"source_location_id"`
	MoveTargetLocationID string              `json:"move_target_location_id"`
	ReplicationTargets   []ReplicationTarget `json:"replication_targets"`
}

type ReplicationTarget struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// WorkflowConfig holds configuration for individual workflows.
type WorkflowConfig struct {
	Move WorkflowMoveConfig `json:"move"`
}

// WorkflowMoveConfig controls behaviour specific to the move workflow.
type WorkflowMoveConfig struct {
	CheckFixity bool `json:"check_fixity"`
}

type DatabaseConfig struct {
	Engine string       `json:"engine"`
	SQLite SQLiteConfig `json:"sqlite"`
}

type SQLiteConfig struct {
	Path string `json:"path"`
}

func DefaultSQLitePath() string {
	dir, err := os.UserConfigDir()
	if err == nil && dir != "" {
		if _, statErr := os.Stat(dir); statErr == nil {
			return filepath.Join(dir, "migrate", "migrate.db")
		}
	}
	return "migrate.db"
}
