package domain

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type Kind string

const (
	EC2Resize    Kind = "ec2_resize"
	EC2Terminate Kind = "ec2_terminate"
	RegionShift  Kind = "region_shift"
)

type Status string

const (
	Planned    Status = "planned"
	Approved   Status = "approved"
	Completed  Status = "completed"
	Failed     Status = "failed"
	RolledBack Status = "rolled_back"
	Cancelled  Status = "cancelled"
)

type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// Plan is what a human reviews and then applies with their own tooling. Everything in it was built from
// validated identifiers (see planner.go); the platform never runs it.
type Plan struct {
	Summary   string   `json:"summary"`
	Steps     []string `json:"steps"`
	Terraform string   `json:"terraform,omitempty"`
	CLI       []string `json:"cli,omitempty"`
}

type Risk struct {
	Level   RiskLevel `json:"level"`
	Factors []string  `json:"factors"`
}

// Job is a reviewed change plan for one approved recommendation.
type Job struct {
	ID               string     `json:"id"`
	ProjectID        string     `json:"project_id"`
	RecommendationID string     `json:"recommendation_id"`
	Kind             Kind       `json:"kind"`
	Status           Status     `json:"status"`
	Plan             Plan       `json:"plan"`
	Risk             Risk       `json:"risk"`
	RollbackPlan     string     `json:"rollback_plan"`
	CreatedBy        string     `json:"created_by"`
	CreatedAt        time.Time  `json:"created_at"`
	ApprovedBy       *string    `json:"approved_by"`
	ApprovedAt       *time.Time `json:"approved_at"`
	FinishedBy       *string    `json:"finished_by"`
	FinishedAt       *time.Time `json:"finished_at"`
	ResultNote       *string    `json:"result_note"`
}

var (
	ErrInvalidTransition = errors.New("invalid job status transition")
	ErrNotApproved       = errors.New("automation: plan has no human approval")
	ErrNotFound          = errors.New("not found")
	ErrActiveJobExists   = errors.New("a live automation job already exists for this recommendation")
	ErrRecommendationNot = errors.New("recommendation must be approved before it can be planned")
	ErrUnsupported       = errors.New("this recommendation type has no automation plan")
	ErrInvalidInput      = errors.New("invalid automation input")
	ErrNoteRequired      = errors.New("a note is required for this outcome")
)

var allowed = map[Status][]Status{
	Planned:   {Approved, Cancelled},
	Approved:  {Completed, Failed, Cancelled},
	Completed: {RolledBack},
}

// Transition validates a status change. Nothing reaches "completed" without passing "approved".
func (j *Job) Transition(to Status) error {
	for _, s := range allowed[j.Status] {
		if s == to {
			j.Status = to
			return nil
		}
	}
	return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, j.Status, to)
}

// CanExecute enforces the core safety rule: never act without explicit human approval and a rollback plan.
func (j Job) CanExecute() error {
	if j.Status != Approved || j.ApprovedBy == nil || *j.ApprovedBy == "" || j.ApprovedAt == nil {
		return ErrNotApproved
	}
	if j.RollbackPlan == "" {
		return errors.New("automation: rollback plan required")
	}
	return nil
}

// Outcome is what the person who applied the plan reports.
type Outcome string

const (
	OutcomeCompleted  Outcome = "completed"
	OutcomeFailed     Outcome = "failed"
	OutcomeRolledBack Outcome = "rolled_back"
)

// Status returns the job status of an outcome.
func (o Outcome) Status() (Status, bool) {
	switch o {
	case OutcomeCompleted:
		return Completed, true
	case OutcomeFailed:
		return Failed, true
	case OutcomeRolledBack:
		return RolledBack, true
	}
	return "", false
}

// RecommendationInfo is the slice of a recommendation that planning needs (supplied through a port).
type RecommendationInfo struct {
	ID                string
	ProjectID         string
	Type              string // rightsizing | region_shift | ...
	Status            string // open | approved | applied | dismissed
	Title             string
	CurrentRegion     string
	RecommendedRegion string
	Details           map[string]string
}

// Repository is tenant-scoped (explicit tenantID + RLS).
type Repository interface {
	Insert(ctx context.Context, tenantID string, j Job) (Job, error)
	List(ctx context.Context, tenantID string, limit int) ([]Job, error)
	Get(ctx context.Context, tenantID, id string) (Job, error)
	// Transition locks the row, validates the move, runs check against the current state, applies the change
	// and audits it, atomically.
	Transition(ctx context.Context, tenantID, id string, to Status, actor, note string, check func(current *Job) error) (Job, error)
}

func timeNow() time.Time { return time.Now().UTC() }
