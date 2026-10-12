// Package domain defines reports: what can be requested and the neutral table form every format renders from.
package domain

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
)

type Kind string

const (
	KindCarbon Kind = "carbon"
	KindSCI    Kind = "sci"
	KindFinops Kind = "finops"
	// KindSustainability is the executive summary: totals, data quality, top services and regions, recommendations.
	KindSustainability Kind = "sustainability"
	// KindGHG is the GHG Protocol Scope 3 (category 1, cloud services) inventory laid out for ESRS E1-6 / CSRD reporting.
	KindGHG Kind = "ghg"
)

type Format string

const (
	CSV  Format = "csv"
	JSON Format = "json"
	PDF  Format = "pdf"
)

type Status string

const (
	Pending Status = "pending"
	Ready   Status = "ready"
	Failed  Status = "failed"
)

var (
	ErrInvalidReport = errors.New("invalid report request")
	ErrNotFound      = errors.New("not found")
	ErrNotReady      = errors.New("report is not ready")
	// ErrFormatNotAllowed: the organization's settings do not allow this format.
	ErrFormatNotAllowed = errors.New("this report format is not enabled for your organization")
)

// Report is the metadata row; the file itself lives in object storage.
type Report struct {
	ID          string     `json:"id"`
	ProjectID   *string    `json:"project_id"`
	Kind        Kind       `json:"kind"`
	Format      Format     `json:"format"`
	PeriodStart time.Time  `json:"period_start"`
	PeriodEnd   time.Time  `json:"period_end"` // inclusive
	Status      Status     `json:"status"`
	ObjectKey   *string    `json:"-"`
	Error       *string    `json:"error"`
	RequestedBy string     `json:"requested_by"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at"`
	// Clouds narrows the report to some of the clouds of its scope (aws, azure, gcp, ...); empty means every cloud.
	Clouds []string `json:"clouds"`
}

var cloudNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

// NormalizeClouds lower-cases and de-duplicates a cloud list; ok is false for a malformed name or more than eight clouds.
func NormalizeClouds(in []string) (out []string, ok bool) {
	out = []string{}
	seen := map[string]bool{}
	for _, c := range in {
		c = strings.ToLower(strings.TrimSpace(c))
		if c == "" || seen[c] {
			continue
		}
		if !cloudNameRe.MatchString(c) {
			return nil, false
		}
		seen[c] = true
		out = append(out, c)
	}
	return out, len(out) <= 8
}

// Validate rejects unknown kinds/formats and periods that are empty, inverted or longer than 400 days.
func (r Report) Validate() error {
	if _, ok := NormalizeClouds(r.Clouds); !ok {
		return ErrInvalidReport
	}
	switch r.Kind {
	case KindCarbon, KindSCI, KindFinops, KindSustainability, KindGHG:
	default:
		return ErrInvalidReport
	}
	switch r.Format {
	case CSV, JSON, PDF:
	default:
		return ErrInvalidReport
	}
	if r.PeriodEnd.Before(r.PeriodStart) || r.PeriodEnd.Sub(r.PeriodStart) > 400*24*time.Hour {
		return ErrInvalidReport
	}
	return nil
}

// Ext and ContentType describe the rendered file.
func (f Format) Ext() string { return string(f) }

func (f Format) ContentType() string {
	switch f {
	case CSV:
		return "text/csv; charset=utf-8"
	case JSON:
		return "application/json"
	default:
		return "application/pdf"
	}
}

// Table is the neutral, format-independent content of a report.
type Table struct {
	Title  string `json:"title"`
	Period string `json:"period"`
	// Organization and Scope say for whom and for what the report was made ("All projects" or a project's name). They
	// frame a document (the PDF's title block and footer); the data formats carry them as a note.
	Organization string     `json:"organization,omitempty"`
	Scope        string     `json:"scope,omitempty"`
	Notes        []string   `json:"notes"`
	Headers      []string   `json:"headers"`
	Rows         [][]string `json:"rows"`
}

// Repository is tenant-scoped (explicit tenantID + RLS). Dataset builders are read-only views of other schemas.
type Repository interface {
	Insert(ctx context.Context, tenantID string, r Report) (Report, error)
	List(ctx context.Context, tenantID string) ([]Report, error)
	Get(ctx context.Context, tenantID, id string) (Report, error)
	SetReady(ctx context.Context, tenantID, id, objectKey string) error
	SetFailed(ctx context.Context, tenantID, id, message string) error
	CarbonTable(ctx context.Context, tenantID string, project *string, from, to time.Time) (Table, error)
	FinopsTable(ctx context.Context, tenantID string, project *string, from, to time.Time) (Table, error)
	SCITable(ctx context.Context, tenantID string, project *string, from, to time.Time) (Table, error)
	SustainabilityTable(ctx context.Context, tenantID string, project *string, from, to time.Time) (Table, error)
	GHGTable(ctx context.Context, tenantID string, project *string, from, to time.Time) (Table, error)
	// Scope names the organization and the project (empty project: all of them) a report is about.
	Scope(ctx context.Context, tenantID string, project *string) (org, projectName string, err error)
	// Delete removes the metadata row and returns what it was (the caller removes the stored file).
	Delete(ctx context.Context, tenantID, id string) (Report, error)
}
