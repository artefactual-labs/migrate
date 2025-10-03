package application

type Config struct {
	SSURL                string      `json:"ss_url"`
	SSUserName           string      `json:"ss_user_name"`
	SSAPIKey             string      `json:"ss_api_key"`
	MoveLocationUUID     string      `json:"move_location_uuid"`
	SSManagePath         string      `json:"ss_manage_path"`
	PythonPath           string      `json:"python_path"`
	LocationUUID         string      `json:"location_uuid"`
	Docker               bool        `json:"docker"`
	SSContainerName      string      `json:"ss_container_name"`
	ReplicationLocations []Location  `json:"replication_locations"`
	Environment          Environment `json:"environment"`
}

type Location struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

type Environment struct {
	DjangoSettingsModule string `json:"django_settings_module"`
	DjangoSecretKey      string `json:"django_secret_key"`
	DjangoAllowedHosts   string `json:"django_allowed_hosts"`
	SsGunicornBind       string `json:"ss_gunicorn_bind"`
	EmailHost            string `json:"email_host"`
	SsAuditLogMiddleware string `json:"ss_audit_log_middleware"`
	SsDbUrl              string `json:"ss_db_url"`
	EmailUseTls          string `json:"email_use_tls"`
	SsPrometheusEnabled  string `json:"ss_prometheus_enabled"`
	DefaultFromEmail     string `json:"default_from_email"`
	TimeZone             string `json:"time_zone"`
	SsGunicornWorkers    string `json:"ss_gunicorn_workers"`
	RequestsCaBundle     string `json:"requests_ca_bundle"`
}
