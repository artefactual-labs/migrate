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
	    "django_settings_module": "settings",
	    "django_secret_key": "secret",
	    "django_allowed_hosts": "*",
	    "ss_gunicorn_bind": "0.0.0.0:8000",
	    "email_host": "localhost",
	    "ss_audit_log_middleware": "false",
	    "ss_db_url": "sqlite:///db",
	    "email_use_tls": "false",
	    "ss_prometheus_enabled": "false",
	    "default_from_email": "noreply@example.com",
	    "time_zone": "UTC",
	    "ss_gunicorn_workers": "2",
	    "requests_ca_bundle": ""
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
}
