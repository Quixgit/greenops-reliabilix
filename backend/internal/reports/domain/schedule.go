package domain

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"
)

// Cadence is how often a schedule makes its report.
type Cadence string

const (
	Weekly  Cadence = "weekly"
	Monthly Cadence = "monthly"
)

// MaxRecipients bounds the people one schedule tells.
const MaxRecipients = 20

var (
	ErrInvalidSchedule = errors.New("invalid report schedule")
	// ErrRecipientNotMember: an address is not a member of the organization. Reports are not sent to outsiders.
	ErrRecipientNotMember = errors.New("every recipient must be a member of the organization")
)

// runHour is when (UTC) a schedule fires: early morning, after the overnight calculations (03:30 recalculation).
const runHour = 6

// Schedule asks for the same report again every week or month.
type Schedule struct {
	ID         string     `json:"id"`
	ProjectID  *string    `json:"project_id"`
	Kind       Kind       `json:"kind"`
	Format     Format     `json:"format"`
	Cadence    Cadence    `json:"cadence"`
	Recipients []string   `json:"recipients"`
	Enabled    bool       `json:"enabled"`
	NextRunAt  time.Time  `json:"next_run_at"`
	LastRunAt  *time.Time `json:"last_run_at"`
	CreatedBy  string     `json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
}

// Normalize lower-cases and de-duplicates the recipients.
func (s *Schedule) Normalize() {
	seen := map[string]bool{}
	out := []string{}
	for _, r := range s.Recipients {
		r = strings.ToLower(strings.TrimSpace(r))
		if r != "" && !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	s.Recipients = out
}

// Validate rejects unknown kinds, formats and cadences and malformed or too many recipients.
func (s Schedule) Validate() error {
	if err := (Report{Kind: s.Kind, Format: s.Format}).Validate(); err != nil {
		return ErrInvalidSchedule
	}
	if s.Cadence != Weekly && s.Cadence != Monthly {
		return ErrInvalidSchedule
	}
	if len(s.Recipients) > MaxRecipients {
		return ErrInvalidSchedule
	}
	for _, r := range s.Recipients {
		a, err := mail.ParseAddress(r)
		if err != nil || a.Address != r || strings.Count(r, "@") != 1 || len(r) > 254 {
			return ErrInvalidSchedule
		}
	}
	return nil
}

// NextRun is the first run strictly after t: Monday for a weekly schedule, the 1st for a monthly one, at 06:00 UTC.
func NextRun(c Cadence, t time.Time) time.Time {
	t = t.UTC()
	day := time.Date(t.Year(), t.Month(), t.Day(), runHour, 0, 0, 0, time.UTC)
	if c == Monthly {
		first := time.Date(t.Year(), t.Month(), 1, runHour, 0, 0, 0, time.UTC)
		if !first.After(t) {
			first = first.AddDate(0, 1, 0)
		}
		return first
	}
	for !(day.Weekday() == time.Monday && day.After(t)) {
		day = day.AddDate(0, 0, 1)
	}
	return day
}

// Period is the span a run at runAt reports on, end inclusive: the seven days before a weekly run, the calendar month before a monthly one.
func Period(c Cadence, runAt time.Time) (start, end time.Time) {
	d := time.Date(runAt.UTC().Year(), runAt.UTC().Month(), runAt.UTC().Day(), 0, 0, 0, 0, time.UTC)
	if c == Monthly {
		first := time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC)
		return first.AddDate(0, -1, 0), first.AddDate(0, 0, -1)
	}
	end = d.AddDate(0, 0, -1)
	return end.AddDate(0, 0, -6), end
}

// ScheduleRepository stores schedules (the report rows they make go through Repository).
type ScheduleRepository interface {
	CreateSchedule(ctx context.Context, tenantID string, s Schedule) (Schedule, error)
	ListSchedules(ctx context.Context, tenantID string) ([]Schedule, error)
	SetScheduleEnabled(ctx context.Context, tenantID, id string, enabled bool, next time.Time) (Schedule, error)
	DeleteSchedule(ctx context.Context, tenantID, id string) error
	// Due lists schedules whose time has come in every organization (tenant id, schedule id).
	DueSchedules(ctx context.Context, now time.Time) ([][2]string, error)
	// Claim moves a due schedule to its next run and returns it, or false when someone else did already or it is off.
	ClaimSchedule(ctx context.Context, tenantID, id string, now time.Time) (Schedule, bool, error)
	// Link records that a report was made by a schedule.
	LinkSchedule(ctx context.Context, tenantID, reportID, scheduleID string) error
	// RunRecipients returns the schedule's recipients for a report it made (found=false: not a scheduled report).
	RunRecipients(ctx context.Context, tenantID, reportID string) (recipients []string, found bool, err error)
	// MemberEmails returns those of the addresses that belong to a current member of the organization.
	MemberEmails(ctx context.Context, tenantID string, emails []string) ([]string, error)
}

// Headline is the few figures a scheduled report's e-mail carries in its body, so that the message is a digest and not only a link.
type Headline struct {
	Currency       string
	Cost, PrevCost float64
	CarbonKg       float64
	PrevCarbonKg   float64
	TopService     string
	TopServiceCost float64
}

// HeadlineReader reads the headline of a window and of the equally long one before it. Optional on the service.
type HeadlineReader interface {
	Headline(ctx context.Context, tenantID string, project *string, from, to time.Time) (Headline, error)
}
