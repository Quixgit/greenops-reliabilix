package azureexport

import (
	"testing"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

const payloadJSON = `{
 "cost":[
  {"day":"2026-10-01","service":"Virtual Machines - Compute","category":"Compute","region":"westeurope","currency":"eur","billed":15,"effective":12},
  {"day":"2026-10-01","service":"Virtual Machines - Other","category":"Compute","region":"","currency":"EUR","billed":100,"effective":100},
  {"day":"2026-10-01","service":"Bandwidth","category":"Networking","region":"Zone 1","currency":"EUR","billed":0,"effective":0},
  {"day":"2026-10-02","service":"Storage - Capacity","category":"Storage","region":"northeurope","currency":"EUR","billed":6,"effective":6},
  {"day":"2026-10-02","service":"Key Vault","category":"Security","region":"northeurope","currency":"EUR","billed":1,"effective":1}
 ],
 "vm_vcpu_hours":[{"day":"2026-10-01","region":"westeurope","value":192}],
 "storage_gb_months":[{"day":"2026-10-02","region":"northeurope","value":300},{"day":"2026-10-03","region":"northeurope","value":0}]
}`

func TestNormalize(t *testing.T) {
	recs, err := Normalizer{}.Normalize(focus.RawBatch{Provider: "azure", Source: "focus_export", Payload: []byte(payloadJSON)})
	if err != nil {
		t.Fatal(err)
	}
	var cost, measured int
	for _, r := range recs {
		if r.Provider != "azure" || r.Source != "focus_export" {
			t.Fatalf("provenance: %+v", r)
		}
		if r.ConsumedQuantity != nil {
			measured++
			if r.BilledCost != 0 || r.EffectiveCost != 0 {
				t.Errorf("a measured record must carry no cost, or every total doubles: %+v", r)
			}
			continue
		}
		cost++
	}
	if cost != 4 || measured != 2 { // the zero-cost Bandwidth line and the zero-quantity row are dropped
		t.Fatalf("cost=%d measured=%d: %+v", cost, measured, recs)
	}
	for _, r := range recs {
		switch {
		case r.ServiceName == "Virtual Machines - Compute" && r.ConsumedQuantity != nil:
			if *r.ConsumedUnit != "vCPU-Hrs" || *r.ConsumedQuantity != 192 || r.RegionID != "westeurope" || r.ServiceCategory != "compute" {
				t.Errorf("vCPU record: %+v", r)
			}
		case r.ServiceName == "Storage - Capacity" && r.ConsumedQuantity != nil:
			if *r.ConsumedUnit != "GB-Mo" || *r.ConsumedQuantity != 300 {
				t.Errorf("storage record: %+v", r)
			}
		case r.ServiceName == "Virtual Machines - Compute":
			if r.Currency != "EUR" || r.BilledCost != 15 || r.EffectiveCost != 12 || r.ServiceCategory != "compute" {
				t.Errorf("cost line: %+v", r)
			}
		case r.ServiceName == "Virtual Machines - Other":
			if r.RegionID != "global" {
				t.Errorf("a purchase has no region to attribute carbon to: %+v", r)
			}
		case r.ServiceName == "Key Vault":
			if r.ServiceCategory != "other" {
				t.Errorf("category: %+v", r)
			}
		}
	}
}

func TestBadDayIsAnError(t *testing.T) {
	_, err := Normalizer{}.Normalize(focus.RawBatch{Payload: []byte(`{"cost":[{"day":"yesterday","billed":1}]}`)})
	if err == nil {
		t.Error("a bad day must fail the batch, not be guessed")
	}
}

func TestRegion(t *testing.T) {
	for in, want := range map[string]string{"WestEurope": "westeurope", "": "global", "Global": "global", "Zone 2": "global", "Unassigned": "global", "East US 2": "eastus2"} {
		if got := region(in); got != want {
			t.Errorf("region(%q) = %q, want %q", in, got, want)
		}
	}
}
