package config

import (
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

const goodSecret = "0123456789abcdef0123456789abcdef"

func TestLoadAppliesDefaults(t *testing.T) {
	c, err := Load(env(map[string]string{"DATABASE_URL": "postgres://x", "JWT_SECRET": goodSecret}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != "8080" || c.JWTTTL != 24*time.Hour || c.UrgentSLA != 4*time.Hour || c.SeedEnabled {
		t.Errorf("unexpected defaults: %+v", c)
	}
}

func TestLoadRefusesToStartWithoutSecrets(t *testing.T) {
	_, err := Load(env(map[string]string{"JWT_SECRET": "too-short"}))
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"DATABASE_URL", "JWT_SECRET"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}

func TestLoadSeedRequiresPasswordAndParsesCORS(t *testing.T) {
	base := map[string]string{"DATABASE_URL": "postgres://x", "JWT_SECRET": goodSecret, "SEED_ENABLED": "true"}
	if _, err := Load(env(base)); err == nil || !strings.Contains(err.Error(), "SEED_PASSWORD") {
		t.Errorf("want SEED_PASSWORD error, got %v", err)
	}

	base["SEED_PASSWORD"] = "demo-password"
	base["CORS_ORIGINS"] = "http://localhost:5173, http://localhost:3000 ,"
	c, err := Load(env(base))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.CORSOrigins) != 2 || c.CORSOrigins[1] != "http://localhost:3000" {
		t.Errorf("CORS origins = %q", c.CORSOrigins)
	}

	base["JWT_TTL"] = "soon"
	if _, err := Load(env(base)); err == nil {
		t.Error("invalid JWT_TTL should be rejected")
	}
}
