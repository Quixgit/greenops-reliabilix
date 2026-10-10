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

// A source that publishes monthly national averages fills the days that have no hourly reading, labelled as such, and
// the operator sees what each source has for the regions in use.
func TestMonthlyNationalFigureFillsDaysWithoutHourlyReadings(t *testing.T) {
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
	region := "zz-month-" + rid[len(rid)-6:]
	alice := fmt.Sprintf("dev:mon-%s:-:-:mon-%s@example.com", rid, rid)
	opSub := "opmon-" + rid
	opTok := fmt.Sprintf("dev:%s:-:-:opmon-%s@example.com", opSub, rid)
	c.must("POST", "/api/v1/onboarding", alice, `{"organization_name":"Monthly Test Org"}`, 201)
	if _, err := owner.Exec(ctx, `INSERT INTO platform.operators (auth_sub, email, role) VALUES ($1,$2,'viewer')`, opSub, "opmon-"+rid+"@example.com"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = owner.Exec(ctx, `DELETE FROM carbon.grid_intensity WHERE region_id = $1`, region)
		_, _ = owner.Exec(ctx, `DELETE FROM platform.operator_access_log WHERE operator_sub = $1`, opSub)
		_, _ = owner.Exec(ctx, `DELETE FROM platform.operators WHERE auth_sub = $1`, opSub)
	})

	monthStart := time.Now().UTC().AddDate(0, -1, 0)
	monthStart = time.Date(monthStart.Year(), monthStart.Month(), 1, 0, 0, 0, 0, time.UTC)
	if _, err := owner.Exec(ctx, `INSERT INTO carbon.grid_intensity (provider, region_id, ts, g_co2e_per_kwh, is_forecast, measure, resolution) VALUES ('ember',$1,$2,180,false,'average','monthly')`, region, monthStart); err != nil {
		t.Fatal(err)
	}
	from, to := monthStart, monthStart.AddDate(0, 0, 3)
	h := c.must("GET", "/api/v1/carbon/intensity?region="+region+"&from="+from.Format(time.DateOnly)+"&to="+to.Format(time.DateOnly), alice, "", 200)
	pts := h["points"].([]any)
	if len(pts) < 3 { // the period end date is inclusive
		t.Fatalf("points = %v, want a point for every day of the window", pts)
	}
	p := pts[0].(map[string]any)
	if p["source"] != "ember" || p["basis"] != "month_mean" || p["g_per_kwh"].(float64) != 180 {
		t.Errorf("point = %v", p)
	}

	// the operator's view: per region what each source has, and the cost-of-serving estimate for every organization
	c.must("GET", "/api/v1/operator/carbon/regions", opTok, "", 200)
	u := c.must("GET", "/api/v1/operator/usage?days=30", opTok, "", 200)
	if len(items(u)) == 0 || u["days"].(float64) != 30 {
		t.Errorf("usage = %v", u)
	}
	if code, _, _ := c.do("GET", "/api/v1/operator/usage?days=0", opTok, ""); code != 400 {
		t.Errorf("days=0 = %d, want 400", code)
	}
}

// Stored forecasts give a start time for a flexible job, for people and for CI pipelines; and the coverage function
// that drives the automatic loading of a region's past runs.
func TestScheduleAdviceFromStoredForecastsAndRegionCoverage(t *testing.T) {
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
	region := "zz-sched-" + rid[len(rid)-6:]
	alice := fmt.Sprintf("dev:sch-%s:-:-:sch-%s@example.com", rid, rid)
	c.must("POST", "/api/v1/onboarding", alice, `{"organization_name":"Schedule Test Org"}`, 201)
	t.Cleanup(func() { _, _ = owner.Exec(ctx, `DELETE FROM carbon.grid_intensity WHERE region_id = $1`, region) })

	now := time.Now().UTC().Truncate(time.Hour)
	for i, g := range []float64{400, 390, 380, 150, 120, 110, 300, 350, 360, 370} {
		if _, err := owner.Exec(ctx, `INSERT INTO carbon.grid_intensity (provider, region_id, ts, g_co2e_per_kwh, is_forecast, measure, resolution) VALUES ('electricitymaps',$1,$2,$3,true,'average','hourly')`,
			region, now.Add(time.Duration(i)*time.Hour), g); err != nil {
			t.Fatal(err)
		}
	}
	a := c.must("GET", "/api/v1/carbon/schedule?region="+region+"&duration_hours=2&within_hours=8", alice, "", 200)
	w := sub(a, "window")
	if a["source"] != "electricitymaps" || w == nil || w["average_g_per_kwh"].(float64) != 115 || w["saving_pct"].(float64) < 50 {
		t.Errorf("advice = %v", a)
	}
	c.must("GET", "/api/v1/carbon/schedule?region="+region+"&duration_hours=2&within_hours=1", alice, "", 400)
	c.must("GET", "/api/v1/carbon/schedule?region=zz-nothing-1&duration_hours=1", alice, "", 404)

	var n int
	if err := owner.QueryRow(ctx, `SELECT count(*) FROM carbon.region_coverage(30)`).Scan(&n); err != nil {
		t.Errorf("region coverage: %v", err)
	}
}
