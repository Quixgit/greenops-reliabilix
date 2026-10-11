package application

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/reports/domain"
)

type fakeRepo struct {
	domain.Repository
	rows int
}

func (f *fakeRepo) CarbonTable(context.Context, string, *string, time.Time, time.Time) (domain.Table, error) {
	t := domain.Table{Title: "Carbon", Headers: []string{"day", "kg"}}
	for i := 0; i < f.rows; i++ {
		t.Rows = append(t.Rows, []string{fmt.Sprint(i), "1"})
	}
	return t, nil
}

func TestPreviewReturnsTheHeadAndTheTotal(t *testing.T) {
	s := Service{Repo: &fakeRepo{rows: 30}}
	day := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	p, err := s.Preview(context.Background(), "t", domain.Report{Kind: domain.KindCarbon, PeriodStart: day, PeriodEnd: day.AddDate(0, 0, 6)})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Rows) != PreviewRows || p.TotalRows != 30 {
		t.Fatalf("rows %d total %d", len(p.Rows), p.TotalRows)
	}
}

func TestPreviewRejectsAnInvertedPeriod(t *testing.T) {
	s := Service{Repo: &fakeRepo{}}
	day := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	if _, err := s.Preview(context.Background(), "t", domain.Report{Kind: domain.KindCarbon, PeriodStart: day, PeriodEnd: day.AddDate(0, 0, -1)}); err == nil {
		t.Fatal("an inverted period must be refused")
	}
}

func TestFormatsAreAllWithoutSettings(t *testing.T) {
	if got := (Service{}).Formats(context.Background(), "t"); len(got) != 3 {
		t.Fatalf("got %v", got)
	}
}
