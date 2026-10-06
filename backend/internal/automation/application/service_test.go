package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/automation/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
)

type fakeRecs struct {
	info        domain.RecommendationInfo
	recheckErr  error
	applied     []string
	applyErr    error
	recheckCall int
}

func (f *fakeRecs) Get(context.Context, string, string) (domain.RecommendationInfo, error) {
	return f.info, nil
}
func (f *fakeRecs) Recheck(context.Context, string, string) error {
	f.recheckCall++
	return f.recheckErr
}
func (f *fakeRecs) MarkApplied(_ context.Context, _, id, _ string) error {
	if f.applyErr != nil {
		return f.applyErr
	}
	f.applied = append(f.applied, id)
	return nil
}

// memRepo keeps one job in memory and applies the same rules as the PostgreSQL adapter.
type memRepo struct{ job *domain.Job }

func (m *memRepo) Insert(_ context.Context, _ string, j domain.Job) (domain.Job, error) {
	if m.job != nil && (m.job.Status == domain.Planned || m.job.Status == domain.Approved) {
		return domain.Job{}, domain.ErrActiveJobExists
	}
	j.ID, j.Status = "job1", domain.Planned
	m.job = &j
	return j, nil
}
func (m *memRepo) List(context.Context, string, int) ([]domain.Job, error) { return nil, nil }
func (m *memRepo) Get(context.Context, string, string) (domain.Job, error) {
	if m.job == nil {
		return domain.Job{}, domain.ErrNotFound
	}
	return *m.job, nil
}
func (m *memRepo) Transition(_ context.Context, _, _ string, to domain.Status, actor, note string, check func(*domain.Job) error) (domain.Job, error) {
	j := *m.job
	next := j
	if err := next.Transition(to); err != nil {
		return domain.Job{}, err
	}
	if check != nil {
		if err := check(&j); err != nil {
			return domain.Job{}, err
		}
	}
	now := time.Now()
	if to == domain.Approved {
		j.ApprovedBy, j.ApprovedAt = &actor, &now
	} else {
		j.FinishedBy, j.FinishedAt = &actor, &now
		if note != "" {
			j.ResultNote = &note
		}
	}
	j.Status = to
	m.job = &j
	return j, nil
}

var alice = auth.Claims{Subject: "alice", TenantID: "t"}

func approvedRegionShift() *fakeRecs {
	return &fakeRecs{info: domain.RecommendationInfo{ID: "r1", ProjectID: "p", Type: "region_shift", Status: "approved", CurrentRegion: "us-east-1", RecommendedRegion: "eu-north-1"}}
}

func TestPlanRequiresAnApprovedRecommendationAndCurrentCompliance(t *testing.T) {
	recs := approvedRegionShift()
	recs.info.Status = "open"
	s := Service{Repo: &memRepo{}, Recs: recs}
	if _, err := s.Plan(context.Background(), "t", alice, "r1"); !errors.Is(err, domain.ErrRecommendationNot) {
		t.Fatalf("an open recommendation must not be planned: %v", err)
	}
	recs = approvedRegionShift()
	recs.recheckErr = domain.ErrInvalidInput
	s = Service{Repo: &memRepo{}, Recs: recs}
	if _, err := s.Plan(context.Background(), "t", alice, "r1"); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("a recommendation that no longer passes compliance must not be planned: %v", err)
	}
	recs = approvedRegionShift()
	recs.info.Type = "spot_migration"
	if _, err := (Service{Repo: &memRepo{}, Recs: recs}).Plan(context.Background(), "t", alice, "r1"); !errors.Is(err, domain.ErrUnsupported) {
		t.Fatalf("unsupported type: %v", err)
	}
}

func TestFullLifecycleMarksTheRecommendationApplied(t *testing.T) {
	recs := approvedRegionShift()
	repo := &memRepo{}
	s := Service{Repo: repo, Recs: recs}
	ctx := context.Background()
	j, err := s.Plan(ctx, "t", alice, "r1")
	if err != nil || j.Status != domain.Planned || j.CreatedBy != "alice" || j.RollbackPlan == "" {
		t.Fatalf("plan: %+v %v", j, err)
	}
	if _, err := s.Plan(ctx, "t", alice, "r1"); !errors.Is(err, domain.ErrActiveJobExists) {
		t.Fatalf("second live job: %v", err)
	}
	if _, err := s.Report(ctx, "t", "job1", alice, domain.OutcomeCompleted, ""); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("completing an unapproved job: %v", err)
	}
	if _, err := s.Approve(ctx, "t", "job1", alice); err != nil {
		t.Fatal(err)
	}
	if recs.recheckCall < 2 {
		t.Errorf("compliance must be re-checked at planning and at approval, got %d checks", recs.recheckCall)
	}
	done, err := s.Report(ctx, "t", "job1", alice, domain.OutcomeCompleted, "  done  ")
	if err != nil || done.Status != domain.Completed || done.ResultNote == nil || *done.ResultNote != "done" {
		t.Fatalf("complete: %+v %v", done, err)
	}
	if len(recs.applied) != 1 || recs.applied[0] != "r1" {
		t.Fatalf("recommendation not marked applied: %v", recs.applied)
	}
}

func TestReportValidation(t *testing.T) {
	s := Service{Repo: &memRepo{}, Recs: approvedRegionShift()}
	ctx := context.Background()
	if _, err := s.Plan(ctx, "t", alice, "r1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Approve(ctx, "t", "job1", alice); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Report(ctx, "t", "job1", alice, "approved", ""); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("an outcome must not set arbitrary statuses: %v", err)
	}
	if _, err := s.Report(ctx, "t", "job1", alice, domain.OutcomeFailed, " "); !errors.Is(err, domain.ErrNoteRequired) {
		t.Errorf("a failure needs a note: %v", err)
	}
	if _, err := s.Report(ctx, "t", "job1", alice, domain.OutcomeFailed, string(make([]byte, 2001))); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("overlong note: %v", err)
	}
}

func TestMarkAppliedFailureKeepsTheJobApproved(t *testing.T) {
	recs := approvedRegionShift()
	repo := &memRepo{}
	s := Service{Repo: repo, Recs: recs}
	ctx := context.Background()
	if _, err := s.Plan(ctx, "t", alice, "r1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Approve(ctx, "t", "job1", alice); err != nil {
		t.Fatal(err)
	}
	recs.applyErr = errors.New("db down")
	if _, err := s.Report(ctx, "t", "job1", alice, domain.OutcomeCompleted, ""); err == nil {
		t.Fatal("the error must surface")
	}
	if repo.job.Status != domain.Approved {
		t.Fatalf("the job must stay approved so the report can be retried, got %s", repo.job.Status)
	}
	recs.applyErr = nil
	if j, err := s.Report(ctx, "t", "job1", alice, domain.OutcomeCompleted, ""); err != nil || j.Status != domain.Completed {
		t.Fatalf("retry: %+v %v", j, err)
	}
}

func TestCancel(t *testing.T) {
	s := Service{Repo: &memRepo{}, Recs: approvedRegionShift()}
	ctx := context.Background()
	if _, err := s.Plan(ctx, "t", alice, "r1"); err != nil {
		t.Fatal(err)
	}
	if j, err := s.Cancel(ctx, "t", "job1", alice); err != nil || j.Status != domain.Cancelled {
		t.Fatalf("cancel: %+v %v", j, err)
	}
	if _, err := s.Approve(ctx, "t", "job1", alice); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("a cancelled plan cannot be approved: %v", err)
	}
	if _, err := s.Plan(ctx, "t", alice, "r1"); err != nil {
		t.Fatalf("a cancelled job frees the recommendation for a new plan: %v", err)
	}
}
