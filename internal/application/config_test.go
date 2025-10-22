package application

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	// We need to write config.json next to the test executable.
	exe, err := os.Executable()
	assert.NilError(t, err)
	configPath := filepath.Join(filepath.Dir(exe), "config.json")

	// Will use config.json.example from the repo root.
	dir, err := os.Getwd()
	assert.NilError(t, err)
	example := filepath.Join(dir, "../../config.json.example")
	data, err := os.ReadFile(example)
	assert.NilError(t, err)

	assert.NilError(t, os.WriteFile(configPath, data, 0o644))
	t.Cleanup(func() {
		_ = os.Remove(configPath)
	})

	cfg, err := LoadConfig()
	assert.NilError(t, err)
	assert.Equal(t, cfg.SSURL, "http://localhost:62081")
	assert.Equal(t, cfg.SSUserName, "test")
	assert.Equal(t, cfg.Environment["DJANGO_SETTINGS_MODULE"], "archivematica.storage_service.storage_service.settings.local")
	assert.Equal(t, cfg.Temporal.Address, "127.0.0.1:7233")
	assert.Equal(t, cfg.Temporal.TaskQueue, "default-task-queue")
	assert.Equal(t, cfg.Database.Engine, "sqlite")
	assert.Equal(t, cfg.Database.SQLite.Path, defaultSQLitePath())
}
