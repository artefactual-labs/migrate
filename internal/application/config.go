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
	Dashboard            Dashboard   `json:"dashboard"`
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

type Dashboard struct {
	ManagePath           string `json:"manage_path"`
	PythonPath           string `json:"python_path"`
	Lang                 string `json:"lang"`
	DjangoSettingsModule string `json:"django_settings_module"`
	DjangoAllowedHosts   string `json:"django_allowed_hosts"`
	DjangoSecretKey      string `json:"django_secret_key"`
	GunicornBind         string `json:"gunicorn_bind"`
	ElasticSearchServer  string `json:"elastic_search_url"`
	SSClientQuickTimeout string `json:"ss_client_quick_timeout"`
}
