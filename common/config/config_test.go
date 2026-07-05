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
