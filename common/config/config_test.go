package config

import "testing"

func TestDefaultAllowsInsecureAdminCompatibility(t *testing.T) {
	cfg := Default()
	if !cfg.Auth.AllowInsecureDefaultAdmin {
		t.Fatal("default should allow insecure admin login for compatibility")
	}
	if cfg.Auth.InitialAdminUsername != "admin" {
		t.Fatalf("initial admin username = %q", cfg.Auth.InitialAdminUsername)
	}
	if cfg.Server.AdminMaxBodyBytes != DefaultAdminMaxBodyBytes {
		t.Fatalf("admin max body = %d", cfg.Server.AdminMaxBodyBytes)
	}
	if cfg.Relay.MaxBodyBytes != DefaultRelayMaxBodyBytes {
		t.Fatalf("relay max body = %d", cfg.Relay.MaxBodyBytes)
	}
}

func TestEnvOverridesSecurityFields(t *testing.T) {
	t.Setenv("APIRELAY_ADMIN_MAX_BODY_BYTES", "1234")
	t.Setenv("APIRELAY_RELAY_MAX_BODY_BYTES", "5678")
	t.Setenv("APIRELAY_CORS_ALLOWED_ORIGINS", "https://a.example, https://b.example,https://a.example")
	t.Setenv("APIRELAY_INITIAL_ADMIN_USERNAME", "root")
	t.Setenv("APIRELAY_INITIAL_ADMIN_PASSWORD", "secret")
	t.Setenv("APIRELAY_ALLOW_INSECURE_DEFAULT_ADMIN", "false")
	t.Setenv("APIRELAY_LOGIN_MAX_FAILURES", "3")
	t.Setenv("APIRELAY_LOGIN_FAILURE_WINDOW_SECONDS", "60")
	t.Setenv("APIRELAY_LOGIN_LOCKOUT_SECONDS", "120")
	t.Setenv("APIRELAY_REQUEST_TIMEOUT", "9")

	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.AdminMaxBodyBytes != 1234 || cfg.Relay.MaxBodyBytes != 5678 {
		t.Fatalf("body limits not overridden: admin=%d relay=%d", cfg.Server.AdminMaxBodyBytes, cfg.Relay.MaxBodyBytes)
	}
	if got := cfg.Server.CORSAllowedOrigins; len(got) != 2 || got[0] != "https://a.example" || got[1] != "https://b.example" {
		t.Fatalf("cors origins = %#v", got)
	}
	if cfg.Auth.InitialAdminUsername != "root" || cfg.Auth.InitialAdminPassword != "secret" {
		t.Fatalf("admin bootstrap not overridden: %#v", cfg.Auth)
	}
	if cfg.Auth.AllowInsecureDefaultAdmin {
		t.Fatal("allow insecure should be false after env override")
	}
	if cfg.Auth.LoginMaxFailures != 3 || cfg.Auth.LoginFailureWindowSeconds != 60 || cfg.Auth.LoginLockoutSeconds != 120 {
		t.Fatalf("login limiter settings not overridden: %#v", cfg.Auth)
	}
	if cfg.Relay.RequestTimeout != 9 {
		t.Fatalf("request timeout = %d", cfg.Relay.RequestTimeout)
	}
}

func TestEnvOverridesRelayRuntimeFields(t *testing.T) {
	t.Setenv("APIRELAY_MAX_RETRIES", "4")
	t.Setenv("APIRELAY_CHANNEL_MAX_RETRIES", "0")
	t.Setenv("APIRELAY_COOLDOWN_SECONDS", "45")
	t.Setenv("APIRELAY_REQUEST_TIMEOUT", "12")
	t.Setenv("APIRELAY_RELAY_MAX_BODY_BYTES", "3333")
	t.Setenv("APIRELAY_DEFAULT_GROUP", "vip")
	t.Setenv("APIRELAY_CIRCUIT_BREAKER_FAILURE_THRESHOLD", "7")
	t.Setenv("APIRELAY_CIRCUIT_BREAKER_SUCCESS_THRESHOLD", "3")
	t.Setenv("APIRELAY_CIRCUIT_BREAKER_TIMEOUT_SECONDS", "90")
	t.Setenv("APIRELAY_CIRCUIT_BREAKER_ERROR_RATE_THRESHOLD", "0.75")
	t.Setenv("APIRELAY_CIRCUIT_BREAKER_MIN_REQUESTS", "20")
	t.Setenv("APIRELAY_CIRCUIT_BREAKER_WINDOW_SECONDS", "120")

	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Relay.MaxRetries != 4 || cfg.Relay.ChannelMaxRetries != 0 || cfg.Relay.CooldownSeconds != 45 || cfg.Relay.RequestTimeout != 12 {
		t.Fatalf("relay retry/timeout env overrides not applied: %#v", cfg.Relay)
	}
	if cfg.Relay.MaxBodyBytes != 3333 || cfg.Relay.DefaultGroup != "vip" {
		t.Fatalf("relay body/group env overrides not applied: %#v", cfg.Relay)
	}
	cb := cfg.Relay.CircuitBreaker
	if cb.FailureThreshold != 7 || cb.SuccessThreshold != 3 || cb.TimeoutSeconds != 90 || cb.ErrorRateThreshold != 0.75 || cb.MinRequests != 20 || cb.WindowSeconds != 120 {
		t.Fatalf("circuit breaker env overrides not applied: %#v", cb)
	}
}

func TestNormalizeSecurityDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.Normalize()
	if cfg.Server.AdminMaxBodyBytes != DefaultAdminMaxBodyBytes {
		t.Fatalf("admin max body = %d", cfg.Server.AdminMaxBodyBytes)
	}
	if cfg.Relay.MaxBodyBytes != DefaultRelayMaxBodyBytes {
		t.Fatalf("relay max body = %d", cfg.Relay.MaxBodyBytes)
	}
	if cfg.Auth.LoginMaxFailures != DefaultLoginMaxFailures {
		t.Fatalf("login max failures = %d", cfg.Auth.LoginMaxFailures)
	}
}

func TestNormalizeCircuitBreakerErrorRateCap(t *testing.T) {
	cfg := Default()
	cfg.Relay.CircuitBreaker.ErrorRateThreshold = 2
	cfg.Normalize()
	if cfg.Relay.CircuitBreaker.ErrorRateThreshold != 1 {
		t.Fatalf("error rate threshold = %v", cfg.Relay.CircuitBreaker.ErrorRateThreshold)
	}
}

func TestConfigFilePathDefaultsAndCleans(t *testing.T) {
	t.Cleanup(func() { SetConfigFilePath("") })
	SetConfigFilePath("")
	if got := ConfigFilePath(); got != DefaultConfigPath {
		t.Fatalf("default config path = %q", got)
	}
	SetConfigFilePath("./configs/../apirelay.yaml")
	if got := ConfigFilePath(); got != "apirelay.yaml" {
		t.Fatalf("cleaned config path = %q", got)
	}
}

func TestValidateYAMLRejectsInvalidConfigShape(t *testing.T) {
	if err := ValidateYAML([]byte("server:\n  port: 3001\nrelay:\n  max_retries: 2\n")); err != nil {
		t.Fatalf("valid yaml rejected: %v", err)
	}
	if err := ValidateYAML([]byte("server:\n  port: [bad]\n")); err == nil {
		t.Fatal("expected invalid config yaml to be rejected")
	}
}

func TestTrustedProxiesEnvAndYAML(t *testing.T) {
	// 环境变量覆盖并去重去空
	t.Setenv("APIRELAY_TRUSTED_PROXIES", "10.0.0.0/8, 127.0.0.1 ,10.0.0.0/8")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Server.TrustedProxies
	if len(got) != 2 || got[0] != "10.0.0.0/8" || got[1] != "127.0.0.1" {
		t.Fatalf("trusted proxies = %#v", got)
	}
}

func TestTrustedProxiesDefaultEmpty(t *testing.T) {
	cfg := Default()
	cfg.Normalize()
	if len(cfg.Server.TrustedProxies) != 0 {
		t.Fatalf("default trusted proxies should be empty, got %#v", cfg.Server.TrustedProxies)
	}
}

func TestLogDatabaseDefaultsToSharedAndInheritsDriver(t *testing.T) {
	cfg := Default()
	if cfg.LogDatabase != nil {
		t.Fatalf("default log database should be nil, got %#v", cfg.LogDatabase)
	}
	cfg.LogDatabase = &DatabaseConfig{DSN: " ./apirelay-logs.db "}
	cfg.Normalize()
	if cfg.LogDatabase == nil {
		t.Fatal("log database should remain configured")
	}
	if cfg.LogDatabase.Driver != "sqlite" || cfg.LogDatabase.DSN != "./apirelay-logs.db" {
		t.Fatalf("normalized log database = %#v", cfg.LogDatabase)
	}

	cfg.LogDatabase = &DatabaseConfig{Driver: "postgres", DSN: "   "}
	cfg.Normalize()
	if cfg.LogDatabase != nil {
		t.Fatalf("empty log database DSN should fall back to shared mode: %#v", cfg.LogDatabase)
	}
}

func TestLogDatabaseEnvironmentOverrides(t *testing.T) {
	t.Setenv("APIRELAY_LOG_DB_DRIVER", "POSTGRES")
	t.Setenv("APIRELAY_LOG_DB_DSN", " postgres://logs.example/apirelay ")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LogDatabase == nil {
		t.Fatal("environment should enable the log database")
	}
	if cfg.LogDatabase.Driver != "postgres" || cfg.LogDatabase.DSN != "postgres://logs.example/apirelay" {
		t.Fatalf("environment log database = %#v", cfg.LogDatabase)
	}
}

func TestLogRetentionDefaultsAreDisabled(t *testing.T) {
	cfg := Default()
	// 升级到本版本不应静默开始删除既有日志。
	if cfg.LogRetention.Enabled {
		t.Fatal("log retention must be disabled by default")
	}
	if cfg.LogRetention.Days != DefaultLogRetentionDays {
		t.Fatalf("days = %d", cfg.LogRetention.Days)
	}
	if cfg.LogRetention.PayloadDays != DefaultLogPayloadRetentionDays {
		t.Fatalf("payload days = %d", cfg.LogRetention.PayloadDays)
	}
	if cfg.LogRetention.BatchSize != DefaultLogRetentionBatchSize {
		t.Fatalf("batch size = %d", cfg.LogRetention.BatchSize)
	}
}

func TestNormalizeLogRetentionGuardsAgainstDestructiveValues(t *testing.T) {
	// Days <= 0 必须回退默认保留期，绝不能变成"删除全部"。
	got := normalizeLogRetention(LogRetentionConfig{Enabled: true, Days: 0})
	if got.Days != DefaultLogRetentionDays {
		t.Fatalf("zero days should fall back to default, got %d", got.Days)
	}
	got = normalizeLogRetention(LogRetentionConfig{Enabled: true, Days: -10})
	if got.Days != DefaultLogRetentionDays {
		t.Fatalf("negative days should fall back to default, got %d", got.Days)
	}

	// 载荷保留期不能超过摘要保留期（摘要删除会级联删除载荷）。
	got = normalizeLogRetention(LogRetentionConfig{Enabled: true, Days: 7, PayloadDays: 30})
	if got.PayloadDays != 7 {
		t.Fatalf("payload days = %d, want clamped to 7", got.PayloadDays)
	}

	// 未配置载荷保留期时跟随摘要。
	got = normalizeLogRetention(LogRetentionConfig{Enabled: true, Days: 14})
	if got.PayloadDays != 14 {
		t.Fatalf("payload days = %d, want 14", got.PayloadDays)
	}

	// 批量大小有上下界，避免长事务阻塞 SQLite 唯一写连接。
	got = normalizeLogRetention(LogRetentionConfig{Enabled: true, Days: 7, BatchSize: -1})
	if got.BatchSize != DefaultLogRetentionBatchSize {
		t.Fatalf("batch size = %d", got.BatchSize)
	}
	got = normalizeLogRetention(LogRetentionConfig{Enabled: true, Days: 7, BatchSize: 1 << 20})
	if got.BatchSize != maxLogRetentionBatchSize {
		t.Fatalf("batch size = %d, want clamped to %d", got.BatchSize, maxLogRetentionBatchSize)
	}

	got = normalizeLogRetention(LogRetentionConfig{Enabled: true, Days: 7, IntervalMinutes: 0})
	if got.IntervalMinutes != DefaultLogRetentionIntervalMinutes {
		t.Fatalf("interval = %d", got.IntervalMinutes)
	}
}

func TestLogRetentionEnvironmentOverrides(t *testing.T) {
	t.Setenv("APIRELAY_LOG_RETENTION_ENABLED", "true")
	t.Setenv("APIRELAY_LOG_RETENTION_DAYS", "14")
	t.Setenv("APIRELAY_LOG_RETENTION_PAYLOAD_DAYS", "3")
	t.Setenv("APIRELAY_LOG_RETENTION_INTERVAL_MINUTES", "15")
	t.Setenv("APIRELAY_LOG_RETENTION_BATCH_SIZE", "250")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	r := cfg.LogRetention
	if !r.Enabled || r.Days != 14 || r.PayloadDays != 3 || r.IntervalMinutes != 15 || r.BatchSize != 250 {
		t.Fatalf("log retention env overrides not applied: %#v", r)
	}
}
