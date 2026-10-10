//go:build integration

package tests

import (
	"context"
	"fmt"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/app"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
)

// Several grid sources: the history takes one trusted source per day (never a blend), shows the others beside it, and
// follows the platform source policy when it changes.
func TestIntensityHistoryFollowsTheSourcePolicy(t *testing.T) {
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
	region := "zz-test-" + rid[len(rid)-6:]
	alice := fmt.Sprintf("dev:int-%s:-:-:int-%s@example.com", rid, rid)
	c.must("POST", "/api/v1/onboarding", alice, `{"organization_name":"Intensity Test Org"}`, 201)
	t.Cleanup(func() {
		_, _ = owner.Exec(ctx, `DELETE FROM carbon.grid_intensity WHERE region_id = $1`, region)
		_, _ = owner.Exec(ctx, `DELETE FROM settings.platform_values WHERE key = 'carbon.provider_priority'`)
	})

	// the SQL default of the shared priority function equals the catalog default of the setting
	var sqlDefault []string
	_, _ = owner.Exec(ctx, `DELETE FROM settings.platform_values WHERE key = 'carbon.provider_priority'`)
	if err := owner.QueryRow(ctx, `SELECT carbon.provider_priority()`).Scan(&sqlDefault); err != nil {
		t.Fatal(err)
	}
	def, _ := settings.Default().Lookup(settings.KeyCarbonPriority)
	if want, _ := def.Default.([]string); !reflect.DeepEqual(sqlDefault, want) {
		t.Fatalf("carbon.provider_priority() default = %v, catalog default = %v", sqlDefault, want)
	}

	day := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -3)
	for h := 0; h < 24; h++ {
		ts := day.Add(time.Duration(h) * time.Hour)
		for _, r := range []struct {
			provider string
			g        float64
		}{{"electricitymaps", 300}, {"uk_carbon_intensity", 100}} {
			if _, err := owner.Exec(ctx, `INSERT INTO carbon.grid_intensity (provider, region_id, ts, g_co2e_per_kwh, is_forecast, measure) VALUES ($1,$2,$3,$4,false,'average')`, r.provider, region, ts, r.g); err != nil {
				t.Fatal(err)
			}
		}
	}
	// a marginal reading must never be used for the average history
	if _, err := owner.Exec(ctx, `INSERT INTO carbon.grid_intensity (provider, region_id, ts, g_co2e_per_kwh, is_forecast, measure) VALUES ('watttime',$1,$2,900,false,'marginal')`, region, day); err != nil {
		t.Fatal(err)
	}

	read := func() map[string]any {
		return c.must("GET", "/api/v1/carbon/intensity?region="+region+"&from="+day.Format(time.DateOnly)+"&to="+day.AddDate(0, 0, 2).Format(time.DateOnly), alice, "", 200)
	}
	first := func(m map[string]any) map[string]any {
		pts := m["points"].([]any)
		if len(pts) != 1 {
			t.Fatalf("points = %v, want exactly the one day with readings", pts)
		}
		return pts[0].(map[string]any)
	}
	p := first(read())
	if p["source"] != "electricitymaps" || p["g_per_kwh"].(float64) != 300 || p["basis"] != "day_mean" || p["readings"].(float64) != 24 {
		t.Errorf("default policy point = %v", p)
	}
	others := p["others"].([]any)
	if len(others) != 1 || others[0].(map[string]any)["source"] != "uk_carbon_intensity" || others[0].(map[string]any)["g_per_kwh"].(float64) != 100 {
		t.Errorf("the other source must be shown beside, not blended: %v", others)
	}

	// the platform policy changes: the other source is now trusted first
	if _, err := owner.Exec(ctx, `INSERT INTO settings.platform_values (key, value, updated_by) VALUES ('carbon.provider_priority', '["uk_carbon_intensity","electricitymaps"]', 'test')
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`); err != nil {
		t.Fatal(err)
	}
	// the settings engine caches values for a few seconds
	deadline := time.Now().Add(15 * time.Second)
	for {
		p = first(read())
		if p["source"] == "uk_carbon_intensity" || time.Now().After(deadline) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if p["source"] != "uk_carbon_intensity" || p["g_per_kwh"].(float64) != 100 {
		t.Errorf("after the policy change point = %v", p)
	}
}
