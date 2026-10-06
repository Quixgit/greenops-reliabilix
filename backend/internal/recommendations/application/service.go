// Package application holds the recommendation use-cases: generation and the approval workflow.
package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/ec2spec"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/methodology"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/observability"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/recommendations/domain"
)

type ProjectRef struct{ TenantID, ProjectID string }

// ProjectLister is the cross-tenant fan-out port (implemented by the projects module in the composition root).
type ProjectLister interface {
	ProjectsForJobs(ctx context.Context) ([]ProjectRef, error)
}

type Service struct {
	Repo     domain.Repository
	Projects ProjectLister
	Queue    queue.Enqueuer
	Log      *slog.Logger
	Now      func() time.Time
}

const windowDays = 30

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func fingerprint(project string, r domain.Recommendation) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%s|%s", r.Type, project, r.Provider, r.ServiceName, r.CurrentRegion, r.RecommendedRegion)))
	return hex.EncodeToString(h[:])
}

// Generate analyses the last 30 days of a project and stores new findings (deduplicated by fingerprint;
// decided findings are never recreated or reopened). Returns the number of new recommendations.
func (s Service) Generate(ctx context.Context, tenantID, projectID string) (int, error) {
	now := s.now()
	to := now.Truncate(24 * time.Hour).Add(24 * time.Hour)
	from := to.AddDate(0, 0, -windowDays)

	ws, days, err := s.Repo.Workloads(ctx, tenantID, projectID, from, to)
	if err != nil {
		return 0, err
	}
	allowed, err := s.Repo.AllowedRegions(ctx, tenantID, projectID)
	if err != nil {
		return 0, err
	}
	intensity, err := s.Repo.Intensities(ctx)
	if err != nil {
		return 0, err
	}
	prices := map[string]float64{} // price index is per provider; phase 1 has AWS only
	if p, err := s.Repo.PriceIndex(ctx, "aws"); err == nil {
		prices = p
	}
	recs := domain.GenerateRegionShift(domain.RegionShiftInput{Workloads: ws, WindowDays: windowDays, DataDays: days,
		Intensity: intensity, AllowedRegions: allowed, PriceIndex: prices, Now: now})

	created := 0
	for _, r := range recs {
		r.ProjectID = projectID
		r.Fingerprint = fingerprint(projectID, r)
		ok, err := s.Repo.Insert(ctx, tenantID, r)
		if err != nil {
			return created, err
		}
		if ok {
			created++
		}
	}
	observability.RecommendationsGenerated.Add(float64(created))
	return created, nil
}

// energyModel is the active carbon methodology expressed for single instances, so a rightsizing estimate
// uses exactly the coefficients of the carbon engine.
func energyModel() domain.EnergyModel {
	f := methodology.Current
	return domain.EnergyModel{
		KWhPerVCPUHour: f.EnergyPerUnit["vcpu_hours"], KWhPerGBHour: f.EnergyPerUnit["memory_gb_hours"], PUE: f.PUE,
		ShapeOf: func(t string) (domain.Shape, bool) {
			sp, ok := ec2spec.Parse(t)
			return domain.Shape{VCPU: sp.VCPU, MemoryGB: sp.MemoryGB}, ok
		},
	}
}

// IngestRightsizing turns the provider's findings for a project into recommendations (deduplicated by
// fingerprint; decided findings are never recreated). Returns the number of new recommendations.
func (s Service) IngestRightsizing(ctx context.Context, tenantID, projectID string, findings []domain.RightsizingFinding) (int, error) {
	if len(findings) == 0 {
		return 0, nil
	}
	intensity, err := s.Repo.Intensities(ctx)
	if err != nil {
		return 0, err
	}
	res := domain.GenerateRightsizing(domain.RightsizingInput{
		ProjectID: projectID, Findings: findings, Intensity: intensity, Model: energyModel(), Provider: "aws", Now: s.now()})
	if len(res.Skipped) > 0 {
		s.Log.Info("rightsizing findings skipped", "project", projectID, "skipped", res.Skipped)
	}
	created := 0
	for _, r := range res.Recommendations {
		r.ProjectID = projectID
		ok, err := s.Repo.Insert(ctx, tenantID, r)
		if err != nil {
			return created, err
		}
		if ok {
			created++
		}
	}
	observability.RecommendationsGenerated.Add(float64(created))
	return created, nil
}

// RefreshAll queues a generation job per project across tenants (scheduler).
func (s Service) RefreshAll(ctx context.Context) (int, error) {
	refs, err := s.Projects.ProjectsForJobs(ctx)
	if err != nil {
		return 0, err
	}
	for _, p := range refs {
		if err := s.Queue.Enqueue(ctx, queue.TaskCalcRecommendation, queue.TenantPayload{TenantID: p.TenantID, ProjectID: p.ProjectID}); err != nil {
			s.Log.Error("enqueue recommendations", "project", p.ProjectID, "err", err)
		}
	}
	return len(refs), nil
}

func (s Service) Refresh(ctx context.Context, tenantID, projectID string) error {
	return s.Queue.Enqueue(ctx, queue.TaskCalcRecommendation, queue.TenantPayload{TenantID: tenantID, ProjectID: projectID})
}

func (s Service) List(ctx context.Context, tenantID, status string, project *string, limit int) ([]domain.Recommendation, error) {
	return s.Repo.List(ctx, tenantID, status, project, limit)
}

func (s Service) Get(ctx context.Context, tenantID, id string) (domain.Recommendation, error) {
	return s.Repo.Get(ctx, tenantID, id)
}

// recheck re-validates compliance against the project's CURRENT policy: it may have changed since the
// recommendation was generated. A recommendation whose target is no longer allowed cannot move forward.
func (s Service) recheck(ctx context.Context, tenantID string) func(*domain.Recommendation) error {
	return func(r *domain.Recommendation) error {
		if r.Type != domain.RegionShift {
			return nil
		}
		allowed, err := s.Repo.AllowedRegions(ctx, tenantID, r.ProjectID)
		if err != nil {
			return err
		}
		if !slices.Contains(allowed, r.RecommendedRegion) {
			return domain.ErrComplianceChanged
		}
		return nil
	}
}

// CheckCompliance re-validates a recommendation against the project's current policy without changing it
// (used before planning and approving automation).
func (s Service) CheckCompliance(ctx context.Context, tenantID, id string) error {
	r, err := s.Repo.Get(ctx, tenantID, id)
	if err != nil {
		return err
	}
	return s.recheck(ctx, tenantID)(&r)
}

// Approve, Apply and Dismiss are explicit human decisions by an authenticated user. Phase 1-2 "apply"
// records that the change was made; execution automation (phase 3) will hang off the approved state.
func (s Service) Approve(ctx context.Context, tenantID, id string, who auth.Claims) (domain.Recommendation, error) {
	return s.Repo.Transition(ctx, tenantID, id, domain.Approved, who.Subject, s.recheck(ctx, tenantID))
}

func (s Service) Apply(ctx context.Context, tenantID, id string, who auth.Claims) (domain.Recommendation, error) {
	return s.Repo.Transition(ctx, tenantID, id, domain.Applied, who.Subject, s.recheck(ctx, tenantID))
}

func (s Service) Dismiss(ctx context.Context, tenantID, id string, who auth.Claims) (domain.Recommendation, error) {
	return s.Repo.Transition(ctx, tenantID, id, domain.Dismissed, who.Subject, nil)
}
