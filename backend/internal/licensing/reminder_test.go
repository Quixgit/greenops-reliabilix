package licensing

import (
	"strings"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
)

func TestReminderEscalatesAsTheEndComes(t *testing.T) {
	end := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		days int
		typ  string
		sev  notify.Severity
	}{{7, notify.TypeBillingRenewalDue, notify.Info}, {3, notify.TypeBillingRenewalDue, notify.Warning}, {1, notify.TypeBillingRenewalDue, notify.Critical}, {-1, notify.TypeBillingPlanEnded, notify.Warning}}
	keys := map[string]bool{}
	for _, c := range cases {
		e := ReminderEvent("Team", end, c.days)
		if e.Type != c.typ || e.Severity != c.sev {
			t.Fatalf("%d days: %s %s", c.days, e.Type, e.Severity)
		}
		if err := e.Validate(); err != nil {
			t.Fatalf("%d days: %v", c.days, err)
		}
		if keys[e.DedupeKey] || !strings.Contains(e.DedupeKey, "2026-11-01") {
			t.Fatalf("keys must differ per step and carry the period: %s", e.DedupeKey)
		}
		keys[e.DedupeKey] = true
	}
}

func TestSameDistanceSameKey(t *testing.T) {
	end := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	if ReminderEvent("Team", end, 6).DedupeKey != ReminderEvent("Team", end, 5).DedupeKey {
		t.Fatal("the long-range reminder must go once, not every day")
	}
}
