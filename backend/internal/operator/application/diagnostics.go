package application

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/operator/domain"
)

// diagTimeout bounds one self-test: a hung dependency is a finding, not a hung console.
var diagTimeout = 15 * time.Second

// diagTimeoutForTest lets a test shorten the bound.
func diagTimeoutForTest(d time.Duration) { diagTimeout = d }

// Diagnostics are the infrastructure self-tests the composition root supplies (database, queue, storage, mail, data sources...).
// The ones that need the console's own repository are built here.
func (s Service) checks() []domain.DiagCheck {
	out := append([]domain.DiagCheck(nil), s.Diagnostics...)
	out = append(out,
		domain.DiagCheck{ID: "data.grid_fresh", Group: "Data", Name: "Grid data is fresh", Description: "Every source in the policy that has data for a region in use has a recent reading.",
			Run: func(ctx context.Context) (domain.DiagStatus, string) {
				a, err := s.Aggregation(ctx, time.Now())
				if err != nil {
					return domain.DiagFail, "the aggregation could not be read"
				}
				var stale []string
				fresh := 0
				for _, src := range a.Sources {
					if src.Position == 0 || src.Regions == 0 {
						continue
					}
					if src.Freshness == "fresh" || src.Freshness == "aging" {
						fresh++
					} else {
						stale = append(stale, src.Label)
					}
				}
				switch {
				case fresh == 0:
					return domain.DiagFail, "no source has a recent reading for a region in use"
				case len(stale) > 0:
					return domain.DiagWarn, fmt.Sprintf("%d source(s) with data are stale: %s", len(stale), joinMax(stale, 4))
				}
				return domain.DiagOK, fmt.Sprintf("%d source(s) with recent readings", fresh)
			}},
		domain.DiagCheck{ID: "data.divergence", Group: "Data", Name: "Sources agree", Description: "No region where two sources differ by 2.5 times or more (Aggregation → Source divergence).",
			Run: func(ctx context.Context) (domain.DiagStatus, string) {
				a, err := s.Aggregation(ctx, time.Now())
				if err != nil {
					return domain.DiagFail, "the aggregation could not be read"
				}
				var bad []string
				for _, d := range a.Divergence {
					if d.Level == "disagrees" {
						bad = append(bad, d.Region)
					}
				}
				if len(bad) > 0 {
					sort.Strings(bad)
					return domain.DiagWarn, fmt.Sprintf("%d region(s) where sources disagree: %s", len(bad), joinMax(bad, 5))
				}
				return domain.DiagOK, fmt.Sprintf("%d region(s) compared, none disagrees", len(a.Divergence))
			}},
		domain.DiagCheck{ID: "data.provenance", Group: "Data", Name: "Figures have an origin", Description: "Share of the last 30 days of calculation rows whose intensity source is recorded.",
			Run: func(ctx context.Context) (domain.DiagStatus, string) {
				a, err := s.Aggregation(ctx, time.Now())
				if err != nil {
					return domain.DiagFail, "the aggregation could not be read"
				}
				if a.Provenance.Total == 0 {
					return domain.DiagSkipped, "no calculation rows in the last 30 days"
				}
				share := 100 - float64(a.Provenance.Unattributed)*100/float64(a.Provenance.Total)
				if share < 80 {
					return domain.DiagWarn, fmt.Sprintf("%.0f%% of rows have a recorded source (older rows are not guessed)", share)
				}
				return domain.DiagOK, fmt.Sprintf("%.0f%% of %d rows have a recorded source", share, a.Provenance.Total)
			}},
		domain.DiagCheck{ID: "connections.health", Group: "Customers", Name: "Customer connections work", Description: "No cloud connection of any organization is failing to synchronize.",
			Run: func(ctx context.Context) (domain.DiagStatus, string) {
				c, err := s.Repo.UnhealthyConnections(ctx, 50)
				if err != nil {
					return domain.DiagFail, "the connections could not be read"
				}
				if len(c) > 0 {
					return domain.DiagWarn, fmt.Sprintf("%d connection(s) failing", len(c))
				}
				return domain.DiagOK, "all connections synchronize"
			}},
		domain.DiagCheck{ID: "notifications.delivery", Group: "Customers", Name: "Notifications are delivered", Description: "Share of deliveries of the last 24 hours that failed.",
			Run: func(ctx context.Context) (domain.DiagStatus, string) {
				st, err := s.Repo.DeliveryStats(ctx, 24)
				if err != nil {
					return domain.DiagFail, "the deliveries could not be read"
				}
				var total, failed int64
				for _, x := range st {
					total += x.Count
					if x.Status == "failed" || x.Status == "dead" {
						failed += x.Count
					}
				}
				switch {
				case total == 0:
					return domain.DiagSkipped, "no deliveries in the last 24 hours"
				case float64(failed)/float64(total) > 0.2:
					return domain.DiagWarn, fmt.Sprintf("%d of %d deliveries failed", failed, total)
				}
				return domain.DiagOK, fmt.Sprintf("%d of %d deliveries failed", failed, total)
			}},
	)
	return out
}

func joinMax(v []string, n int) string {
	if len(v) <= n {
		return fmt.Sprint(v)
	}
	return fmt.Sprintf("%v and %d more", v[:n], len(v)-n)
}

// DiagnosticsCatalog lists the self-tests, in the order they are shown.
func (s Service) DiagnosticsCatalog() []domain.DiagResult {
	cs := s.checks()
	out := make([]domain.DiagResult, 0, len(cs))
	for _, c := range cs {
		out = append(out, domain.DiagResult{ID: c.ID, Group: c.Group, Name: c.Name, Description: c.Description})
	}
	return out
}

// RunDiagnostics runs the chosen self-tests (all when ids is empty) at once. A test that panics or runs too long is a failure
// of that test only.
func (s Service) RunDiagnostics(ctx context.Context, ids []string) []domain.DiagResult {
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var run []domain.DiagCheck
	for _, c := range s.checks() {
		if len(want) == 0 || want[c.ID] {
			run = append(run, c)
		}
	}
	out := make([]domain.DiagResult, len(run))
	var wg sync.WaitGroup
	for i, c := range run {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = runOne(ctx, c)
		}()
	}
	wg.Wait()
	return out
}

func runOne(ctx context.Context, c domain.DiagCheck) domain.DiagResult {
	res := domain.DiagResult{ID: c.ID, Group: c.Group, Name: c.Name, Description: c.Description, At: time.Now().UTC()}
	start := time.Now()
	cctx, cancel := context.WithTimeout(ctx, diagTimeout)
	defer cancel()
	type answer struct {
		st  domain.DiagStatus
		msg string
	}
	// buffered: a test that answers after the bound has nowhere to be blocked, and never touches the returned value
	ch := make(chan answer, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- answer{domain.DiagFail, "the test crashed"}
			}
		}()
		st, msg := c.Run(cctx)
		ch <- answer{st, msg}
	}()
	select {
	case a := <-ch:
		res.Status, res.Message = a.st, a.msg
	case <-cctx.Done():
		res.Status, res.Message = domain.DiagFail, "no answer within "+diagTimeout.String()
	}
	res.Ms = time.Since(start).Milliseconds()
	return res
}
