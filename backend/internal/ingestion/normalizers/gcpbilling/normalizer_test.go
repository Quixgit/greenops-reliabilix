package gcpbilling

import (
	"math"
	"testing"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

func norm(t *testing.T, rows string) []focus.Record {
	t.Helper()
	recs, err := Normalizer{}.Normalize(focus.RawBatch{Provider: "gcp", Source: "bigquery_export", Payload: []byte(`{"rows":[` + rows + `]}`)})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range recs {
		if err := r.Validate(); err != nil {
			t.Errorf("record fails FOCUS validation: %v (%+v)", err, r)
		}
	}
	return recs
}

func l(service, sku, region, unit, cost, credits, qty string) string {
	return `{"day":"2026-09-01","service":"` + service + `","sku":"` + sku + `","region":"` + region + `","currency":"usd","pricing_unit":"` + unit +
		`","cost":"` + cost + `","credits":"` + credits + `","quantity":"` + qty + `"}`
}

type key struct{ service, unit string }

func index(recs []focus.Record) (cost map[string]focus.Record, measured map[key]float64) {
	cost, measured = map[string]focus.Record{}, map[key]float64{}
	for _, r := range recs {
		if r.ConsumedQuantity == nil {
			cost[r.ServiceName+"|"+r.RegionID] = r
		} else {
			measured[key{r.ServiceName, *r.ConsumedUnit}] += *r.ConsumedQuantity
		}
	}
	return
}

func TestComputeEngineIsSplitIntoMeasuredAndUnmeasuredWorkloads(t *testing.T) {
	recs := norm(t, ""+
		l("Compute Engine", "N2 Instance Core running in Frankfurt", "europe-west3", "hour", "10", "-2", "100")+","+
		l("Compute Engine", "N2 Instance Ram running in Frankfurt", "europe-west3", "gibibyte hour", "5", "-1", "400")+","+
		l("Compute Engine", "Spot Preemptible N2 Instance Core running in Frankfurt", "europe-west3", "hour", "3", "0", "50")+","+
		l("Compute Engine", "Nvidia Tesla T4 GPU running in Frankfurt", "europe-west3", "hour", "20", "0", "10")+","+
		l("Compute Engine", "Balanced PD Capacity in Frankfurt", "europe-west3", "gibibyte month", "2", "0", "30")+","+
		l("Compute Engine", "Storage PD Snapshot in Frankfurt", "europe-west3", "gibibyte month", "1", "0", "5")+","+
		l("Compute Engine", "Network Internet Egress from Frankfurt to Americas", "europe-west3", "gibibyte", "4", "0", "40")+","+
		l("Compute Engine", "Licensing Fee for Windows Server", "europe-west3", "hour", "6", "0", "1"))
	cost, measured := index(recs)

	if c := cost["Compute Engine - Compute|europe-west3"]; c.BilledCost != 18 || c.EffectiveCost != 15 || c.ServiceCategory != "compute" || c.Currency != "USD" {
		t.Errorf("compute cost line must sum core+ram+spot and apply credits to the effective cost: %+v", c)
	}
	if m := measured[key{svcCompute, unitVCPUHours}]; m != 150 {
		t.Errorf("vCPU hours = %v, want 100 + 50 spot", m)
	}
	if m := measured[key{svcCompute, unitMemoryHour}]; math.Abs(m-400*gibToGB) > 1e-9 {
		t.Errorf("memory must be converted from GiB to GB: %v", m)
	}
	if m := measured[key{svcDisk, unitStorage}]; math.Abs(m-30*gibToGB) > 1e-9 {
		t.Errorf("disk capacity must be converted from GiB-months to GB-months: %v", m)
	}
	// GPUs, snapshots, network and licences are NOT measured and must keep their own cost lines, otherwise the
	// carbon engine would drop their cost-based estimate when it replaces the measured compute line
	for _, svc := range []string{svcAccel, svcComputeStor, svcComputeNet, svcComputeOth} {
		if _, ok := cost[svc+"|europe-west3"]; !ok {
			t.Errorf("%s needs its own cost line", svc)
		}
	}
	if cost[svcAccel+"|europe-west3"].BilledCost != 20 || cost[svcComputeNet+"|europe-west3"].ServiceCategory != "networking" {
		t.Errorf("accelerator/network lines wrong")
	}
}

func TestCloudStorageCapacityIsMeasuredButOperationsAreNot(t *testing.T) {
	recs := norm(t, ""+
		l("Cloud Storage", "Standard Storage Frankfurt", "europe-west3", "gibibyte month", "1", "0", "100")+","+
		l("Cloud Storage", "Nearline Storage Frankfurt", "europe-west3", "gibibyte month", "0.5", "0", "10")+","+
		l("Cloud Storage", "Standard Storage Class A Operations Frankfurt", "europe-west3", "count", "0.2", "0", "1000")+","+
		l("Cloud Storage", "Nearline Storage Retrieval Frankfurt", "europe-west3", "gibibyte", "0.3", "0", "10"))
	cost, measured := index(recs)
	if m := measured[key{svcGCSCapacity, unitStorage}]; math.Abs(m-110*gibToGB) > 1e-9 {
		t.Errorf("capacity SKUs must be measured, operations and retrievals must not: %v", m)
	}
	if _, ok := cost["Cloud Storage - Capacity|europe-west3"]; !ok {
		t.Error("the capacity cost line must carry the measured service name so it can be replaced")
	}
	if c := cost["Cloud Storage|europe-west3"]; math.Abs(c.BilledCost-0.5) > 1e-9 {
		t.Errorf("operations and retrieval keep a cost-based line: %+v", c)
	}
}

func TestOtherServicesGetCategoriesAndStayCostBased(t *testing.T) {
	recs := norm(t, ""+
		l("Cloud SQL", "Postgres vCPU", "europe-west3", "hour", "4", "0", "100")+","+
		l("BigQuery", "Analysis", "eu", "tebibyte", "3", "0", "1")+","+
		l("Cloud DNS", "Zone", "global", "month", "1", "0", "1")+","+
		l("Kubernetes Engine", "Autopilot Pod vCPU", "us-central1", "hour", "8", "0", "40")+","+
		l("Support", "Standard Support", "", "month", "100", "0", "1"))
	cat := map[string]string{}
	reg := map[string]string{}
	for _, r := range recs {
		if r.ConsumedQuantity != nil {
			t.Fatalf("nothing here is measurable: %+v", r)
		}
		cat[r.ServiceName], reg[r.ServiceName] = r.ServiceCategory, r.RegionID
	}
	want := map[string]string{"Cloud SQL": "database", "BigQuery": "database", "Cloud DNS": "networking", "Kubernetes Engine": "compute", "Support": "other"}
	for svc, c := range want {
		if cat[svc] != c {
			t.Errorf("%s category = %q, want %q", svc, cat[svc], c)
		}
	}
	if reg["Support"] != "global" || reg["Cloud DNS"] != "global" || reg["BigQuery"] != "eu" {
		t.Errorf("regions: %v", reg)
	}
}

func TestZeroLinesDropAndCreditsMayExceedCost(t *testing.T) {
	recs := norm(t, ""+
		l("Cloud DNS", "Zone", "global", "month", "0", "0", "1")+","+
		l("Cloud Run", "CPU", "europe-west1", "second", "1", "-3", "10"))
	if len(recs) != 1 || recs[0].ServiceName != "Cloud Run" || recs[0].BilledCost != 1 || recs[0].EffectiveCost != -2 {
		t.Fatalf("zero lines must not become rows; a credit larger than the cost gives a negative effective cost: %+v", recs)
	}
}

func TestUnexpectedUnitsAreNotMeasured(t *testing.T) {
	recs := norm(t, l("Compute Engine", "N2 Instance Core running in Frankfurt", "europe-west3", "minute", "1", "0", "100")+","+
		l("Compute Engine", "Sole Tenancy Instance Core running in Frankfurt", "europe-west3", "hour", "1", "0", "100"))
	for _, r := range recs {
		if r.ConsumedQuantity != nil {
			t.Fatalf("a changed unit or a sole-tenant line must never be scaled into a measurement: %+v", r)
		}
		if r.ServiceName == svcCompute {
			t.Fatalf("unmeasurable core lines must not share the measured service name: %+v", r)
		}
	}
}

func TestMalformedInput(t *testing.T) {
	for _, p := range []string{`nope`, `{"rows":[{"day":"x","service":"s","cost":"1","credits":"0"}]}`,
		`{"rows":[{"day":"2026-09-01","service":"s","cost":"abc","credits":"0"}]}`} {
		if _, err := (Normalizer{}).Normalize(focus.RawBatch{Payload: []byte(p)}); err == nil {
			t.Errorf("want an error for %s", p)
		}
	}
}
