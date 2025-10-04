package application

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

var configExample = []byte(`{
	  "ss_url": "http://storage",
	  "ss_user_name": "user",
	  "ss_api_key": "key",
	  "temporal": {
	    "namespace": "default",
	    "address": "temporal:7233",
	    "taskQueue": "queue"
	  },
	  "move_location_uuid": "move-uuid",
	  "location_uuid": "loc-uuid",
	  "ss_manage_path": "/manage",
	  "python_path": "/python",
	  "docker": true,
	  "ss_container_name": "container",
	  "replication_locations": [
	    {"uuid": "rep-1", "name": "Replica One"}
	  ],
	  "environment": {
	    "DJANGO_SETTINGS_MODULE": "settings",
	    "DJANGO_SECRET_KEY": "secret",
	    "DJANGO_ALLOWED_HOSTS": "*",
	    "SS_GUNICORN_BIND": "0.0.0.0:8000",
	    "EMAIL_HOST": "localhost",
	    "SS_AUDIT_LOG_MIDDLEWARE": "false",
	    "SS_DB_URL": "sqlite:///db",
	    "EMAIL_USE_TLS": "false",
	    "SS_PROMETHEUS_ENABLED": "false",
	    "DEFAULT_FROM_EMAIL": "noreply@example.com",
	    "TIME_ZONE": "UTC",
	    "SS_GUNICORN_WORKERS": "2",
	    "REQUESTS_CA_BUNDLE": ""
	  }
	}`)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	exe, err := os.Executable()
	assert.NilError(t, err)
	configPath := filepath.Join(filepath.Dir(exe), "config.json")

	assert.NilError(t, os.WriteFile(configPath, configExample, 0o644))
	t.Cleanup(func() {
		_ = os.Remove(configPath)
	})

	cfg, err := LoadConfig()
	assert.NilError(t, err)
	assert.Equal(t, cfg.SSURL, "http://storage")
	assert.Equal(t, cfg.SSUserName, "user")
	assert.Equal(t, cfg.MoveLocationUUID, "move-uuid")
	assert.Equal(t, cfg.Environment["DJANGO_SETTINGS_MODULE"], "settings")
	assert.Equal(t, cfg.Temporal.Namespace, "default")
	assert.Equal(t, cfg.Temporal.Address, "temporal:7233")
	assert.Equal(t, cfg.Temporal.TaskQueue, "queue")
}
