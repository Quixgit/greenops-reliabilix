package repository

import "testing"

func TestActivityDetailKeepsOnlyTheAllowlistedFacts(t *testing.T) {
	got := activityDetail([]byte(`{"name":"production","records":1204,"allowed_regions":["a","b"],"secret":"x","kind":"sci","nested":{"a":1}}`))
	if got["name"] != "production" || got["records"] != "1204" || got["kind"] != "sci" {
		t.Errorf("facts lost: %v", got)
	}
	if _, leaked := got["secret"]; leaked || len(got) != 3 {
		t.Errorf("only the allowlist may leave: %v", got)
	}
	if len(activityDetail(nil)) != 0 || len(activityDetail([]byte("not json"))) != 0 {
		t.Error("no metadata, no detail")
	}
}
