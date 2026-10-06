package awsceusage

import (
	"testing"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

func norm(t *testing.T, payload string) []focus.Record {
	t.Helper()
	recs, err := Normalizer{}.Normalize(focus.RawBatch{Provider: "aws", Source: "cost_explorer_usage", Payload: []byte(payload)})
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

func TestEC2HoursBecomeVCPUAndMemoryRecords(t *testing.T) {
	recs := norm(t, `{"EC2Hours":[{"TimePeriod":{"Start":"2026-09-01","End":"2026-09-02"},"Estimated":false,"Groups":[
	  {"Keys":["eu-central-1","m5.large"],"Metrics":{"UsageQuantity":{"Amount":"48","Unit":"Hrs"}}}]}]}`)
	if len(recs) != 2 {
		t.Fatalf("got %d records, want a vCPU and a memory record", len(recs))
	}
	got := map[string]float64{}
	for _, r := range recs {
		got[*r.ConsumedUnit] = *r.ConsumedQuantity
		if r.BilledCost != 0 || r.EffectiveCost != 0 || r.ServiceName != ec2Service || r.ServiceCategory != "compute" || r.ResourceID != "instance-type/m5.large" {
			t.Errorf("record must be a zero-cost measurement of the EC2 service: %+v", r)
		}
		if err := r.Validate(); err != nil {
			t.Errorf("record must pass FOCUS validation: %v", err)
		}
	}
	if got["vCPU-Hrs"] != 96 || got["GB-Hrs"] != 384 { // 48h * 2 vCPU, 48h * 8 GB
		t.Errorf("quantities wrong: %v", got)
	}
}

func TestUnknownInstanceTypeDropsTheWholeRegionDay(t *testing.T) {
	recs := norm(t, `{"EC2Hours":[{"TimePeriod":{"Start":"2026-09-01","End":"2026-09-02"},"Groups":[
	  {"Keys":["eu-central-1","m5.large"],"Metrics":{"UsageQuantity":{"Amount":"24","Unit":"Hrs"}}},
	  {"Keys":["eu-central-1","p4d.24xlarge"],"Metrics":{"UsageQuantity":{"Amount":"24","Unit":"Hrs"}}},
	  {"Keys":["us-east-1","c7g.large"],"Metrics":{"UsageQuantity":{"Amount":"24","Unit":"Hrs"}}}]}]}`)
	for _, r := range recs {
		if r.RegionID == "eu-central-1" {
			t.Fatalf("a partial region-day must not be emitted (it would undercount): %+v", r)
		}
	}
	if len(recs) != 2 {
		t.Fatalf("the complete region must still be measured, got %d records", len(recs))
	}
}

func TestEstimatedPeriodsAndForeignUnitsAreSkipped(t *testing.T) {
	recs := norm(t, `{"EC2Hours":[
	 {"TimePeriod":{"Start":"2026-09-01","End":"2026-09-02"},"Estimated":true,"Groups":[
	   {"Keys":["eu-central-1","m5.large"],"Metrics":{"UsageQuantity":{"Amount":"24","Unit":"Hrs"}}}]},
	 {"TimePeriod":{"Start":"2026-09-02","End":"2026-09-03"},"Groups":[
	   {"Keys":["eu-central-1","m5.large"],"Metrics":{"UsageQuantity":{"Amount":"24","Unit":"Minutes"}}}]}],
	 "S3Storage":[{"TimePeriod":{"Start":"2026-09-02","End":"2026-09-03"},"Groups":[
	   {"Keys":["us-east-1"],"Metrics":{"UsageQuantity":{"Amount":"0","Unit":"GB-Mo"}}}]}]}`)
	if len(recs) != 0 {
		t.Fatalf("got %d records, want none", len(recs))
	}
}

func TestS3Storage(t *testing.T) {
	recs := norm(t, `{"S3Storage":[{"TimePeriod":{"Start":"2026-09-01","End":"2026-09-02"},"Groups":[
	  {"Keys":["us-east-1"],"Metrics":{"UsageQuantity":{"Amount":"31.5","Unit":"GB-Mo"}}}]}]}`)
	if len(recs) != 1 || *recs[0].ConsumedUnit != "GB-Mo" || *recs[0].ConsumedQuantity != 31.5 || recs[0].ServiceCategory != "storage" {
		t.Fatalf("unexpected: %+v", recs)
	}
}

func TestMalformedPayloadsFail(t *testing.T) {
	for _, p := range []string{`not json`, `{"EC2Hours":[{"TimePeriod":{"Start":"x","End":"y"},"Groups":[]}]}`,
		`{"EC2Hours":[{"TimePeriod":{"Start":"2026-09-01","End":"2026-09-02"},"Groups":[{"Keys":["a"],"Metrics":{}}]}]}`} {
		if _, err := (Normalizer{}).Normalize(focus.RawBatch{Payload: []byte(p)}); err == nil {
			t.Errorf("want an error for %s", p)
		}
	}
}
