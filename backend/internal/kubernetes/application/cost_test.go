package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/kubernetes/domain"
)

type costRepo struct {
	memRepo
	refs     []domain.ClusterRef
	inputs   map[string]domain.CostInputs
	pools    map[string][]string
	setCalls int
	capacity []domain.Capacity
}

func (r *costRepo) CostClusters(context.Context, string, *string, string, time.Time, time.Time) ([]domain.ClusterRef, error) {
	return r.refs, nil
}
func (r *costRepo) CostInputs(_ context.Context, _, p, c string, _, _ time.Time) (domain.CostInputs, error) {
	return r.inputs[p+"/"+c], nil
}
func (r *costRepo) GetPool(_ context.Context, _, p, c string) ([]string, error) {
	return r.pools[p+"/"+c], nil
}
func (r *costRepo) SetPool(_ context.Context, _, p, c string, s []string) error {
	r.setCalls++
	if r.pools == nil {
		r.pools = map[string][]string{}
	}
	r.pools[p+"/"+c] = s
	return nil
}
func (r *costRepo) UpsertCapacity(_ context.Context, _, _ string, rows []domain.Capacity) error {
	r.capacity = append(r.capacity, rows...)
	return nil
}

type fakePool struct {
	got      []string
	perDay   float64
	currency string
}

func (f *fakePool) DailyPool(_ context.Context, _, _ string, services []string, from, to time.Time) ([]PoolDay, string, error) {
	f.got = services
	var out []PoolDay
	for d := from; d.Before(to); d = d.AddDate(0, 0, 1) {
		out = append(out, PoolDay{Day: d, Cost: f.perDay})
	}
	return out, f.currency, nil
}
func (f *fakePool) Services(context.Context, string, string, time.Time, time.Time) ([]PoolService, string, error) {
	return []PoolService{{"Virtual Machines", 500}}, f.currency, nil
}

func TestCostReportJoinsThePoolTheHoursAndTheWorkloads(t *testing.T) {
	day := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	repo := &costRepo{refs: []domain.ClusterRef{{ProjectID: "p1", Cluster: "prod"}}, pools: map[string][]string{"p1/prod": {"Virtual Machines"}}, inputs: map[string]domain.CostInputs{
		"p1/prod": {Days: []domain.DayHours{{Day: day, HoursSeen: 24}}, Unmeasured: 2, Workloads: []domain.DayWorkload{
			{Day: day, Namespace: "shop", Workload: "web", CPUHold: 24, MemHold: 24, CPURequested: 24, CPUUsed: 6, CPUKnown: true, Hours: 24},
			{Day: day, Namespace: "ops", Workload: "agent", CPUHold: 24, MemHold: 24, Hours: 24}}}}}
	pool := &fakePool{perDay: 100, currency: "EUR"}
	s := Service{Repo: repo, Pool: pool}
	rep, err := s.Cost(context.Background(), "T", nil, "", day, day.AddDate(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Currency != "EUR" || len(rep.Clusters) != 1 {
		t.Fatalf("%+v", rep)
	}
	c := rep.Clusters[0]
	if c.PoolAssumed || len(pool.got) != 1 || pool.got[0] != "Virtual Machines" {
		t.Fatalf("the services chosen for the cluster are the ones the pool is read for: %+v %v", c, pool.got)
	}
	if c.PoolCost != 100 || c.Allocated != 100 || c.Idle != nil || c.Unmeasured != 2 || len(c.Workloads) != 2 || len(c.Namespaces) != 2 {
		t.Fatalf("%+v", c)
	}
	var web WorkloadLine
	for _, w := range c.Workloads {
		if w.Workload == "web" {
			web = w
		}
	}
	if c.WasteCost <= 0 || web.CPUUtilPct == nil || *web.CPUUtilPct != 25 || web.Waste <= 0 {
		t.Fatalf("waste of a workload that asks for more than it uses: %+v", c.Workloads)
	}
	if share := c.Namespaces[0].Share + c.Namespaces[1].Share; share < 99.999 || share > 100.001 {
		t.Fatalf("shares of the pool add up: %v", share)
	}
}

func TestWithoutAChosenPoolEveryComputeCostIsAssumedAndSaid(t *testing.T) {
	day := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	repo := &costRepo{refs: []domain.ClusterRef{{ProjectID: "p1", Cluster: "prod"}}}
	rep, err := Service{Repo: repo, Pool: &fakePool{perDay: 10}}.Cost(context.Background(), "T", nil, "", day, day.AddDate(0, 0, 1))
	if err != nil || !rep.Clusters[0].PoolAssumed || rep.Clusters[0].Unallocated != 0 {
		t.Fatalf("%v %+v", err, rep)
	}
	if _, err := (Service{Repo: repo}).Cost(context.Background(), "T", nil, "", day, day); !errors.Is(err, ErrNoPoolSource) {
		t.Fatalf("a deployment without a bill source does not pretend: %v", err)
	}
}

func TestSetPoolCleansTheListAndChecksTheProject(t *testing.T) {
	repo := &costRepo{memRepo: memRepo{project: true}}
	s := Service{Repo: repo, Pool: &fakePool{}}
	if err := s.SetPool(context.Background(), "T", "p1", "prod", []string{" Virtual Machines ", "", "Virtual Machines", "AKS"}); err != nil {
		t.Fatal(err)
	}
	if got := repo.pools["p1/prod"]; len(got) != 2 || got[0] != "Virtual Machines" || got[1] != "AKS" {
		t.Fatalf("%v", got)
	}
	repo.project = false
	if err := s.SetPool(context.Background(), "T", "nope", "prod", nil); !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("%v", err)
	}
}

func TestCapacityMayComeAloneAndIsValidated(t *testing.T) {
	repo := &costRepo{memRepo: memRepo{project: true}}
	s := Service{Repo: repo, Intensity: grid{}, Now: func() time.Time { return at }}
	good := domain.Capacity{Cluster: "c", PeriodStart: time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC), CPUCores: 16, MemoryBytes: 64 << 30}
	if res, err := s.IngestWithCapacity(context.Background(), "T", "p", nil, []domain.Capacity{good}); err != nil || res.Accepted != 0 || len(repo.capacity) != 1 {
		t.Fatalf("%v %+v %d", err, res, len(repo.capacity))
	}
	for name, bad := range map[string]domain.Capacity{
		"future":    {Cluster: "c", PeriodStart: time.Date(2026, 10, 10, 13, 0, 0, 0, time.UTC), CPUCores: 1, MemoryBytes: 1},
		"half hour": {Cluster: "c", PeriodStart: time.Date(2026, 10, 10, 9, 30, 0, 0, time.UTC), CPUCores: 1, MemoryBytes: 1},
		"negative":  {Cluster: "c", PeriodStart: good.PeriodStart, CPUCores: -1, MemoryBytes: 1},
		"name":      {Cluster: "bad name", PeriodStart: good.PeriodStart, CPUCores: 1, MemoryBytes: 1},
	} {
		if _, err := s.IngestWithCapacity(context.Background(), "T", "p", nil, []domain.Capacity{bad}); !errors.Is(err, domain.ErrInvalidSample) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := s.IngestWithCapacity(context.Background(), "T", "p", nil, nil); !errors.Is(err, domain.ErrInvalidSample) {
		t.Errorf("an empty request is refused: %v", err)
	}
}
