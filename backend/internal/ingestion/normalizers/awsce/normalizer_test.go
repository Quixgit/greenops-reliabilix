package awsce

import (
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

const fixture = `{"ResultsByTime":[
 {"TimePeriod":{"Start":"2026-09-01","End":"2026-09-02"},"Estimated":false,"Groups":[
   {"Keys":["Amazon Elastic Compute Cloud - Compute","eu-central-1"],"Metrics":{"UnblendedCost":{"Amount":"12.50","Unit":"USD"},"AmortizedCost":{"Amount":"11.00","Unit":"USD"}}},
   {"Keys":["Amazon Route 53","NoRegion"],"Metrics":{"UnblendedCost":{"Amount":"0.50","Unit":"USD"}}},
   {"Keys":["AWS Support","global"],"Metrics":{"UnblendedCost":{"Amount":"0","Unit":"USD"},"AmortizedCost":{"Amount":"0","Unit":"USD"}}},
   {"Keys":["EC2 - Other","us-east-1"],"Metrics":{"UnblendedCost":{"Amount":"-2.25","Unit":"USD"},"AmortizedCost":{"Amount":"-2.25","Unit":"USD"}}}]},
 {"TimePeriod":{"Start":"2026-09-02","End":"2026-09-03"},"Estimated":true,"Groups":[
   {"Keys":["Amazon Simple Storage Service","us-east-1"],"Metrics":{"UnblendedCost":{"Amount":"3","Unit":"USD"}}}]}]}`

func TestNormalizeFixture(t *testing.T) {
	recs, err := Normalizer{}.Normalize(focus.RawBatch{Provider: "aws", Source: "cost_explorer", Payload: []byte(fixture)})
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 3 {
		t.Fatalf("got %d records, want 3 (zero-cost and estimated rows dropped): %+v", len(recs), recs)
	}
	ec2 := recs[0]
	if ec2.ServiceName != "Amazon Elastic Compute Cloud - Compute" || ec2.ServiceCategory != "compute" || ec2.RegionID != "eu-central-1" ||
		ec2.BilledCost != 12.5 || ec2.EffectiveCost != 11 || ec2.Currency != "USD" || ec2.Provider != "aws" || ec2.Source != "cost_explorer" {
		t.Errorf("ec2 record wrong: %+v", ec2)
	}
	if !ec2.ChargePeriodStart.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)) || !ec2.ChargePeriodEnd.Equal(time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("period wrong: %v - %v", ec2.ChargePeriodStart, ec2.ChargePeriodEnd)
	}
	if r53 := recs[1]; r53.RegionID != "global" || r53.EffectiveCost != 0.5 || r53.ServiceCategory != "networking" {
		t.Errorf("route53 (no amortized metric, NoRegion): %+v", r53)
	}
	if credit := recs[2]; credit.BilledCost != -2.25 || credit.ServiceCategory != "other" {
		t.Errorf("credit/EC2-Other: %+v", credit)
	}
	for i, r := range recs {
		if err := r.Validate(); err != nil {
			t.Errorf("record %d does not satisfy the FOCUS contract: %v", i, err)
		}
	}
}

func TestNormalizeRejectsMalformed(t *testing.T) {
	for name, payload := range map[string]string{
		"not json":   `nope`,
		"bad date":   `{"ResultsByTime":[{"TimePeriod":{"Start":"x","End":"y"}}]}`,
		"one key":    `{"ResultsByTime":[{"TimePeriod":{"Start":"2026-09-01","End":"2026-09-02"},"Groups":[{"Keys":["a"],"Metrics":{}}]}]}`,
		"no metric":  `{"ResultsByTime":[{"TimePeriod":{"Start":"2026-09-01","End":"2026-09-02"},"Groups":[{"Keys":["a","b"],"Metrics":{}}]}]}`,
		"bad amount": `{"ResultsByTime":[{"TimePeriod":{"Start":"2026-09-01","End":"2026-09-02"},"Groups":[{"Keys":["a","b"],"Metrics":{"UnblendedCost":{"Amount":"abc","Unit":"USD"}}}]}]}`,
	} {
		if _, err := (Normalizer{}).Normalize(focus.RawBatch{Payload: []byte(payload)}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestCategoryFor(t *testing.T) {
	for svc, want := range map[string]string{
		"Amazon Elastic Compute Cloud - Compute": "compute", "AWS Lambda": "compute", "Amazon Relational Database Service": "database",
		"Amazon DynamoDB": "database", "Amazon Simple Storage Service": "storage", "Amazon Elastic Block Store": "storage",
		"Amazon Virtual Private Cloud": "networking", "Amazon CloudFront": "networking", "EC2 - Other": "other", "AWS Support (Business)": "other",
	} {
		if got := CategoryFor(svc); got != want {
			t.Errorf("%s -> %s, want %s", svc, got, want)
		}
	}
}
