package app

import (
	"context"
	"time"

	carbonapp "github.com/quixgit/greenops-reliabilix/backend/internal/carbon/application"
	carbon "github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/application"
	recsapp "github.com/quixgit/greenops-reliabilix/backend/internal/recommendations/application"
	usageapp "github.com/quixgit/greenops-reliabilix/backend/internal/usage/application"
)

// The composition root is the only place that knows several domains at once. Domains declare the ports they
// need (small interfaces); these adapters satisfy them from other modules, so domains never import each other.

// usageForCarbon feeds the carbon engine from stored FOCUS usage.
type usageForCarbon struct{ usage usageapp.Service }

func (a usageForCarbon) DailyAggregates(ctx context.Context, tenantID, projectID string, from, to time.Time) ([]carbon.UsageDay, error) {
	rows, err := a.usage.DailyAggregates(ctx, tenantID, projectID, from, to)
	out := make([]carbon.UsageDay, 0, len(rows))
	for _, r := range rows {
		out = append(out, carbon.UsageDay{Provider: r.Provider, ServiceName: r.ServiceName, ServiceCategory: r.ServiceCategory, RegionID: r.RegionID,
			Currency: r.Currency, ConsumedUnit: r.ConsumedUnit, Day: r.Day, Quantity: r.Quantity, HasQuantity: r.HasQuantity, BilledCost: r.BilledCost})
	}
	return out, err
}

// projectPorts adapts the projects module to the ports of carbon and recommendations.
type projectPorts struct{ projects application.Service }

func (a projectPorts) FunctionalUnitsForDay(ctx context.Context, tenantID, projectID string, day time.Time) (float64, error) {
	return a.projects.FunctionalUnitsForDay(ctx, tenantID, projectID, day)
}

func (a projectPorts) CIThresholds(ctx context.Context, tenantID, projectID string) (carbon.Thresholds, error) {
	p, err := a.projects.GetPolicy(ctx, tenantID, projectID)
	if err != nil {
		return carbon.Thresholds{}, err
	}
	return carbon.Thresholds{CarbonWarnKg: p.CICarbonWarnKg, CarbonBlockKg: p.CICarbonBlockKg, CostWarn: p.CICostWarn, CostBlock: p.CICostBlock}, nil
}

type carbonProjects struct{ projects application.Service }

func (a carbonProjects) ProjectsForJobs(ctx context.Context) ([]carbonapp.ProjectRef, error) {
	refs, err := a.projects.ListForJobs(ctx)
	out := make([]carbonapp.ProjectRef, 0, len(refs))
	for _, r := range refs {
		out = append(out, carbonapp.ProjectRef{TenantID: r.TenantID, ProjectID: r.ProjectID})
	}
	return out, err
}

type recsProjects struct{ projects application.Service }

func (a recsProjects) ProjectsForJobs(ctx context.Context) ([]recsapp.ProjectRef, error) {
	refs, err := a.projects.ListForJobs(ctx)
	out := make([]recsapp.ProjectRef, 0, len(refs))
	for _, r := range refs {
		out = append(out, recsapp.ProjectRef{TenantID: r.TenantID, ProjectID: r.ProjectID})
	}
	return out, err
}
