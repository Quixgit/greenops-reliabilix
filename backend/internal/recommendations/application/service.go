// Package application holds the recommendation use-cases: generation and the approval workflow.
package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/ec2spec"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/methodology"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/observability"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/pricing"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
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
	// Notify is optional: the measured result of a change is announced (best effort, never fails the run).
	Notify notify.Notifier
	Log    *slog.Logger
	Now    func() time.Time
	// Settings is the organization settings engine (optional: catalog defaults without it).
	Settings settings.Reader
}

const windowDays = 30

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

// fingerprint identifies a finding by what it is about (the kind, the project, the service and where it runs), not by what it
// proposes: the proposed region can change from run to run as the grid does, and that is the same finding with a better
// answer, not a new one.
func fingerprint(project string, r domain.Recommendation) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%s", r.Type, project, r.Provider, r.ServiceName, r.CurrentRegion)))
	return hex.EncodeToString(h[:])
}

// PriceSource fetches the relative region price index (an AWS price list adapter in production, a stand-in in tests).
type PriceSource interface {
	// Provider is the cloud the index is for ("aws", "azure", "gcp"); Basis names the basket it was built from.
	Provider() string
	Basis() string
	BuildIndex(ctx context.Context, regions []string) ([]PriceEntry, map[string]error, error)
}

// PriceEntry is one region's relative price and how many basket instances it rests on.
type PriceEntry struct {
	Region string
	Idx    float64
	N      int
}

// RefreshPriceIndex stores the relative on-demand price per region. A region that could not be read keeps its previous
// value (stale beats absent), and the first failure is returned after the others were stored.
func (s Service) RefreshPriceIndex(ctx context.Context, src PriceSource, regions []string) (stored int, err error) {
	idx, errs, err := src.BuildIndex(ctx, regions)
	if err != nil {
		return 0, err
	}
	source := src.Basis() + ":" + s.now().Format("2006-01-02")
	for _, e := range idx {
		if err := s.Repo.SavePriceIndex(ctx, src.Provider(), e.Region, e.Idx, source); err != nil {
			return stored, err
		}
		stored++
	}
	for r, e := range errs {
		if errors.Is(e, pricing.ErrNotOffered) {
			// the service is not sold there: no index, no estimate, and nothing to retry
			s.Log.Info("price index: not offered", "provider", src.Provider(), "region", r)
			continue
		}
		s.Log.Warn("price index", "region", r, "err", e)
		if err == nil {
			err = e
		}
	}
	return stored, err
}

// maxLatencyMs is the organization's limit on the round trip a recommended move may add.
func (s Service) maxLatencyMs(ctx context.Context, tenantID string) int {
	var r settings.Reader = settings.Defaults{}
	if s.Settings != nil {
		r = s.Settings
	}
	return int(r.Int(ctx, tenantID, settings.KeyRecMaxLatencyMs))
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
	// One index per cloud; region ids of the clouds never collide, so one map serves every workload (a ratio is only ever taken
	// between two regions of the same cloud).
	prices := map[string]float64{}
	for _, cloud := range []string{"aws", "azure", "gcp"} {
		if p, err := s.Repo.PriceIndex(ctx, cloud); err == nil {
			for r, v := range p {
				prices[r] = v
			}
		}
	}
	services := map[string]map[string]float64{}
	for _, fam := range []string{"azure:postgresql", "azure:redis", "aws:rds", "aws:elasticache", "gcp:cloudsql", "gcp:memorystore"} {
		if p, err := s.Repo.PriceIndex(ctx, fam); err == nil && len(p) > 0 {
			services[fam] = p
		}
	}
	recs := domain.GenerateRegionShift(domain.RegionShiftInput{Workloads: ws, WindowDays: windowDays, DataDays: days,
		Intensity: intensity, AllowedRegions: allowed, PriceIndex: prices, ServiceIndex: services, MaxLatencyMs: s.maxLatencyMs(ctx, tenantID), Now: now})

	// Kubernetes: smaller requests for workloads that use a fraction of them (ADR-0031). Two weeks of measurements.
	if usage, err := s.Repo.K8sUsage(ctx, tenantID, projectID, now.AddDate(0, 0, -14), now); err != nil {
		return 0, err
	} else {
		recs = append(recs, domain.GenerateK8sRightsizing(projectID, usage, now)...)
	}

	created := 0
	var keep []string
	for _, r := range recs {
		r.ProjectID = projectID
		r.Fingerprint = fingerprint(projectID, r)
		if r.Type == domain.RegionShift {
			keep = append(keep, r.Fingerprint)
		}
		ok, err := s.Repo.Insert(ctx, tenantID, r)
		if err != nil {
			return created, err
		}
		if ok {
			created++
		}
	}
	// With nothing to judge by (no workloads, no grid data) an empty result means "cannot tell", not "nothing is worth doing":
	// the open ones stay as they are. Otherwise an open move the analysis no longer finds is obsolete and is closed.
	if len(ws) > 0 && len(intensity) > 0 {
		if _, err := s.Repo.RetireStale(ctx, tenantID, projectID, keep); err != nil {
			return created, err
		}
	}
	observability.RecommendationsGenerated.Add(float64(created))
	return created, nil
}

// VerifyApplied measures applied recommendations of a project whose after window is complete (ADR-0025) and
// stores the result. A change that cannot be measured yet is left alone and tried again on the next run;
// after GiveUpDays it is closed as inconclusive so it does not wait forever. Returns the number verified.
func (s Service) VerifyApplied(ctx context.Context, tenantID, projectID string) (int, error) {
	now := s.now()
	recs, err := s.Repo.AppliedAwaitingVerification(ctx, tenantID, projectID)
	if err != nil {
		return 0, err
	}
	done := 0
	for _, r := range recs {
		if r.Type == domain.K8sRightsizing {
			continue // it claims no carbon or cost saving, so there is nothing to measure afterwards
		}
		if r.AppliedAt == nil || !domain.Due(*r.AppliedAt, now) {
			continue
		}
		bFrom, bTo, aFrom, aTo := domain.Windows(*r.AppliedAt)
		before, cur, err := s.Repo.DailySeries(ctx, tenantID, projectID, r.ServiceName, bFrom, bTo)
		if err != nil {
			return done, err
		}
		after, curAfter, err := s.Repo.DailySeries(ctx, tenantID, projectID, r.ServiceName, aFrom, aTo)
		if err != nil {
			return done, err
		}
		v := domain.Verify(r, before, after, now)
		if v.Outcome == domain.Inconclusive && !domain.Expired(*r.AppliedAt, now) {
			continue // data may still arrive (late billing export)
		}
		if cur != "" && curAfter != "" && cur != curAfter { // a currency change would make the cost delta meaningless
			v.MeasuredCostMonth, v.ExpectedCostMonth = nil, nil
			v.Note += " Cost not compared: the billing currency changed."
		}
		if cur == "" {
			cur = curAfter
		}
		if err := s.Repo.SaveVerification(ctx, tenantID, r.ID, v, cur); err != nil {
			return done, err
		}
		done++
		s.announce(ctx, tenantID, r, v)
	}
	return done, nil
}

func (s Service) announce(ctx context.Context, tenantID string, r domain.Recommendation, v domain.Verification) {
	if s.Notify == nil {
		return
	}
	body := "Not enough data around the change to measure it."
	if v.MeasuredCarbonKgMonth != nil {
		body = fmt.Sprintf("Measured %.1f kg CO2e/month (expected %.1f).", *v.MeasuredCarbonKgMonth, v.ExpectedCarbonKgMonth)
	}
	_ = s.Notify.Publish(ctx, tenantID, notify.Event{Type: notify.TypeRecommendationVerified, Severity: notify.Info,
		Title: fmt.Sprintf("Result of \"%s\": %s", r.Title, strings.ReplaceAll(string(v.Outcome), "_", " ")), Body: body,
		Path: "/recommendations", DedupeKey: "recommendation:" + r.ID + ":verified",
		Attrs: map[string]string{"recommendation_id": r.ID, "outcome": string(v.Outcome)}})
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
