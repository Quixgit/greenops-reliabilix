package costcsv

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func parse(t *testing.T, csv string, o Options) Result {
	t.Helper()
	if o.Vendor == "" {
		o.Vendor = "Datadog"
	}
	r, err := Parse([]byte(csv), o)
	if err != nil {
		t.Fatalf("%v", err)
	}
	for _, rec := range r.Records {
		if err := rec.Validate(); err != nil {
			t.Errorf("record fails FOCUS validation: %v (%+v)", err, rec)
		}
	}
	return r
}

func TestFocusStyleFileIsSummedPerDayServiceAndCurrency(t *testing.T) {
	r := parse(t, "ChargePeriodStart,ServiceName,BilledCost,EffectiveCost,BillingCurrency\n"+
		"2026-09-01T00:00:00Z,Log Management,10.50,9,USD\n"+
		"2026-09-01T00:00:00Z,Log Management,2.5,2.5,USD\n"+
		"2026-09-01T00:00:00Z,APM,4,4,USD\n"+
		"2026-09-02T00:00:00Z,APM,1,1,usd\n"+
		"2026-09-02T00:00:00Z,APM,,,\n", Options{})
	if len(r.Records) != 3 || r.Services != 2 || r.Total != 18 || r.Currency != "USD" || r.RowsRead != 5 || r.RowsSkipped != 1 {
		t.Fatalf("%+v", r)
	}
	first := r.Records[0]
	if first.Provider != "saas" || first.ServiceName != "Datadog: APM" || first.RegionID != "global" || first.ServiceCategory != "other" || first.Source != "cost_csv" {
		t.Fatalf("%+v", first)
	}
	var log Result = r
	for _, rec := range log.Records {
		if rec.ServiceName == "Datadog: Log Management" && (rec.BilledCost != 13 || rec.EffectiveCost != 11.5) {
			t.Fatalf("two rows of one day and service are one record: %+v", rec)
		}
	}
	if r.From.Format("2006-01-02") != "2026-09-01" || r.To.Format("2006-01-02") != "2026-09-02" {
		t.Fatalf("window %v..%v", r.From, r.To)
	}
	if r.Columns["date"] != "ChargePeriodStart" || r.Columns["cost"] != "BilledCost" || r.Columns["service"] != "ServiceName" {
		t.Fatalf("the answer says which columns it used: %v", r.Columns)
	}
}

func TestOtherVendorsSpellingsAndSeparators(t *testing.T) {
	// Snowflake-style: semicolons, decimal comma, a credit in parentheses, a currency sign, no currency column
	r := parse(t, "USAGE_DATE;SERVICE_TYPE;CREDITS_USED_COST\n2026-09-01;COMPUTE;\"1234,56\"\n2026-09-01;STORAGE;(10,5)\n2026-09-02;COMPUTE;$99.50\n", Options{Vendor: "Snowflake", Currency: "eur"})
	if r.Currency != "EUR" || len(r.Records) != 3 {
		t.Fatalf("%+v", r)
	}
	got := map[string]float64{}
	for _, rec := range r.Records {
		got[rec.ServiceName+"@"+rec.ChargePeriodStart.Format("01-02")] = rec.BilledCost
	}
	if got["Snowflake: COMPUTE@09-01"] != 1234.56 || got["Snowflake: STORAGE@09-01"] != -10.5 || got["Snowflake: COMPUTE@09-02"] != 99.5 {
		t.Fatalf("%v", got)
	}
	// US thousands separators with a comma are not decimal commas
	r2 := parse(t, "date,cost\n2026-09-01,\"1,234.50\"\n", Options{})
	if r2.Records[0].BilledCost != 1234.5 || r2.Warnings[len(r2.Warnings)-1] == "" {
		t.Fatalf("%+v", r2.Records)
	}
}

func TestMonthlyFiguresAreFlaggedNotSpread(t *testing.T) {
	r := parse(t, "Month,Service,Cost\n2026-08,Seats,300\n2026-09,Seats,300\n", Options{})
	if len(r.Records) != 2 || r.Records[0].ChargePeriodStart.Day() != 1 {
		t.Fatalf("%+v", r.Records)
	}
	if !contains(r.Warnings, "first day of the month") {
		t.Fatalf("warnings %v", r.Warnings)
	}
}

func contains(ws []string, s string) bool {
	for _, w := range ws {
		if strings.Contains(w, s) {
			return true
		}
	}
	return false
}

func TestTagsFromAJSONColumnAndFromTagColumns(t *testing.T) {
	r := parse(t, "Date,Service,Cost,Tags,tag_team,label_Env\n"+
		"2026-09-01,Logs,10,\"{\"\"team\"\":\"\"Payments\"\",\"\"x\"\":\"\"y\"\"}\",,\n"+
		"2026-09-01,Logs,5,,Search,prod\n"+
		"2026-09-01,Logs,1,,,\n", Options{TagKeys: []string{"team", "env"}})
	got := map[string]float64{}
	for _, x := range r.Tagged {
		got[x.Key+"="+x.Value] = x.Billed
	}
	if len(got) != 3 || got["team=payments"] != 10 || got["team=search"] != 5 || got["env=prod"] != 5 {
		t.Fatalf("%v", got)
	}
	// no allocation keys: no tags are kept
	if none := parse(t, "Date,Service,Cost,Tags\n2026-09-01,Logs,10,\"{\"\"team\"\":\"\"a\"\"}\"\n", Options{}); len(none.Tagged) != 0 {
		t.Fatal("tags are kept only for the allocation keys")
	}
}

func TestFilesThatCannotBeUsedAreRefusedWithAReason(t *testing.T) {
	for name, c := range map[string]struct {
		csv  string
		opt  Options
		want string
	}{
		"empty":         {"", Options{Vendor: "X"}, "empty"},
		"no vendor":     {"date,cost\n2026-09-01,1\n", Options{}, "vendor"},
		"bad currency":  {"date,cost\n2026-09-01,1\n", Options{Vendor: "X", Currency: "dollars"}, "three-letter"},
		"no cost":       {"date,notes\n2026-09-01,hello\n", Options{Vendor: "X"}, "needs a date column"},
		"header only":   {"date,cost\n", Options{Vendor: "X"}, "no rows"},
		"nothing valid": {"date,cost\nsoon,lots\nnever,none\n", Options{Vendor: "X"}, "no row could be used"},
		"absurd dates":  {"date,cost\n0001-01-01,5\n", Options{Vendor: "X"}, "no row could be used"},
	} {
		_, err := Parse([]byte(c.csv), c.opt)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want it to mention %q", name, err, c.want)
		}
	}
	// the refusal names what was found, so the person can see what to rename
	_, err := Parse([]byte("foo,bar\n1,2\n"), Options{Vendor: "X"})
	if err == nil || !strings.Contains(err.Error(), "foo, bar") {
		t.Errorf("the error should list the header: %v", err)
	}
}

func TestLimitsAreEnforced(t *testing.T) {
	var b strings.Builder
	b.WriteString("date,cost\n")
	for i := 0; i < MaxRows+1; i++ {
		fmt.Fprintf(&b, "2026-09-01,1\n")
	}
	if _, err := Parse([]byte(b.String()), Options{Vendor: "X"}); err == nil || !strings.Contains(err.Error(), "more than") {
		t.Errorf("too many rows: %v", err)
	}
	if _, err := Parse(make([]byte, MaxBytes+1), Options{Vendor: "X"}); err == nil {
		t.Error("too large a file")
	}
}

func TestRowsOfNothingAndMixedCurrencies(t *testing.T) {
	r := parse(t, "date,cost,currency\n2026-09-01,0,USD\n2026-09-01,10,USD\n2026-09-01,5,EUR\n", Options{})
	if len(r.Records) != 2 || r.Currency != "USD" || r.Total != 10 || !contains(r.Warnings, "more than one currency") {
		t.Fatalf("%+v", r)
	}
	if math.IsNaN(r.Total) {
		t.Fatal("nan")
	}
}

func TestParseAmount(t *testing.T) {
	for in, want := range map[string]float64{"12": 12, "-3.5": -3.5, "(7)": -7, "$1,000": 1000, "€ 2,50": 2.5, "1 000,25": 1000.25} {
		if got, ok := parseAmount(in, false); !ok || math.Abs(got-want) > 1e-9 {
			t.Errorf("parseAmount(%q) = %v %v, want %v", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "abc", "NaN", "Inf", "1e99"} {
		if _, ok := parseAmount(in, false); ok {
			t.Errorf("parseAmount(%q) must fail", in)
		}
	}
}
