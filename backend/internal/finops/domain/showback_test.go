package domain

import (
	"math"
	"testing"
)

func TestBuildShowbackSplitsByValueAndNeverSpreadsTheRest(t *testing.T) {
	sb := BuildShowback("team", "USD", []TagCost{
		{"payments", "EC2", 60}, {"payments", "S3", 20}, {"payments", "RDS", 10}, {"payments", "SQS", 1},
		{"search", "EC2", 30},
	}, 200)
	if len(sb.Lines) != 2 || sb.Lines[0].Value != "payments" || sb.Lines[0].Cost != 91 {
		t.Fatalf("%+v", sb.Lines)
	}
	if sb.Tagged != 121 || sb.Untagged != 79 || math.Abs(sb.Coverage-60.5) > 1e-9 || math.Abs(sb.Lines[0].Share-45.5) > 1e-9 {
		t.Fatalf("tagged %v untagged %v coverage %v share %v", sb.Tagged, sb.Untagged, sb.Coverage, sb.Lines[0].Share)
	}
	if got := sb.Lines[0].TopServices; len(got) != 3 || got[0].Service != "EC2" || got[2].Service != "RDS" {
		t.Fatalf("the three biggest services only: %+v", got)
	}
	if !sb.HasData {
		t.Fatal("has data")
	}
}

func TestBuildShowbackWithNothingTaggedStillReportsTheBill(t *testing.T) {
	sb := BuildShowback("team", "EUR", nil, 150)
	if sb.Lines == nil || len(sb.Lines) != 0 || sb.Untagged != 150 || sb.Coverage != 0 || !sb.HasData {
		t.Fatalf("%+v", sb)
	}
	if empty := BuildShowback("team", "EUR", nil, 0); empty.HasData || empty.Coverage != 0 {
		t.Fatalf("no data at all: %+v", empty)
	}
}

func TestBuildShowbackKeepsCreditsAndNeverExceedsFullCoverage(t *testing.T) {
	sb := BuildShowback("env", "USD", []TagCost{{"prod", "EC2", 100}, {"test", "EC2", -5}}, 100)
	if sb.Lines[len(sb.Lines)-1].Value != "test" || sb.Lines[len(sb.Lines)-1].Cost != -5 {
		t.Fatalf("a credit is kept and sorts last: %+v", sb.Lines)
	}
	over := BuildShowback("env", "USD", []TagCost{{"a", "S", 120}}, 100) // rounding or a lagging total
	if over.Coverage != 100 || over.Untagged != 0 {
		t.Fatalf("coverage is capped at 100 (untagged %v, never negative)", over.Untagged)
	}
}
