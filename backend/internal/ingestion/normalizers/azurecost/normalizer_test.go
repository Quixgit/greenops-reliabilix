package azurecost

import (
	"testing"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

func TestNormalize(t *testing.T) {
	raw := focus.RawBatch{Provider: "azure", Source: "cost_management_api", Payload: []byte(`{"rows":[
	 {"day":"2026-10-01","service":"Virtual Machines","location":"EU West","currency":"eur","billed":12.5,"effective":10},
	 {"day":"2026-10-01","service":"Bandwidth","location":"","currency":"EUR","billed":0,"effective":0},
	 {"day":"2026-10-02","service":"","location":"US East 2","currency":"USD","billed":3,"effective":3},
	 {"day":"2026-10-02","service":"Azure Cosmos DB","location":"Unassigned","currency":"USD","billed":1,"effective":1}]}`)}
	recs, err := Normalizer{}.Normalize(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 3 {
		t.Fatalf("got %d records, want 3 (the zero line is dropped): %+v", len(recs), recs)
	}
	vm := recs[0]
	if vm.Provider != "azure" || vm.Source != "cost_management_api" || vm.ServiceName != "Virtual Machines" || vm.ServiceCategory != "compute" ||
		vm.RegionID != "westeurope" || vm.BilledCost != 12.5 || vm.EffectiveCost != 10 || vm.Currency != "EUR" {
		t.Errorf("vm: %+v", vm)
	}
	if recs[1].ServiceName != "Other" || recs[1].RegionID != "eastus2" || recs[1].ServiceCategory != "other" {
		t.Errorf("unnamed service: %+v", recs[1])
	}
	if recs[2].RegionID != "global" || recs[2].ServiceCategory != "database" {
		t.Errorf("cosmos: %+v", recs[2])
	}
	for _, r := range recs {
		if r.ConsumedQuantity != nil {
			t.Errorf("the Query API measures nothing, a quantity was invented: %+v", r)
		}
	}
}

func TestRegion(t *testing.T) {
	for in, want := range map[string]string{
		"EU West": "westeurope", "EU North": "northeurope", "US East": "eastus", "US East 2": "eastus2", "US Central": "centralus",
		"US North Central": "northcentralus", "US West 3": "westus3", "UK South": "uksouth", "Germany West Central": "germanywestcentral",
		"Asia Pacific Southeast": "southeastasia", "AU East": "australiaeast", "westeurope": "westeurope", "": "global",
		"Unassigned": "global", "Zone 1": "global", "Global": "global",
	} {
		if got := Region(in); got != want {
			t.Errorf("Region(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBadDay(t *testing.T) {
	if _, err := (Normalizer{}).Normalize(focus.RawBatch{Payload: []byte(`{"rows":[{"day":"x","billed":1}]}`)}); err == nil {
		t.Error("a bad day must fail the batch")
	}
}
