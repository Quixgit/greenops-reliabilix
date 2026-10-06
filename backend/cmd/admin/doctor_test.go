package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/config"
)

func TestSanitizeNeverLeaksCredentials(t *testing.T) {
	for _, in := range []string{
		`dial postgres://greenops_api:S3cr3t@db.internal:5432/greenops: connection refused`,
		`Get "https://user:tok@host/path": timeout`,
		`redis://:hunter2@cache:6379 refused`,
	} {
		got := sanitize(errors.New(in))
		for _, secret := range []string{"S3cr3t", "tok@", "hunter2"} {
			if strings.Contains(got, secret) {
				t.Errorf("credential %q leaked: %s", secret, got)
			}
		}
	}
	if long := sanitize(errors.New(strings.Repeat("x", 1000))); len(long) > 310 {
		t.Errorf("message not truncated: %d", len(long))
	}
}

func levelOf(fs []finding, name string) (level, bool) {
	for _, f := range fs {
		if f.name == name {
			return f.lvl, true
		}
	}
	return 0, false
}

func TestCheckConfig(t *testing.T) {
	if l, found := levelOf(checkConfig(config.Config{Env: "dev", HTTPAddr: ":8080"}, nil), "dev-exposure"); !found || l != lvlWarn {
		t.Error("dev mode on all interfaces must warn")
	}
	if _, found := levelOf(checkConfig(config.Config{Env: "dev", HTTPAddr: "127.0.0.1:8080"}, nil), "dev-exposure"); found {
		t.Error("loopback dev must not warn")
	}
	if l, _ := levelOf(checkConfig(config.Config{Env: "prod", CORSOrigins: []string{"http://x.com"}}, nil), "cors"); l != lvlWarn {
		t.Error("insecure CORS origin must warn")
	}
	if l, _ := levelOf(checkConfig(config.Config{Env: "prod"}, errors.New("config: DATABASE_URL is required")), "config"); l != lvlFail {
		t.Error("config error must fail")
	}
	if l, _ := levelOf(checkConfig(config.Config{Env: "dev", ElectricityMapsZoneOverrides: "bad"}, nil), "zone-overrides"); l != lvlFail {
		t.Error("bad overrides must fail")
	}
}

func TestCheckDatabaseFlagsInsecureProductionDSN(t *testing.T) {
	cfg := config.Config{Env: "prod", DatabaseURL: "postgres://greenops_api:dev-only@127.0.0.1:1/greenops?sslmode=disable"}
	fs := checkDatabase(context.Background(), cfg)
	if l, _ := levelOf(fs, "db-password"); l != lvlFail {
		t.Error("placeholder password must fail in prod")
	}
	if l, _ := levelOf(fs, "db-tls"); l != lvlFail {
		t.Error("sslmode=disable must fail in prod")
	}
	for _, f := range fs {
		if strings.Contains(f.msg+f.hint, "dev-only") {
			t.Errorf("password leaked in output: %s", f.msg)
		}
	}
	if fs := checkDatabase(context.Background(), config.Config{Env: "dev"}); fs[0].lvl != lvlFail {
		t.Error("missing DATABASE_URL must fail")
	}
}

func TestCheckStorageLocalAndAuth0(t *testing.T) {
	fs := checkStorage(context.Background(), config.Config{Env: "dev", LocalStorageDir: t.TempDir()}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if fs[0].lvl != lvlOK {
		t.Errorf("local storage probe: %+v", fs[0])
	}
	if fs := checkStorage(context.Background(), config.Config{Env: "prod"}, slog.New(slog.NewTextHandler(io.Discard, nil))); fs[0].lvl != lvlFail {
		t.Error("prod without S3_BUCKET must fail")
	}
	if fs := checkAuth0(context.Background(), config.Config{Env: "prod", Auth0Domain: "evil.com/../x", Auth0Audience: "a"}); fs[0].lvl != lvlFail {
		t.Error("malformed AUTH0_DOMAIN must fail without any request being made")
	}
	if fs := checkAuth0(context.Background(), config.Config{Env: "dev"}); fs[0].lvl != lvlInfo {
		t.Error("dev skips auth0")
	}
}
