//go:build integration

package tests

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"github.com/quixgit/greenops-reliabilix/backend/internal/app"
	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/application"
	clouddomain "github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/providers/azure"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/secretbox"
)

// The whole Azure path: the API seals the client secret, a worker (a separate container, as in production) opens it,
// reads costs from a fake Cost Management endpoint, ingests them as FOCUS records and the dashboard lists Azure.
func TestAzureServicePrincipalSyncEndToEnd(t *testing.T) {
	ctx := context.Background()
	owner, apiPool, workerPool := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_API_DSN"), pool(t, "TEST_WORKER_DSN")

	const (
		tenantGUID = "9c5fa158-8ee6-46f7-81a9-0021646e265e"
		clientGUID = "b8b97d8b-fec7-4b74-a232-b0b605ff0959"
		subGUID    = "ee210ee4-7656-46c4-9c17-bb47a985f7e0"
		secret     = "test-only-azure-client-secret-not-a-real-credential"
	)
	day := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -2)
	date := day.Format("20060102")

	var queries int
	cloud := http.NewServeMux()
	cloud.HandleFunc("/"+tenantGUID+"/oauth2/v2.0/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("client_secret") != secret {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `{"access_token":"t","expires_in":3600}`)
	})
	cloud.HandleFunc("/subscriptions/"+subGUID+"/providers/Microsoft.CostManagement/query", func(w http.ResponseWriter, r *http.Request) {
		queries++
		var body struct {
			Type    string
			Dataset struct{ Grouping []any }
		}
		_ = decode(r, &body)
		if len(body.Dataset.Grouping) == 0 { // the independent day total
			fmt.Fprintf(w, `{"properties":{"columns":[{"name":"Cost"},{"name":"UsageDate"},{"name":"Currency"}],"rows":[[100,%s,"USD"]]}}`, date)
			return
		}
		fmt.Fprintf(w, `{"properties":{"columns":[{"name":"Cost"},{"name":"UsageDate"},{"name":"ServiceName"},{"name":"ResourceLocation"},{"name":"Currency"}],"rows":[
			[60,%[1]s,"Virtual Machines","EU West","USD"],[30,%[1]s,"Storage","EU West","USD"],[6,%[1]s,"Bandwidth","","USD"],[4,%[1]s,"Bandwidth","Unassigned","USD"]]}}`, date)
	})
	az := httptest.NewServer(cloud)
	defer az.Close()

	// the same sealing key in both containers (a development key derived from the environment-less default)
	k := sha256.Sum256([]byte("reliabilix-development-notifications-key"))
	box, err := secretbox.New(k[:])
	if err != nil {
		t.Fatal(err)
	}
	h := azure.NoRedirects(http.DefaultClient)
	prov := &azure.Provider{HTTP: h, Authority: az.URL, Cost: &azure.CostAPI{HTTP: h, ARM: az.URL},
		OpenSecret: func(c clouddomain.Connection) (string, error) {
			return box.Open(c.SealedSecret, application.SecretAAD(c.TenantID, c.ExternalID))
		}}
	reg := clouddomain.NewRegistry(prov)

	cfg := devConfig(t.TempDir())
	recA, recW := &taskRec{}, &taskRec{}
	apiC, err := app.Build(ctx, cfg, quietLog(), apiPool, recA, app.WithProviders(reg))
	if err != nil {
		t.Fatal(err)
	}
	workerC, err := app.Build(ctx, cfg, quietLog(), workerPool, recW, app.WithProviders(reg))
	if err != nil {
		t.Fatal(err)
	}
	mux := asynq.NewServeMux()
	for _, j := range workerC.JobRegistrars() {
		j.RegisterJobs(mux)
	}
	srv := httptest.NewServer(app.NewRouter(cfg, quietLog(), apiC.Verifier, apiC.Resolver, apiPool.Ping, apiC.Modules()...))
	defer srv.Close()
	c := api{t: t, srv: srv}

	rid := fmt.Sprint(time.Now().UnixNano())
	alice := fmt.Sprintf("dev:azs-%s:-:-:azs-%s@example.com", rid, rid)
	tenant := str(c.must("POST", "/api/v1/onboarding", alice, `{"organization_name":"Azure Sync Org"}`, 201), "tenant_id")
	var project string
	if err := owner.QueryRow(ctx, `INSERT INTO projects.projects (tenant_id, name, functional_unit) VALUES ($1,'p','req') RETURNING id::text`, tenant).Scan(&project); err != nil {
		t.Fatal(err)
	}

	create := fmt.Sprintf(`{"project_id":%q,"provider":"azure","account_ref":%q,"credential_ref":"azcost://%s@%s","client_secret":%q}`, project, subGUID, tenantGUID, clientGUID, secret)
	conn := sub(c.must("POST", "/api/v1/cloud-accounts", alice, create, 201), "connection")
	id := str(conn, "id")
	if v := c.must("POST", "/api/v1/cloud-accounts/"+id+"/verify", alice, "", 200); str(v, "sync_status") != "healthy" {
		t.Fatalf("verify: %v", v)
	}
	pump(t, mux, recA, recW)

	var rows int
	var cost float64
	if err := owner.QueryRow(ctx, `SELECT count(*), COALESCE(sum(billed_cost),0)::float8 FROM usage.usage_records WHERE tenant_id=$1 AND provider='azure'`, tenant).Scan(&rows, &cost); err != nil {
		t.Fatal(err)
	}
	if rows != 3 || cost != 100 {
		runs := items(c.must("GET", "/api/v1/cloud-accounts/"+id+"/sync-runs", alice, "", 200))
		t.Fatalf("azure usage rows = %d, cost = %v; want 3 and 100 (the two global bandwidth locations merge). runs: %v", rows, cost, runs)
	}
	var global int
	_ = owner.QueryRow(ctx, `SELECT count(*) FROM usage.usage_records WHERE tenant_id=$1 AND provider='azure' AND region_id='global' AND billed_cost = 10`, tenant).Scan(&global)
	if global != 1 {
		t.Errorf("the unassigned bandwidth must be one record of 10, got %d", global)
	}

	runs := items(c.must("GET", "/api/v1/cloud-accounts/"+id+"/sync-runs", alice, "", 200))
	if len(runs) == 0 || str(runs[0].(map[string]any), "status") != "succeeded" {
		t.Errorf("sync runs: %v", runs)
	}
	if q := sub(runs[0].(map[string]any), "quality"); str(q, "status") != "ok" {
		t.Errorf("the detailed rows must reconcile with the independent total: %v", q)
	}

	// the dashboard lists Azure as a provider with cost
	found := false
	for _, p := range items(c.must("GET", "/api/v1/dashboard/providers", alice, "", 200)) {
		if strings.EqualFold(str(p.(map[string]any), "provider"), "azure") {
			found = true
		}
	}
	// carbon may not be calculated yet (no grid data for westeurope in a fresh database): cost is what must show
	svcs := items(c.must("GET", "/api/v1/dashboard/services", alice, "", 200))
	if len(svcs) < 3 {
		t.Errorf("dashboard services = %v (providers found azure=%v)", svcs, found)
	}
}

func decode(r *http.Request, v any) error { return json.NewDecoder(r.Body).Decode(v) }
