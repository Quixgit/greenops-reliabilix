package awsexport

import (
	"testing"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

func run(t *testing.T, payload string) []focus.Record {
	t.Helper()
	recs, err := Normalizer{}.Normalize(focus.RawBatch{Provider: "aws", Source: "focus_export", Payload: []byte(payload)})
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

func TestCostAndMeasuredRecords(t *testing.T) {
	recs := run(t, `{
	 "cost":[
	  {"day":"2026-09-01","service":"Amazon Elastic Compute Cloud - Compute","category":"Compute","region":"eu-central-1","currency":"usd","billed":12.5,"effective":11},
	  {"day":"2026-09-01","service":"Amazon Elastic Block Store","category":"Storage","region":"eu-central-1","currency":"USD","billed":2,"effective":2},
	  {"day":"2026-09-01","service":"AWS Support","category":"Other","region":"","currency":"USD","billed":0,"effective":0},
	  {"day":"2026-09-01","service":"Amazon Route 53","category":"Networking","region":"global","currency":"USD","billed":1,"effective":1}],
	 "ec2_hours":[{"day":"2026-09-01","region":"eu-central-1","instance_type":"m5.large","hours":24}],
	 "s3_gb_months":[{"day":"2026-09-01","region":"eu-central-1","gb_months":3.5}]}`)
	var costs, measured int
	cats := map[string]string{}
	for _, r := range recs {
		if r.ConsumedQuantity == nil {
			costs++
			cats[r.ServiceName] = r.ServiceCategory
			if r.Currency != "USD" {
				t.Errorf("currency must be upper-cased: %q", r.Currency)
			}
		} else {
			measured++
			if r.BilledCost != 0 {
				t.Errorf("measured records carry no cost: %+v", r)
			}
		}
	}
	if costs != 3 || measured != 3 { // 3 charged lines; vCPU + memory for EC2 plus one S3 storage record
		t.Fatalf("costs=%d measured=%d (records: %+v)", costs, measured, recs)
	}
	if cats["Amazon Elastic Block Store"] != "storage" || cats["Amazon Elastic Compute Cloud - Compute"] != "compute" || cats["Amazon Route 53"] != "networking" {
		t.Errorf("FOCUS categories not mapped: %v", cats)
	}
	for _, r := range recs {
		if r.ServiceName == "Amazon Route 53" && r.RegionID != "global" {
			t.Errorf("global region not normalised: %+v", r)
		}
	}
}

func TestPartialInstanceShapesFallBackToCostBased(t *testing.T) {
	recs := run(t, `{"ec2_hours":[
	  {"day":"2026-09-01","region":"eu-central-1","instance_type":"m5.large","hours":24},
	  {"day":"2026-09-01","region":"eu-central-1","instance_type":"p4d.24xlarge","hours":24}]}`)
	if len(recs) != 0 {
		t.Fatalf("a region with an unknown shape must not be measured (it would undercount): %+v", recs)
	}
}

func TestMalformedPayload(t *testing.T) {
	for _, p := range []string{`nope`, `{"cost":[{"day":"x","service":"s","category":"c","region":"r","currency":"USD","billed":1}]}`} {
		if _, err := (Normalizer{}).Normalize(focus.RawBatch{Payload: []byte(p)}); err == nil {
			t.Errorf("want an error for %s", p)
		}
	}
}
