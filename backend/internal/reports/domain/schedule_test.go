package domain

import (
	"testing"
	"time"
)

func d(s string) time.Time { t, _ := time.Parse("2006-01-02 15:04", s); return t }

func TestNextRun(t *testing.T) {
	for _, c := range []struct {
		cadence Cadence
		from    string
		want    string
	}{
		{Weekly, "2026-10-11 12:00", "2026-10-12 06:00"}, // Sunday -> Monday
		{Weekly, "2026-10-12 05:59", "2026-10-12 06:00"}, // Monday before the hour: today
		{Weekly, "2026-10-12 06:00", "2026-10-19 06:00"}, // exactly at the run: strictly after
		{Monthly, "2026-10-11 12:00", "2026-11-01 06:00"},
		{Monthly, "2026-11-01 06:00", "2026-12-01 06:00"},
		{Monthly, "2026-12-15 00:00", "2027-01-01 06:00"},
	} {
		if got := NextRun(c.cadence, d(c.from)); !got.Equal(d(c.want)) {
			t.Errorf("%s from %s: got %s, want %s", c.cadence, c.from, got, c.want)
		}
	}
}

func TestPeriod(t *testing.T) {
	// A Monday run reports the seven days Monday..Sunday before it.
	s, e := Period(Weekly, d("2026-10-12 06:00"))
	if s.Format(time.DateOnly) != "2026-10-05" || e.Format(time.DateOnly) != "2026-10-11" {
		t.Errorf("weekly: %s..%s", s, e)
	}
	// A monthly run on the 1st reports the whole month before, including the year boundary and February.
	s, e = Period(Monthly, d("2027-01-01 06:00"))
	if s.Format(time.DateOnly) != "2026-12-01" || e.Format(time.DateOnly) != "2026-12-31" {
		t.Errorf("monthly: %s..%s", s, e)
	}
	s, e = Period(Monthly, d("2028-03-01 06:00"))
	if e.Format(time.DateOnly) != "2028-02-29" || s.Format(time.DateOnly) != "2028-02-01" {
		t.Errorf("leap February: %s..%s", s, e)
	}
}

func TestScheduleValidate(t *testing.T) {
	ok := Schedule{Kind: KindSCI, Format: PDF, Cadence: Weekly, Recipients: []string{"a@example.com"}}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, mut := range map[string]func(*Schedule){
		"kind":      func(s *Schedule) { s.Kind = "x" },
		"format":    func(s *Schedule) { s.Format = "xls" },
		"cadence":   func(s *Schedule) { s.Cadence = "daily" },
		"address":   func(s *Schedule) { s.Recipients = []string{"not an address"} },
		"display":   func(s *Schedule) { s.Recipients = []string{"A <a@example.com>"} },
		"injection": func(s *Schedule) { s.Recipients = []string{"a@example.com\r\nBcc: x@y.z"} },
		"too many": func(s *Schedule) {
			s.Recipients = make([]string, MaxRecipients+1)
			for i := range s.Recipients {
				s.Recipients[i] = "a@example.com"
			}
		},
	} {
		s := ok
		mut(&s)
		if s.Validate() == nil {
			t.Errorf("%s must be rejected", name)
		}
	}
	n := Schedule{Recipients: []string{" A@Example.com ", "a@example.com", ""}}
	n.Normalize()
	if len(n.Recipients) != 1 || n.Recipients[0] != "a@example.com" {
		t.Errorf("normalize: %v", n.Recipients)
	}
}
