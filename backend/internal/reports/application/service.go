// Package application holds the report use-cases: request, generate (job) and download.
package application

import (
	"bytes"
	"context"
	"fmt"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
	"io"
	"slices"
	"time"

	"github.com/hibiken/asynq"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/storage"
	"github.com/quixgit/greenops-reliabilix/backend/internal/reports/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/reports/render"
)

type Service struct {
	Repo  domain.Repository
	Store storage.Store
	Queue queue.Enqueuer
	// Notify is optional: when set, finished and failed reports are announced (best effort, never fails a report).
	Notify notify.Notifier
	// Settings is optional: it supplies the formats the organization allows (catalog default: all).
	Settings settings.Reader
	Now      func() time.Time
}

func (s Service) announce(ctx context.Context, tenantID string, e notify.Event) {
	if s.Notify != nil {
		_ = s.Notify.Publish(ctx, tenantID, e) // a notification problem must not turn a good report into a failed one
	}
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s Service) Create(ctx context.Context, tenantID, requestedBy string, r domain.Report) (domain.Report, error) {
	if err := r.Validate(); err != nil {
		return domain.Report{}, err
	}
	if s.Settings != nil && !slices.Contains(s.Settings.Strings(ctx, tenantID, settings.KeyReportFormats), string(r.Format)) {
		return domain.Report{}, domain.ErrFormatNotAllowed
	}
	r.RequestedBy = requestedBy
	created, err := s.Repo.Insert(ctx, tenantID, r)
	if err != nil {
		return domain.Report{}, err
	}
	if err := s.Queue.Enqueue(ctx, queue.TaskGenerateReport, queue.TenantPayload{TenantID: tenantID, RefID: created.ID}, asynq.MaxRetry(3)); err != nil {
		_ = s.Repo.SetFailed(ctx, tenantID, created.ID, "could not queue the report")
		return domain.Report{}, err
	}
	return created, nil
}

func (s Service) List(ctx context.Context, tenantID string) ([]domain.Report, error) {
	return s.Repo.List(ctx, tenantID)
}
func (s Service) Get(ctx context.Context, tenantID, id string) (domain.Report, error) {
	return s.Repo.Get(ctx, tenantID, id)
}

// table builds the dataset of a report. The period end is inclusive, the queries take an exclusive end.
func (s Service) table(ctx context.Context, tenantID string, r domain.Report) (domain.Table, error) {
	from, to := r.PeriodStart, r.PeriodEnd.AddDate(0, 0, 1)
	switch r.Kind {
	case domain.KindCarbon:
		return s.Repo.CarbonTable(ctx, tenantID, r.ProjectID, from, to)
	case domain.KindSCI:
		return s.Repo.SCITable(ctx, tenantID, r.ProjectID, from, to)
	case domain.KindSustainability:
		return s.Repo.SustainabilityTable(ctx, tenantID, r.ProjectID, from, to)
	default:
		return s.Repo.FinopsTable(ctx, tenantID, r.ProjectID, from, to)
	}
}

// Generate is the reports:generate job: build the dataset, render, store, mark ready.
func (s Service) Generate(ctx context.Context, tenantID, id string) error {
	r, err := s.Repo.Get(ctx, tenantID, id)
	if err != nil {
		if err == domain.ErrNotFound {
			return nil
		}
		return err
	}
	if r.Status == domain.Ready {
		return nil // already done (job redelivery)
	}
	tbl, err := s.table(ctx, tenantID, r)
	if err != nil {
		return err
	}
	// Whom the report is for; a failure to name them must not lose the report.
	if org, scope, err := s.Repo.Scope(ctx, tenantID, r.ProjectID); err == nil {
		tbl.Organization, tbl.Scope = org, scope
	}
	b, err := render.Render(tbl, r.Format, s.now())
	if err != nil {
		return err
	}
	key, err := storage.Key(tenantID, "reports", id+"."+r.Format.Ext())
	if err != nil {
		return err
	}
	if err := s.Store.Put(ctx, key, bytes.NewReader(b)); err != nil {
		return fmt.Errorf("store report: %w", err)
	}
	if err := s.Repo.SetReady(ctx, tenantID, id, key); err != nil {
		return err
	}
	s.announce(ctx, tenantID, notify.Event{Type: notify.TypeReportReady, Severity: notify.Info,
		Title: fmt.Sprintf("Your %s report is ready", r.Kind),
		Body:  fmt.Sprintf("Period %s to %s, format %s.", r.PeriodStart.Format(time.DateOnly), r.PeriodEnd.Format(time.DateOnly), r.Format),
		Path:  "/reports", DedupeKey: "report:" + id + ":ready",
		Attrs: map[string]string{"report_id": id, "kind": string(r.Kind), "format": string(r.Format)}})
	return nil
}

// MarkFailed records a terminal failure after retries are exhausted.
func (s Service) MarkFailed(ctx context.Context, tenantID, id string) error {
	if err := s.Repo.SetFailed(ctx, tenantID, id, "Report generation failed. Try again or contact support."); err != nil {
		return err
	}
	s.announce(ctx, tenantID, notify.Event{Type: notify.TypeReportFailed, Severity: notify.Warning,
		Title: "A report could not be generated", Body: "Open Reports to try again.", Path: "/reports",
		DedupeKey: "report:" + id + ":failed", Attrs: map[string]string{"report_id": id}})
	return nil
}

// Download is a stored report file opened for streaming.
type Download struct {
	Body        io.ReadCloser
	ContentType string
	Filename    string
}

// Open returns the file of a ready report. The tenant check happens in the repository (RLS), and the object
// key was built from the tenant id at generation time, so one tenant can never read another's file.
func (s Service) Open(ctx context.Context, tenantID, id string) (Download, error) {
	r, err := s.Repo.Get(ctx, tenantID, id)
	if err != nil {
		return Download{}, err
	}
	if r.Status != domain.Ready || r.ObjectKey == nil {
		return Download{}, domain.ErrNotReady
	}
	body, err := s.Store.Get(ctx, *r.ObjectKey)
	if err != nil {
		return Download{}, err
	}
	return Download{Body: body, ContentType: r.Format.ContentType(),
		Filename: fmt.Sprintf("reliabilix-%s-%s_%s.%s", r.Kind, r.PeriodStart.Format(time.DateOnly), r.PeriodEnd.Format(time.DateOnly), r.Format.Ext())}, nil
}

// PreviewRows is how many rows a preview returns.
const PreviewRows = 8

// Preview is the head of a report's table, built the same way the report itself is, without storing anything.
type Preview struct {
	Title     string     `json:"title"`
	Period    string     `json:"period"`
	Notes     []string   `json:"notes"`
	Headers   []string   `json:"headers"`
	Rows      [][]string `json:"rows"`
	TotalRows int        `json:"total_rows"`
}

// Preview lets a person see what a report would contain before asking for it.
func (s Service) Preview(ctx context.Context, tenantID string, r domain.Report) (Preview, error) {
	r.Format = domain.CSV // the format is irrelevant to the content; validate the rest
	if err := r.Validate(); err != nil {
		return Preview{}, err
	}
	t, err := s.table(ctx, tenantID, r)
	if err != nil {
		return Preview{}, err
	}
	rows := t.Rows
	if len(rows) > PreviewRows {
		rows = rows[:PreviewRows]
	}
	if rows == nil {
		rows = [][]string{}
	}
	return Preview{Title: t.Title, Period: t.Period, Notes: t.Notes, Headers: t.Headers, Rows: rows, TotalRows: len(t.Rows)}, nil
}

// Formats lists the formats the organization allows (all of them when no settings reader is wired).
func (s Service) Formats(ctx context.Context, tenantID string) []string {
	all := []string{string(domain.PDF), string(domain.CSV), string(domain.JSON)}
	if s.Settings == nil {
		return all
	}
	allowed := s.Settings.Strings(ctx, tenantID, settings.KeyReportFormats)
	out := []string{}
	for _, f := range all {
		if slices.Contains(allowed, f) {
			out = append(out, f)
		}
	}
	return out
}

// Delete removes a report and its stored file. A file that is already gone is not an error.
func (s Service) Delete(ctx context.Context, tenantID, id string) error {
	r, err := s.Repo.Delete(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if r.ObjectKey != nil {
		_ = s.Store.Delete(ctx, *r.ObjectKey) // best effort: the row is the source of truth
	}
	return nil
}
