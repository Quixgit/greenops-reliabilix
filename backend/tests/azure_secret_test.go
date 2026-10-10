//go:build integration

package tests

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/app"
)

// The client secret of an Azure service principal is the one customer secret the platform keeps. It must be stored
// sealed, never come back from the API, never be part of the data export, and the connection must not open with
// another organization's key material.
func TestAzureClientSecretIsSealedAndNeverExposed(t *testing.T) {
	ctx := context.Background()
	owner, apiPool := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_API_DSN")
	cfg := devConfig(t.TempDir())
	apiC, err := app.Build(ctx, cfg, quietLog(), apiPool, &taskRec{})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.NewRouter(cfg, quietLog(), apiC.Verifier, apiC.Resolver, apiPool.Ping, apiC.Modules()...))
	defer srv.Close()
	c := api{t: t, srv: srv}

	rid := fmt.Sprint(time.Now().UnixNano())
	alice := fmt.Sprintf("dev:az-%s:-:-:az-%s@example.com", rid, rid)
	tenant := str(c.must("POST", "/api/v1/onboarding", alice, `{"organization_name":"Azure Org"}`, 201), "tenant_id")
	var project string
	if err := owner.QueryRow(ctx, `INSERT INTO projects.projects (tenant_id, name, functional_unit) VALUES ($1,'p','req') RETURNING id::text`, tenant).Scan(&project); err != nil {
		t.Fatal(err)
	}
	const secret = "test-only-azure-client-secret-not-a-real-credential"
	body := fmt.Sprintf(`{"project_id":%q,"provider":"azure","account_ref":"11111111-2222-3333-4444-555555555555",
		"credential_ref":"azcost://9c5fa158-8ee6-46f7-81a9-0021646e265e@b8b97d8b-fec7-4b74-a232-b0b605ff0959","client_secret":%q}`, project, secret)
	code, _, raw := c.do("POST", "/api/v1/cloud-accounts", alice, body)
	if code != 201 {
		t.Fatalf("create = %d: %s", code, raw)
	}
	if strings.Contains(string(raw), secret) || strings.Contains(string(raw), "v1:") {
		t.Fatalf("the create response carries the secret or its sealed form: %s", raw)
	}
	for _, path := range []string{"/api/v1/cloud-accounts"} {
		if _, _, raw := c.do("GET", path, alice, ""); strings.Contains(string(raw), secret) || strings.Contains(string(raw), "v1:") {
			t.Fatalf("%s exposes the secret: %s", path, raw)
		}
	}

	var stored *string
	if err := owner.QueryRow(ctx, `SELECT sealed_secret FROM cloudaccounts.connections WHERE tenant_id = $1`, tenant).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == nil || !strings.HasPrefix(*stored, "v1:") || strings.Contains(*stored, secret) {
		t.Fatalf("the secret must be stored sealed: %v", stored)
	}

	// a secret on a kind that takes none, and a service principal without one
	bad := fmt.Sprintf(`{"project_id":%q,"provider":"aws","account_ref":"123456789012","credential_ref":"arn:aws:iam::123456789012:role/ReliabilixR","client_secret":%q}`, project, secret)
	c.must("POST", "/api/v1/cloud-accounts", alice, bad, 422)
	noSecret := fmt.Sprintf(`{"project_id":%q,"provider":"azure","account_ref":"22222222-2222-3333-4444-555555555555",
		"credential_ref":"azcost://9c5fa158-8ee6-46f7-81a9-0021646e265e@b8b97d8b-fec7-4b74-a232-b0b605ff0959"}`, project)
	c.must("POST", "/api/v1/cloud-accounts", alice, noSecret, 422)
}
