package domain

import (
	"testing"
	"time"
)

func valid() UsageRecord {
	now := time.Now()
	return UsageRecord{TenantID: "t", ProjectID: "p", Provider: "aws", Region: "us-east-1",
		UsageAmount: 1, Cost: 1, Currency: "USD", PeriodStart: now, PeriodEnd: now.Add(time.Hour)}
}

func TestValidate(t *testing.T) {
	if err := valid().Validate(); err != nil {
		t.Fatal(err)
	}
	bad := valid()
	bad.Provider = "oracle"
	if bad.Validate() == nil {
		t.Error("unknown provider accepted")
	}
	bad = valid()
	bad.PeriodEnd = bad.PeriodStart
	if bad.Validate() == nil {
		t.Error("empty period accepted")
	}
}
