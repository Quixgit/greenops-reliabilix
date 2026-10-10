package apiusage

import (
	"context"
	"errors"
	"testing"
)

func TestCapStopsRequestsOnceReached(t *testing.T) {
	m := &Meter{Cap: func(context.Context, string) int { return 3 }}
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := m.Allow(ctx, "electricitymaps"); err != nil {
			t.Fatalf("request %d refused: %v", i, err)
		}
		m.Record(ctx, "electricitymaps", i == 1)
	}
	if err := m.Allow(ctx, "electricitymaps"); !errors.Is(err, ErrBudget) {
		t.Errorf("the fourth request = %v, want ErrBudget", err)
	}
	if err := m.Allow(ctx, "other"); err != nil {
		t.Errorf("another provider has its own count: %v", err)
	}
}

func TestZeroMeansUnlimited(t *testing.T) {
	m := &Meter{Cap: func(context.Context, string) int { return 0 }}
	for i := 0; i < 1000; i++ {
		m.Record(context.Background(), "p", false)
	}
	if err := m.Allow(context.Background(), "p"); err != nil {
		t.Errorf("unlimited = %v", err)
	}
}
