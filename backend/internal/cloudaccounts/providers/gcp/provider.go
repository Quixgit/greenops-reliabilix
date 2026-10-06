// Package gcp implements domain.CloudProvider on the customer's Cloud Billing export in BigQuery. Billing data
// is read from the export table (no per-usage API calls), with the platform's own Google identity: the customer
// only grants that service account read access to the export dataset and labels the dataset with the
// connection's ExternalId-style token. The platform stores no customer credential (ADR-0018).
package gcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/oauth2/google"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

// SourceBigQuery is the focus.RawBatch.Source of the aggregate built from a billing export.
const SourceBigQuery = "bigquery_export"

const (
	// defaultMaxBytesBilled caps what one query may bill (BigQuery refuses to run it otherwise). Billing exports
	// are small; the cap only protects the platform from a runaway or hostile table.
	defaultMaxBytesBilled = 20 << 30
	maxRows               = 500_000
	// ownershipLabelValue is the value of the dataset label whose KEY is the connection's ExternalId.
	ownershipLabelValue = "1"
)

type Provider struct {
	BQ             BigQuery
	JobProject     string // the platform's own project: queries run (and are billed) here
	MaxBytesBilled int64
	MaxRows        int // 0 = default; a result larger than this is rejected, not truncated
}

// New builds the provider from Application Default Credentials (the platform's own Google identity). jobProject
// is the project in which queries run; empty uses the project of the credentials.
func New(ctx context.Context, jobProject string) (*Provider, error) {
	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/bigquery")
	if err != nil {
		return nil, fmt.Errorf("gcp: default credentials: %w", err)
	}
	if jobProject == "" {
		jobProject = creds.ProjectID
	}
	if jobProject == "" {
		return nil, errors.New("gcp: no job project: set PLATFORM_GCP_PROJECT")
	}
	return &Provider{BQ: &RESTClient{HTTP: oauthClient(ctx, creds)}, JobProject: jobProject}, nil
}

func (*Provider) Provider() domain.ProviderType { return domain.GCP }

func (p *Provider) maxRows() int {
	if p.MaxRows > 0 {
		return p.MaxRows
	}
	return maxRows
}

func (p *Provider) maxBytes() int64 {
	if p.MaxBytesBilled > 0 {
		return p.MaxBytesBilled
	}
	return defaultMaxBytesBilled
}

// table parses the connection's export reference (validated on creation, re-validated before every query).
func table(c domain.Connection) (domain.BigQueryTable, error) {
	t, err := domain.ParseBigQueryRef(c.CredentialRef)
	if err != nil {
		return t, err
	}
	return t, nil
}

// proveOwnership requires the dataset label "<external_id>: 1". Only someone who can write to the dataset can
// set it, which stops one tenant from pointing a connection at another customer's export.
func (p *Provider) proveOwnership(ctx context.Context, t domain.BigQueryTable, externalID string) error {
	labels, err := p.BQ.DatasetLabels(ctx, t.Project, t.Dataset)
	if err != nil {
		return err
	}
	if externalID == "" || labels[externalID] != ownershipLabelValue {
		return domain.ErrOwnershipNotProven
	}
	return nil
}

// Validate proves ownership, then dry-runs the real query: this checks that the table exists, that it has the
// expected schema and that the platform may read it, without scanning or billing anything.
func (p *Provider) Validate(ctx context.Context, c domain.Connection) error {
	t, err := table(c)
	if err != nil {
		return err
	}
	if err := p.proveOwnership(ctx, t, c.ExternalID); err != nil {
		return err
	}
	now := time.Now().UTC().Truncate(24 * time.Hour)
	q := p.usageQuery(t, c.AccountRef, now.AddDate(0, 0, -2), now)
	q.DryRun = true
	_, err = p.BQ.Query(ctx, q)
	return err
}

// GetUsage reads [From, To) of the project's costs, aggregated by day, service, SKU, region and currency.
func (p *Provider) GetUsage(ctx context.Context, req domain.UsageRequest) ([]focus.RawBatch, error) {
	c := req.Connection
	t, err := table(c)
	if err != nil {
		return nil, err
	}
	if err := p.proveOwnership(ctx, t, c.ExternalID); err != nil {
		return nil, err
	}
	rows, err := p.BQ.Query(ctx, p.usageQuery(t, c.AccountRef, req.From, req.To))
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(map[string]any{"rows": toLines(rows)})
	if err != nil {
		return nil, err
	}
	return []focus.RawBatch{{Provider: "gcp", Source: SourceBigQuery, Payload: payload}}, nil
}

// line is one aggregated billing line; the ingestion module parses the same JSON (the two never share code).
type line struct {
	Day      string `json:"day"`
	Service  string `json:"service"`
	SKU      string `json:"sku"`
	Region   string `json:"region"`
	Currency string `json:"currency"`
	Unit     string `json:"pricing_unit"`
	Cost     string `json:"cost"`
	Credits  string `json:"credits"`
	Quantity string `json:"quantity"`
}

func toLines(rows [][]string) []line {
	out := make([]line, 0, len(rows))
	for _, r := range rows {
		out = append(out, line{r[0], r[1], r[2], r[3], r[4], r[5], r[6], r[7], r[8]})
	}
	return out
}

// usageQuery builds the aggregation. Table identifiers were validated by domain.ParseBigQueryRef (letters, digits
// and underscores only) so quoting them is safe; every value is a query parameter. The project filter keeps a
// billing-account-wide export to the connection's project; the byte cap bounds the cost of a scan.
func (p *Provider) usageQuery(t domain.BigQueryTable, project string, from, to time.Time) Query {
	sql := fmt.Sprintf("SELECT\n"+
		"  FORMAT_DATE('%%Y-%%m-%%d', DATE(usage_start_time, 'UTC')) AS day,\n"+
		"  IFNULL(service.description, '') AS service,\n"+
		"  IFNULL(sku.description, '') AS sku,\n"+
		"  COALESCE(location.region, location.location, 'global') AS region,\n"+
		"  currency,\n"+
		"  IFNULL(usage.pricing_unit, '') AS pricing_unit,\n"+
		"  CAST(SUM(cost) AS STRING) AS cost,\n"+
		"  CAST(SUM(IFNULL((SELECT SUM(c.amount) FROM UNNEST(credits) AS c), 0)) AS STRING) AS credits,\n"+
		"  CAST(SUM(IFNULL(usage.amount_in_pricing_units, 0)) AS STRING) AS quantity\n"+
		"FROM `%s.%s.%s`\n"+
		"WHERE project.id = @project_id AND usage_start_time >= @from_ts AND usage_start_time < @to_ts\n"+
		"GROUP BY day, service, sku, region, currency, pricing_unit", t.Project, t.Dataset, t.Table)
	return Query{
		JobProject: p.JobProject, SQL: sql, MaxBytesBilled: p.maxBytes(), MaxRows: p.maxRows(), ExpectedColumnsN: 9,
		Params: map[string]Param{
			"project_id": {Type: "STRING", Value: project},
			"from_ts":    {Type: "TIMESTAMP", Value: from.UTC().Format(time.RFC3339)},
			"to_ts":      {Type: "TIMESTAMP", Value: to.UTC().Format(time.RFC3339)},
		},
	}
}

// Preflight checks the platform's own side (used by `admin doctor`): credentials work, BigQuery is enabled in the
// job project and the identity may create jobs there. It is a dry run, so it reads and bills nothing.
func (p *Provider) Preflight(ctx context.Context) error {
	_, err := p.BQ.Query(ctx, Query{JobProject: p.JobProject, SQL: "SELECT 1", DryRun: true, MaxBytesBilled: 1 << 20})
	return err
}
