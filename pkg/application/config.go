package application

type Config struct {
	SSURL                   string      `json:"ss_url"`
	SSUserName              string      `json:"ss_user_name"`
	SSAPIKey                string      `json:"ss_api_key"`
	ReplicationLocationUUID string      `json:"replication_location_uuid"`
	MoveLocationUUID        string      `json:"move_location_uuid"`
	SSManagePath            string      `json:"ss_manage_path"`
	ElasticSearchURL        string      `json:"elastic_search_url"`
	PythonPath              string      `json:"python_path"`
	LocationUUID            string      `json:"location_uuid"`
	CheckFixity             bool        `json:"check_fixity"`
	Move                    bool        `json:"move"`
	Clean                   bool        `json:"clean"`
	ReIndex                 bool        `json:"re_index"`
	Replicate               bool        `json:"Replicate"`
	StagingPath             string      `json:"staging_path"`
	LocalCopyPath           string      `json:"local_copy_path"`
	Docker                  bool        `json:"docker"`
	SSContainerName         string      `json:"ss_container_name"`
	ReplicationLocations    []Location  `json:"replication_locations"`
	UseTemporal             bool        `json:"use_temporal"`
	Environment             Environment `json:"environment"`
	Dashboard               Dashboard   `json:"dashboard"`
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
	ElasticSearchTimeout string `json:"elastic_search_timeout"`
	SSClientQuickTimeout string `json:"ss_client_quick_timeout"`
	AuditLogMiddleware   string `json:"audit_log_middleware"`
	PrometheusEnabled    string `json:"prometheus_enabled"`
	EmailHost            string `json:"email_host"`
	EmailDefaultFrom     string `json:"email_default_from"`
	TimeZone             string `json:"time_zone"`
	SearchEnabled        string `json:"search_enabled"`
	ClientHost           string `json:"client_host"`
	ClientDatabase       string `json:"client_database"`
	ClientUser           string `json:"client_user"`
	ClientPassword       string `json:"client_password"`
	RequestsCABundle     string `json:"requests_ca_bundle"`
}
