package application

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/kubernetes/domain"
)

// PoolDay is what the nodes of a cluster cost on a day, in the reporting currency.
type PoolDay struct {
	Day  time.Time
	Cost float64
}

// PoolService is a billed service that could be paying for a cluster, with its cost in the window.
type PoolService struct {
	Name string  `json:"name"`
	Cost float64 `json:"cost"`
}

// PoolSource reads the cloud bill (the usage module, wired in the composition root). An empty services list means every compute cost of the project.
type PoolSource interface {
	DailyPool(ctx context.Context, tenantID, projectID string, services []string, from, to time.Time) ([]PoolDay, string, error)
	Services(ctx context.Context, tenantID, projectID string, from, to time.Time) ([]PoolService, string, error)
}

// ErrNoPoolSource: cost allocation is not wired (a deployment without the usage module).
var ErrNoPoolSource = errors.New("kubernetes: no source for the cluster cost")

// ClusterCost is the answer for one cluster.
type ClusterCost struct {
	ProjectID string `json:"project_id"`
	Cluster   string `json:"cluster"`
	// PoolServices are the billed services chosen to pay for the cluster; empty means every compute cost of the project (assumed).
	PoolServices []string        `json:"pool_services"`
	PoolAssumed  bool            `json:"pool_assumed"`
	PoolCost     float64         `json:"pool_cost"` // of the hours the agent reported
	Allocated    float64         `json:"allocated_cost"`
	Idle         *float64        `json:"idle_cost"` // nil when the nodes' capacity is not reported (never shown as zero)
	IdleDays     int             `json:"idle_days"`
	Days         int             `json:"days"`
	Unallocated  float64         `json:"unallocated_cost"`
	Unmeasured   int             `json:"workloads_without_usage"`
	WasteCost    float64         `json:"waste_cost"`
	Namespaces   []NamespaceLine `json:"namespaces"`
	Workloads    []WorkloadLine  `json:"workloads"`
}

// NamespaceLine and WorkloadLine are the lines of the report.
type NamespaceLine struct {
	Namespace string  `json:"namespace"`
	Cost      float64 `json:"cost"`
	Share     float64 `json:"share_pct"`
	Waste     float64 `json:"waste_cost"`
	Workloads int     `json:"workloads"`
}

type WorkloadLine struct {
	Namespace  string   `json:"namespace"`
	Workload   string   `json:"workload"`
	Cost       float64  `json:"cost"`
	CPUCost    float64  `json:"cpu_cost"`
	MemCost    float64  `json:"memory_cost"`
	Waste      float64  `json:"waste_cost"`
	CPUUtilPct *float64 `json:"cpu_utilization_pct"`
	MemUtilPct *float64 `json:"memory_utilization_pct"`
	Hours      int      `json:"hours"`
}

// CostReport is the cost of the clusters that reported in the window.
type CostReport struct {
	Currency string        `json:"currency"`
	CPUShare float64       `json:"cpu_share"`
	Clusters []ClusterCost `json:"clusters"`
}

const maxWorkloadLines = 200

// Cost allocates the cost of each cluster over its workloads (ADR-0063).
func (s Service) Cost(ctx context.Context, tenantID string, project *string, cluster string, from, to time.Time) (CostReport, error) {
	rep := CostReport{CPUShare: domain.CPUShare, Clusters: []ClusterCost{}}
	if s.Pool == nil {
		return rep, ErrNoPoolSource
	}
	refs, err := s.Repo.CostClusters(ctx, tenantID, project, cluster, from, to)
	if err != nil {
		return rep, err
	}
	for _, ref := range refs {
		services, err := s.Repo.GetPool(ctx, tenantID, ref.ProjectID, ref.Cluster)
		if err != nil {
			return rep, err
		}
		pool, cur, err := s.Pool.DailyPool(ctx, tenantID, ref.ProjectID, services, from, to)
		if err != nil {
			return rep, err
		}
		if cur != "" {
			rep.Currency = cur
		}
		in, err := s.Repo.CostInputs(ctx, tenantID, ref.ProjectID, ref.Cluster, from, to)
		if err != nil {
			return rep, err
		}
		hours := map[time.Time]domain.DayHours{}
		for _, d := range in.Days {
			hours[d.Day.UTC().Truncate(24*time.Hour)] = d
		}
		days := make([]domain.DayCluster, 0, len(pool))
		for _, p := range pool {
			day := p.Day.UTC().Truncate(24 * time.Hour)
			h := hours[day]
			days = append(days, domain.DayCluster{Day: day, Pool: p.Cost, HoursSeen: h.HoursSeen, CapCPU: h.CapCPU, CapMem: h.CapMem, CapHours: h.CapHours})
		}
		a := domain.Allocate(days, in.Workloads)
		cc := ClusterCost{ProjectID: ref.ProjectID, Cluster: ref.Cluster, PoolServices: services, PoolAssumed: len(services) == 0, PoolCost: a.Pool, Allocated: a.Allocated,
			IdleDays: a.IdleDays, Days: a.Days, Unallocated: a.Unallocated, Unmeasured: in.Unmeasured, Namespaces: []NamespaceLine{}, Workloads: []WorkloadLine{}}
		if a.Days > 0 && a.IdleDays > 0 {
			idle := a.Idle
			cc.Idle = &idle
		}
		for _, n := range a.Namespaces {
			share := 0.0
			if a.Pool > 0 {
				share = n.Cost / a.Pool * 100
			}
			cc.WasteCost += n.Waste
			cc.Namespaces = append(cc.Namespaces, NamespaceLine{Namespace: n.Namespace, Cost: n.Cost, Share: share, Waste: n.Waste, Workloads: n.Workloads})
		}
		for i, w := range a.Workloads {
			if i >= maxWorkloadLines {
				break
			}
			cc.Workloads = append(cc.Workloads, WorkloadLine{Namespace: w.Namespace, Workload: w.Workload, Cost: w.Cost, CPUCost: w.CPUCost, MemCost: w.MemCost, Waste: w.Waste,
				CPUUtilPct: w.CPUUtilPct, MemUtilPct: w.MemUtilPct, Hours: w.Hours})
		}
		rep.Clusters = append(rep.Clusters, cc)
	}
	sort.Slice(rep.Clusters, func(i, j int) bool { return rep.Clusters[i].PoolCost > rep.Clusters[j].PoolCost })
	return rep, nil
}

// PoolChoice is what the screen needs to choose the services that pay for a cluster.
type PoolChoice struct {
	Services   []string      `json:"services"`
	Candidates []PoolService `json:"candidates"`
	Currency   string        `json:"currency"`
}

func (s Service) PoolChoice(ctx context.Context, tenantID, projectID, cluster string, from, to time.Time) (PoolChoice, error) {
	if s.Pool == nil {
		return PoolChoice{}, ErrNoPoolSource
	}
	cur, err := s.Repo.GetPool(ctx, tenantID, projectID, cluster)
	if err != nil {
		return PoolChoice{}, err
	}
	cands, currency, err := s.Pool.Services(ctx, tenantID, projectID, from, to)
	if err != nil {
		return PoolChoice{}, err
	}
	return PoolChoice{Services: cur, Candidates: cands, Currency: currency}, nil
}

var ErrInvalidPool = errors.New("kubernetes: invalid pool")

// SetPool chooses the billed services that pay for a cluster (none: every compute cost of the project).
func (s Service) SetPool(ctx context.Context, tenantID, projectID, cluster string, services []string) error {
	clean := make([]string, 0, len(services))
	seen := map[string]bool{}
	for _, x := range services {
		x = strings.TrimSpace(x)
		if x == "" || len(x) > 200 || seen[x] {
			continue
		}
		seen[x] = true
		clean = append(clean, x)
	}
	if len(clean) > 50 {
		return ErrInvalidPool
	}
	if ok, err := s.Repo.ProjectExists(ctx, tenantID, projectID); err != nil {
		return err
	} else if !ok {
		return ErrProjectNotFound
	}
	return s.Repo.SetPool(ctx, tenantID, projectID, cluster, clean)
}
