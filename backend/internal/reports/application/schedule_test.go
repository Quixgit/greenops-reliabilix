package application

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
	"github.com/quixgit/greenops-reliabilix/backend/internal/reports/domain"
)

type fakeSched struct {
	domain.ScheduleRepository
	members    []string // current members
	recipients []string
	found      bool
}

func (f fakeSched) RunRecipients(context.Context, string, string) ([]string, bool, error) {
	return f.recipients, f.found, nil
}
func (f fakeSched) MemberEmails(_ context.Context, _ string, in []string) ([]string, error) {
	var out []string
	for _, e := range in {
		if slices.Contains(f.members, e) {
			out = append(out, e)
		}
	}
	return out, nil
}
func (f fakeSched) CreateSchedule(_ context.Context, _ string, s domain.Schedule) (domain.Schedule, error) {
	return s, nil
}

type copyRecorder struct {
	notify.Recorder
	copies [][]string
}

func (c *copyRecorder) PublishWithCopy(_ context.Context, _ string, e notify.Event, to []string) error {
	c.Events = append(c.Events, e)
	c.copies = append(c.copies, to)
	return nil
}

func TestScheduledReportIsMailedToCurrentMembersOnly(t *testing.T) {
	n := &copyRecorder{}
	s := Service{Notify: n, Sched: fakeSched{members: []string{"a@example.com"}, recipients: []string{"a@example.com", "left@example.com"}, found: true}}
	s.announceReady(context.Background(), "t", domain.Report{ID: "r"}, notify.Event{Title: "Your carbon report is ready"})
	if len(n.copies) != 1 || !slices.Equal(n.copies[0], []string{"a@example.com"}) {
		t.Fatalf("mailed %v; someone who left the organization must not be written to", n.copies)
	}
	if n.Events[0].Title != "Your scheduled carbon report is ready" {
		t.Errorf("title %q", n.Events[0].Title)
	}
}

func TestAReportNobodyScheduledIsNotMailed(t *testing.T) {
	n := &copyRecorder{}
	Service{Notify: n, Sched: fakeSched{found: false}}.announceReady(context.Background(), "t", domain.Report{ID: "r"}, notify.Event{Title: "Your carbon report is ready"})
	if len(n.copies) != 0 || len(n.Events) != 1 {
		t.Fatalf("copies %v events %d", n.copies, len(n.Events))
	}
}

func TestScheduleNeedsMembersAndANextRun(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	s := Service{Sched: fakeSched{members: []string{"a@example.com"}}, Now: func() time.Time { return now }}
	got, err := s.CreateSchedule(context.Background(), "t", "u", domain.Schedule{Kind: domain.KindSCI, Format: domain.PDF, Cadence: domain.Weekly, Recipients: []string{" A@Example.com "}})
	if err != nil {
		t.Fatal(err)
	}
	if !got.NextRunAt.Equal(time.Date(2026, 10, 12, 6, 0, 0, 0, time.UTC)) || got.Recipients[0] != "a@example.com" {
		t.Errorf("got %+v", got)
	}
	_, err = s.CreateSchedule(context.Background(), "t", "u", domain.Schedule{Kind: domain.KindSCI, Format: domain.PDF, Cadence: domain.Weekly, Recipients: []string{"outsider@example.com"}})
	if !errors.Is(err, domain.ErrRecipientNotMember) {
		t.Fatalf("an outsider must be refused, got %v", err)
	}
}
