package focus

import (
	"testing"
	"time"
)

func valid() Record {
	t := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	return Record{Provider: "aws", Source: "cost_explorer", ServiceName: "Amazon EC2", ServiceCategory: "compute",
		RegionID: "eu-central-1", BilledCost: 1, EffectiveCost: 1, Currency: "USD", ChargePeriodStart: t, ChargePeriodEnd: t.Add(24 * time.Hour)}
}

func TestValidate(t *testing.T) {
	if err := valid().Validate(); err != nil {
		t.Fatal(err)
	}
	q := 1.0
	cases := map[string]func(*Record){
		"provider": func(r *Record) { r.Provider = "oracle" },
		"category": func(r *Record) { r.ServiceCategory = "ai" },
		"region":   func(r *Record) { r.RegionID = "" },
		"currency": func(r *Record) { r.Currency = "US" },
		"period":   func(r *Record) { r.ChargePeriodEnd = r.ChargePeriodStart },
		"qty/unit": func(r *Record) { r.ConsumedQuantity = &q },
	}
	for name, mut := range cases {
		r := valid()
		mut(&r)
		if r.Validate() == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	r := valid()
	r.BilledCost = -5 // credits are legal
	if err := r.Validate(); err != nil {
		t.Errorf("negative cost rejected: %v", err)
	}
}
