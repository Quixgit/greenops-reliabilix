package application

import (
	"strings"
	"testing"

	"github.com/quixgit/greenops-reliabilix/backend/internal/reports/domain"
)

func TestChange(t *testing.T) {
	for _, c := range []struct {
		cur, prev float64
		want      string
	}{{110, 100, "+10%"}, {90, 100, "-10%"}, {100, 100, "no change"}, {5, 0, "new"}, {0, 0, "no change"}, {100.2, 100, "no change"}} {
		if got := change("en", c.cur, c.prev); got != c.want {
			t.Errorf("change(%v,%v) = %q, want %q", c.cur, c.prev, got, c.want)
		}
	}
}

func TestHeadlineText(t *testing.T) {
	got := headlineText("en", domain.Headline{Currency: "USD", Cost: 1234.5, PrevCost: 1000, CarbonKg: 56.78, PrevCarbonKg: 60, TopService: "AmazonEC2", TopServiceCost: 800})
	for _, want := range []string{"1234.50 USD", "+23%", "56.8 kgCO2e", "-5%", "AmazonEC2 (800.00 USD)"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q missing from %q", want, got)
		}
	}
	if ru := headlineText("ru", domain.Headline{Currency: "USD", Cost: 1}); !strings.Contains(ru, "Расходы") {
		t.Errorf("russian text: %q", ru)
	}
}
