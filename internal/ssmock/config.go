package ssmock

import (
	"errors"
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Server    ServerConfig     `toml:"server"`
	Locations []LocationConfig `toml:"location"`
}

type ServerConfig struct {
	Listen string `toml:"listen"`
}

type LocationConfig struct {
	ID          string          `toml:"id"`
	Description string          `toml:"description"`
	Purpose     string          `toml:"purpose"`
	Path        string          `toml:"path"`
	Relative    string          `toml:"relative_path"`
	Space       string          `toml:"space"`
	Quota       string          `toml:"quota"`
	Pipeline    []string        `toml:"pipeline"`
	Packages    []PackageConfig `toml:"packages"`
}

type PackageConfig struct {
	ID                string   `toml:"id"`
	Status            string   `toml:"status"`
	Size              uint64   `toml:"size"`
	CurrentPath       string   `toml:"current_path"`
	CurrentFullPath   string   `toml:"current_full_path"`
	Encrypted         bool     `toml:"encrypted"`
	OriginPipeline    string   `toml:"origin_pipeline"`
	PackageType       string   `toml:"package_type"`
	ReplicatedPackage string   `toml:"replicated_package"`
	Replicas          []string `toml:"replicas"`
	StoredDate        string   `toml:"stored_date"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config is nil")
	}
	if c.Server.Listen == "" {
		return errors.New("server.listen is required")
	}
	if len(c.Locations) == 0 {
		return errors.New("at least one location must be defined")
	}

	seenLocations := make(map[string]struct{}, len(c.Locations))
	seenPackages := make(map[string]string)
	for _, loc := range c.Locations {
		if loc.ID == "" {
			return errors.New("location.id is required")
		}
		if _, ok := seenLocations[loc.ID]; ok {
			return fmt.Errorf("duplicate location id %q", loc.ID)
		}
		seenLocations[loc.ID] = struct{}{}

		for _, pkg := range loc.Packages {
			if pkg.ID == "" {
				return fmt.Errorf("package id missing in location %s", loc.ID)
			}
			if otherLoc, ok := seenPackages[pkg.ID]; ok {
				return fmt.Errorf("duplicate package id %q defined in locations %s and %s", pkg.ID, otherLoc, loc.ID)
			}
			seenPackages[pkg.ID] = loc.ID
		}
	}
	return nil
}
