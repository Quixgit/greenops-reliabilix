package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	automationdomain "github.com/quixgit/greenops-reliabilix/backend/internal/automation/domain"
	carbonapp "github.com/quixgit/greenops-reliabilix/backend/internal/carbon/application"
	carbon "github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	clouddomain "github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/application"
	recsapp "github.com/quixgit/greenops-reliabilix/backend/internal/recommendations/application"
	recsdomain "github.com/quixgit/greenops-reliabilix/backend/internal/recommendations/domain"
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

// rightsizingSink hands the findings of a cloud connection to the recommendations module.
type rightsizingSink struct{ recs recsapp.Service }

func (a rightsizingSink) SubmitRightsizing(ctx context.Context, tenantID, projectID string, findings []clouddomain.RightsizingFinding) error {
	out := make([]recsdomain.RightsizingFinding, 0, len(findings))
	for _, f := range findings {
		out = append(out, recsdomain.RightsizingFinding{
			ResourceID: f.ResourceID, Region: f.Region, CurrentType: f.CurrentType, TargetType: f.TargetType,
			Current:                 recsdomain.Shape{VCPU: f.Current.VCPU, MemoryGB: f.Current.MemoryGB},
			Target:                  recsdomain.Shape{VCPU: f.Target.VCPU, MemoryGB: f.Target.MemoryGB},
			EstimatedMonthlySavings: f.EstimatedMonthlySavings, Currency: f.Currency,
		})
	}
	_, err := a.recs.IngestRightsizing(ctx, tenantID, projectID, out)
	return err
}

// automationRecs lets the automation module read and update recommendations without importing them.
type automationRecs struct{ recs recsapp.Service }

// translate maps recommendation-domain errors onto the automation domain's, so handlers answer precisely.
func translate(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, recsdomain.ErrNotFound):
		return automationdomain.ErrNotFound
	case errors.Is(err, recsdomain.ErrComplianceChanged):
		return fmt.Errorf("%w: the project's data-residency policy no longer allows this recommendation", automationdomain.ErrInvalidInput)
	}
	return err
}

func (a automationRecs) Get(ctx context.Context, tenantID, id string) (automationdomain.RecommendationInfo, error) {
	r, err := a.recs.Get(ctx, tenantID, id)
	if err != nil {
		return automationdomain.RecommendationInfo{}, translate(err)
	}
	return automationdomain.RecommendationInfo{ID: r.ID, ProjectID: r.ProjectID, Type: string(r.Type), Status: string(r.Status), Title: r.Title,
		CurrentRegion: r.CurrentRegion, RecommendedRegion: r.RecommendedRegion, Details: r.Details}, nil
}

func (a automationRecs) Recheck(ctx context.Context, tenantID, id string) error {
	return translate(a.recs.CheckCompliance(ctx, tenantID, id))
}

// MarkApplied is idempotent: an already applied recommendation is not an error, so a retry after a partial
// failure completes the job.
func (a automationRecs) MarkApplied(ctx context.Context, tenantID, id, actor string) error {
	r, err := a.recs.Get(ctx, tenantID, id)
	if err != nil {
		return translate(err)
	}
	if r.Status == recsdomain.Applied {
		return nil
	}
	_, err = a.recs.Apply(ctx, tenantID, id, auth.Claims{Subject: actor, TenantID: tenantID})
	return translate(err)
}
