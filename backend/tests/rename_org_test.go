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

// An administrator renames the organization; the change is audited, and a bad name changes nothing.
func TestRenameOrganization(t *testing.T) {
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
	alice := fmt.Sprintf("dev:rn-%s:-:-:rn-%s@example.com", rid, rid)
	tenant := str(c.must("POST", "/api/v1/onboarding", alice, `{"organization_name":"Before Name"}`, 201), "tenant_id")

	c.must("PATCH", "/api/v1/tenant", alice, `{"name":"  After Name  "}`, 200)
	var name string
	if err := owner.QueryRow(ctx, `SELECT name FROM tenants.organizations WHERE id=$1`, tenant).Scan(&name); err != nil || name != "After Name" {
		t.Fatalf("name = %q (%v)", name, err)
	}
	var n int
	_ = owner.QueryRow(ctx, `SELECT count(*) FROM audit.audit_logs WHERE tenant_id=$1 AND action='organization.renamed'`, tenant).Scan(&n)
	if n != 1 {
		t.Errorf("rename must be audited once: %d", n)
	}
	c.must("PATCH", "/api/v1/tenant", alice, `{"name":"x"}`, 422)
	c.must("PATCH", "/api/v1/tenant", alice, fmt.Sprintf(`{"name":%q}`, strings.Repeat("y", 101)), 422)
	_ = owner.QueryRow(ctx, `SELECT name FROM tenants.organizations WHERE id=$1`, tenant).Scan(&name)
	if name != "After Name" {
		t.Errorf("a rejected name must change nothing: %q", name)
	}
	// a viewer may not rename
	viewer := fmt.Sprintf("dev:rv-%s:%s:viewer:rv-%s@example.com", rid, tenant, rid)
	c.must("PATCH", "/api/v1/tenant", viewer, `{"name":"Hacked"}`, 403)
}
