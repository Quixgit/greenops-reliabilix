package application

import (
	"context"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/operator/domain"
)

func TestDiagnosticsRunTogetherAndIsolateFailures(t *testing.T) {
	s := Service{Diagnostics: []domain.DiagCheck{
		{ID: "a", Group: "G", Name: "ok", Run: func(context.Context) (domain.DiagStatus, string) { return domain.DiagOK, "fine" }},
		{ID: "b", Group: "G", Name: "crash", Run: func(context.Context) (domain.DiagStatus, string) { panic("boom") }},
		{ID: "c", Group: "G", Name: "warn", Run: func(context.Context) (domain.DiagStatus, string) { return domain.DiagWarn, "slow" }},
	}}
	got := map[string]domain.DiagResult{}
	// the repository-backed checks need a repository; ask only for ours
	for _, r := range s.RunDiagnostics(context.Background(), []string{"a", "b", "c"}) {
		got[r.ID] = r
	}
	if len(got) != 3 || got["a"].Status != domain.DiagOK || got["b"].Status != domain.DiagFail || got["c"].Status != domain.DiagWarn {
		t.Fatalf("%+v", got)
	}
	if got["b"].Message != "the test crashed" || got["a"].At.IsZero() {
		t.Fatalf("%+v", got["b"])
	}
}

func TestACheckThatNeverAnswersIsAFailureNotAHang(t *testing.T) {
	old := diagTimeout
	defer func() { diagTimeoutForTest(old) }()
	diagTimeoutForTest(50 * time.Millisecond)
	s := Service{Diagnostics: []domain.DiagCheck{{ID: "slow", Run: func(ctx context.Context) (domain.DiagStatus, string) {
		<-time.After(5 * time.Second)
		return domain.DiagOK, ""
	}}}}
	start := time.Now()
	r := s.RunDiagnostics(context.Background(), []string{"slow"})
	if len(r) != 1 || r[0].Status != domain.DiagFail || time.Since(start) > 2*time.Second {
		t.Fatalf("%+v after %s", r, time.Since(start))
	}
}
