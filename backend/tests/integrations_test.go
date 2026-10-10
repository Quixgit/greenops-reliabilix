//go:build integration

package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/integrations"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/secretbox"
)

func TestIntegrationSecretsAreSealedAndWriteOnly(t *testing.T) {
	ctx := context.Background()
	owner, apiPool := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_API_DSN")
	t.Cleanup(func() { _, _ = owner.Exec(ctx, `DELETE FROM platform.integration_values`) })
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	box, _ := secretbox.New(key)
	env := map[string]string{"smtp/host": "env.example.com"}
	s := &integrations.Store{Pool: apiPool, Box: box, Env: func(i, k string) string { return env[i+"/"+k] }, TTL: 1}

	secret := "super-secret-api-key-123456"
	err := s.Set(ctx, "operator-sub", "electricitymaps", map[string]*string{"api_key": &secret})
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	var sealedFlag bool
	if err := owner.QueryRow(ctx, `SELECT value, secret FROM platform.integration_values WHERE integration='electricitymaps' AND key='api_key'`).Scan(&stored, &sealedFlag); err != nil {
		t.Fatal(err)
	}
	if !sealedFlag || strings.Contains(stored, "super-secret") || !strings.HasPrefix(stored, "v1:") {
		t.Fatalf("the database must hold ciphertext only: %q", stored)
	}
	if got := s.Value(ctx, "electricitymaps", "api_key"); got != secret {
		t.Fatalf("runtime value %q", got)
	}
	sts, err := s.Statuses(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range sts {
		for _, f := range st.Fields {
			if f.Secret && (f.Value != "" || strings.Contains(f.Hint, "super-secret")) {
				t.Errorf("%s.%s leaks: %+v", st.ID, f.Key, f)
			}
		}
	}
	// A non-secret value wins over the environment; reset returns to the environment.
	host := "console.example.com"
	if err := s.Set(ctx, "operator-sub", "smtp", map[string]*string{"host": &host}); err != nil {
		t.Fatal(err)
	}
	if v, src := s.Resolve(ctx, "smtp", "host"); v != host || src != integrations.FromConsole {
		t.Fatalf("%q %s", v, src)
	}
	if err := s.Set(ctx, "operator-sub", "smtp", map[string]*string{"host": nil}); err != nil {
		t.Fatal(err)
	}
	if v, src := s.Resolve(ctx, "smtp", "host"); v != "env.example.com" || src != integrations.FromEnvironment {
		t.Fatalf("after reset: %q %s", v, src)
	}
	// A bad value changes nothing; an unknown field is refused.
	bad := "not a port"
	if err := s.Set(ctx, "operator-sub", "smtp", map[string]*string{"port": &bad}); err == nil {
		t.Fatal("an invalid port must be refused")
	}
	if err := s.Set(ctx, "operator-sub", "smtp", map[string]*string{"nope": &host}); err == nil {
		t.Fatal("an unknown field must be refused")
	}
	// The application role has no direct write access to the table: only the audited function.
	if _, err := apiPool.Exec(ctx, `INSERT INTO platform.integration_values (integration, key, value, secret, updated_by) VALUES ('smtp','host','x',false,'x')`); err == nil {
		t.Fatal("the API role must not be able to write the table directly")
	}
}
