// Package config reads service configuration from environment variables (12-factor).
package config

import (
	"os"
	"strconv"
	"time"
)

// String returns the env var value or def when unset/empty.
func String(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Int returns the env var parsed as int or def.
func Int(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

// Duration returns the env var parsed as time.Duration or def.
func Duration(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

// IsDev reports whether the service runs in the local development environment.
func IsDev() bool { return String("ENV", "prod") == "dev" }
