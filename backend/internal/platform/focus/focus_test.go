package focus

import (
	"math"
	"strings"
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
		"provider":            func(r *Record) { r.Provider = "oracle" },
		"category":            func(r *Record) { r.ServiceCategory = "ai" },
		"region":              func(r *Record) { r.RegionID = "" },
		"currency":            func(r *Record) { r.Currency = "US" },
		"currency lower case": func(r *Record) { r.Currency = "usd" },
		"currency digits":     func(r *Record) { r.Currency = "U5D" },
		"NaN billed cost":     func(r *Record) { r.BilledCost = math.NaN() },
		"Inf effective cost":  func(r *Record) { r.EffectiveCost = math.Inf(1) },
		"-Inf billed cost":    func(r *Record) { r.BilledCost = math.Inf(-1) },
		"period":              func(r *Record) { r.ChargePeriodEnd = r.ChargePeriodStart },
		"qty/unit":            func(r *Record) { r.ConsumedQuantity = &q },
		"NaN quantity":        func(r *Record) { n := math.NaN(); u := "Hrs"; r.ConsumedQuantity, r.ConsumedUnit = &n, &u },
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

func TestNormalizeTagKey(t *testing.T) {
	for in, want := range map[string]string{"Team": "team", " user_team ": "team", "user:Cost-Center": "cost-center", "aws:createdBy": "", "goog-dm": "", "microsoft.compute": "", "": "", "kubernetes.io/foo": ""} {
		if got := NormalizeTagKey(in); got != want {
			t.Errorf("NormalizeTagKey(%q) = %q, want %q", in, got, want)
		}
	}
	if NormalizeTagKey(strings.Repeat("a", 65)) != "" {
		t.Error("an over-long key is dropped")
	}
}

func TestSelectTags(t *testing.T) {
	keys := KeySet([]string{"team", "Env", "user_owner"})
	got := SelectTags(map[string]string{"user_team": " payments ", "ENV": "prod", "aws:createdBy": "x", "owner": "", "other": "y"}, keys)
	if len(got) != 2 || got["team"] != "payments" || got["env"] != "prod" {
		t.Fatalf("got %v", got)
	}
	if SelectTags(map[string]string{"x": "y"}, keys) != nil {
		t.Error("nothing selected is nil")
	}
	if v := SelectTags(map[string]string{"team": strings.Repeat("v", 300)}, keys)["team"]; len(v) != MaxTagValue {
		t.Errorf("value is cut to %d, got %d", MaxTagValue, len(v))
	}
}

func TestTaggedCostValidate(t *testing.T) {
	ok := TaggedCost{Provider: "aws", ServiceName: "S3", Currency: "USD", TagKey: "team", TagValue: "a", BilledCost: 1}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, mut := range map[string]func(*TaggedCost){"provider": func(x *TaggedCost) { x.Provider = "x" }, "key": func(x *TaggedCost) { x.TagKey = "" },
		"value": func(x *TaggedCost) { x.TagValue = "" }, "cur": func(x *TaggedCost) { x.Currency = "us" }, "nan": func(x *TaggedCost) { x.BilledCost = math.NaN() }} {
		b := ok
		mut(&b)
		if b.Validate() == nil {
			t.Errorf("%s must be refused", name)
		}
	}
}

func TestParseTagsCell(t *testing.T) {
	for in, want := range map[string]int{`{"team":"a","n":3}`: 2, `"team": "a", "env": "prod"`: 2, ``: 0, `not json`: 0, `{"team":{"x":1}}`: 0} {
		if got := ParseTagsCell(in); len(got) != want {
			t.Errorf("ParseTagsCell(%q) = %v, want %d entries", in, got, want)
		}
	}
	if ParseTagsCell(strings.Repeat("a", MaxTagsCell+1)) != nil {
		t.Error("an over-long cell is not parsed")
	}
}

func TestTagSums(t *testing.T) {
	s := NewTagSums([]string{"team"})
	s.Add("2026-09-01", "S3", "USD", map[string]string{"Team": "Pay", "x": "y"}, true, 5, 4)
	s.Add("2026-09-01", "S3", "USD", map[string]string{"user_team": "PAY"}, true, 1, 1)
	s.Add("2026-09-01", "S3", "USD", nil, true, 9, 9)
	rows := s.Rows()
	if len(rows) != 1 || rows[0].Value != "pay" || rows[0].Billed != 6 || rows[0].Effective != 5 {
		t.Fatalf("%+v", rows)
	}
	var none *TagSums
	none.Add("d", "s", "USD", map[string]string{"team": "a"}, false, 1, 1)
	if none.Len() != 0 || len(none.Rows()) != 0 || len(NewTagSums(nil).Rows()) != 0 {
		t.Fatal("a reader with no keys reads nothing")
	}
}
