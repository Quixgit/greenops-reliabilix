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

// An AWS access key and a Google service account key are customer secrets like the Azure client secret (ADR-0044):
// sealed in the database, never in a response or a listing, accepted only in a checked shape.
func TestAWSAccessKeyAndGoogleKeyAreSealedAndNeverExposed(t *testing.T) {
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
	alice := fmt.Sprintf("dev:ok-%s:-:-:ok-%s@example.com", rid, rid)
	tenant := str(c.must("POST", "/api/v1/onboarding", alice, `{"organization_name":"Own Credential Org"}`, 201), "tenant_id")
	var project string
	if err := owner.QueryRow(ctx, `INSERT INTO projects.projects (tenant_id, name, functional_unit) VALUES ($1,'p','req') RETURNING id::text`, tenant).Scan(&project); err != nil {
		t.Fatal(err)
	}
	const awsSecret = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
	const privateKey = "MIIEvQIBADANBgkqhkiG9w0BAQEFAASC-unique-private-key-material"
	googleKey := fmt.Sprintf(`{"type":"service_account","project_id":"acme-prod-123","private_key":"-----BEGIN PRIVATE KEY-----\n%s\n-----END PRIVATE KEY-----\n","client_email":"reliabilix@acme-prod-123.iam.gserviceaccount.com","token_uri":"https://oauth2.googleapis.com/token"}`, privateKey)
	jsonString := func(s string) string { return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) }

	awsBody := fmt.Sprintf(`{"project_id":%q,"provider":"aws","account_ref":"123456789012","credential_ref":"awskey://AKIAIOSFODNN7EXAMPLE","client_secret":%q}`, project, awsSecret)
	gcpBody := fmt.Sprintf(`{"project_id":%q,"provider":"gcp","account_ref":"acme-prod-123",
		"credential_ref":"bq://acme-prod-123/billing/gcp_billing_export_v1_AAAAAA_BBBBBB_CCCCCC","client_secret":"%s"}`, project, jsonString(googleKey))
	for name, body := range map[string]string{"aws": awsBody, "gcp": gcpBody} {
		code, _, raw := c.do("POST", "/api/v1/cloud-accounts", alice, body)
		if code != 201 {
			t.Fatalf("%s create = %d: %s", name, code, raw)
		}
		for _, leak := range []string{awsSecret, privateKey, "v1:"} {
			if strings.Contains(string(raw), leak) {
				t.Fatalf("%s: the create response carries a secret or its sealed form (%q): %s", name, leak, raw)
			}
		}
	}
	_, _, raw := c.do("GET", "/api/v1/cloud-accounts", alice, "")
	for _, leak := range []string{awsSecret, privateKey, "v1:"} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("the listing exposes a secret (%q): %s", leak, raw)
		}
	}
	if !strings.Contains(string(raw), `"access":"access_key"`) || !strings.Contains(string(raw), `"access":"service_account_key"`) {
		t.Errorf("the listing must say how each connection authenticates: %s", raw)
	}

	rows, err := owner.Query(ctx, `SELECT provider, sealed_secret FROM cloudaccounts.connections WHERE tenant_id = $1`, tenant)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var provider string
		var sealed *string
		if err := rows.Scan(&provider, &sealed); err != nil {
			t.Fatal(err)
		}
		n++
		if sealed == nil || !strings.HasPrefix(*sealed, "v1:") || strings.Contains(*sealed, awsSecret) || strings.Contains(*sealed, privateKey) {
			t.Errorf("%s: the secret must be stored sealed: %v", provider, sealed)
		}
	}
	if n != 2 {
		t.Fatalf("connections = %d, want 2", n)
	}

	// refused: a user credentials file, a session key, no secret for an access key
	c.must("POST", "/api/v1/cloud-accounts", alice, fmt.Sprintf(`{"project_id":%q,"provider":"gcp","account_ref":"acme-prod-124",
		"credential_ref":"bq://acme-prod-124/billing/gcp_billing_export_v1_AAAAAA_BBBBBB_CCCCCC","client_secret":"{\"type\":\"authorized_user\"}"}`, project), 422)
	c.must("POST", "/api/v1/cloud-accounts", alice, fmt.Sprintf(`{"project_id":%q,"provider":"aws","account_ref":"123456789013","credential_ref":"awskey://ASIAIOSFODNN7EXAMPLE","client_secret":%q}`, project, awsSecret), 422)
	c.must("POST", "/api/v1/cloud-accounts", alice, fmt.Sprintf(`{"project_id":%q,"provider":"aws","account_ref":"123456789014","credential_ref":"awskey://AKIAIOSFODNN7EXAMPLF"}`, project), 422)
}
