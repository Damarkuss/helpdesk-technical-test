// Package config loads settings from environment variables. Nothing sensitive
// has a default: a missing DATABASE_URL or JWT_SECRET stops the server at startup.
package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port        string
	DatabaseURL string

	JWTSecret string
	JWTTTL    time.Duration

	// PasswordIterations is the PBKDF2 work factor (0 = library default).
	PasswordIterations int

	CORSOrigins []string

	// UrgentSLA is how long an urgent ticket may remain "open" before it is flagged overdue (0 disables).
	UrgentSLA time.Duration

	SeedEnabled  bool
	SeedPassword string
}

// Load reads the configuration. getenv is os.Getenv in production and a map lookup in tests.
func Load(getenv func(string) string) (Config, error) {
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	c := Config{
		Port:         orDefault(getenv("PORT"), "8080"),
		DatabaseURL:  getenv("DATABASE_URL"),
		JWTSecret:    getenv("JWT_SECRET"),
		SeedPassword: getenv("SEED_PASSWORD"),
	}

	if c.DatabaseURL == "" {
		fail("DATABASE_URL is required")
	}
	if len(c.JWTSecret) < 32 {
		fail("JWT_SECRET is required and must be at least 32 characters")
	}

	c.JWTTTL = duration(getenv, "JWT_TTL", 24*time.Hour, fail)
	c.UrgentSLA = duration(getenv, "URGENT_SLA", 4*time.Hour, fail)

	if v := getenv("PASSWORD_HASH_ITERATIONS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			fail("PASSWORD_HASH_ITERATIONS must be a positive integer")
		}
		c.PasswordIterations = n
	}

	if v := getenv("CORS_ORIGINS"); v != "" {
		for _, o := range strings.Split(v, ",") {
			if o = strings.TrimSpace(o); o != "" {
				c.CORSOrigins = append(c.CORSOrigins, o)
			}
		}
	}

	if v := getenv("SEED_ENABLED"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			fail("SEED_ENABLED must be true or false")
		}
		c.SeedEnabled = b
	}
	if c.SeedEnabled && len(c.SeedPassword) < 8 {
		fail("SEED_PASSWORD (at least 8 characters) is required when SEED_ENABLED=true")
	}

	return c, errors.Join(errs...)
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func duration(getenv func(string) string, key string, def time.Duration, fail func(string, ...any)) time.Duration {
	v := getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 {
		fail("%s must be a duration such as 30m or 24h", key)
		return def
	}
	return d
}
