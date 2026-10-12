// Package aws implements domain.CloudProvider on the AWS Cost Explorer API. Access is by STS
// AssumeRole into the customer's read-only role with a per-connection ExternalId; the platform never
// holds a customer access key. Cost Explorer first (ADR-0008); CUR/FOCUS exports via S3+Athena follow.
package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

// Sources of the payloads produced here (focus.RawBatch.Source).
const (
	SourceCostExplorer = "cost_explorer"
	// SourceCostExplorerUsage carries measured consumption (EC2 running hours, S3 storage) so that the
	// carbon engine can use usage-based energy instead of a spend-based estimate.
	SourceCostExplorerUsage = "cost_explorer_usage"
)

// Usage-type groups that expose a physical quantity. EC2 hours are grouped by instance type, which the
// normalizer converts into vCPU-hours and memory GB-hours.
var (
	ec2HoursGroups  = []string{"EC2: Running Hours"}
	s3StorageGroups = []string{
		"S3: Storage - Standard", "S3: Storage - Standard Infrequent Access", "S3: Storage - One Zone Infrequent Access",
		"S3: Storage - Intelligent Tiering Frequent Access", "S3: Storage - Glacier Instant Retrieval",
	}
)

// CEAPI is the slice of the Cost Explorer client the provider needs (faked in tests).
type CEAPI interface {
	GetCostAndUsage(ctx context.Context, in *costexplorer.GetCostAndUsageInput, opts ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageOutput, error)
	GetRightsizingRecommendation(ctx context.Context, in *costexplorer.GetRightsizingRecommendationInput, opts ...func(*costexplorer.Options)) (*costexplorer.GetRightsizingRecommendationOutput, error)
	GetSavingsPlansUtilization(ctx context.Context, in *costexplorer.GetSavingsPlansUtilizationInput, opts ...func(*costexplorer.Options)) (*costexplorer.GetSavingsPlansUtilizationOutput, error)
	GetSavingsPlansCoverage(ctx context.Context, in *costexplorer.GetSavingsPlansCoverageInput, opts ...func(*costexplorer.Options)) (*costexplorer.GetSavingsPlansCoverageOutput, error)
	GetReservationUtilization(ctx context.Context, in *costexplorer.GetReservationUtilizationInput, opts ...func(*costexplorer.Options)) (*costexplorer.GetReservationUtilizationOutput, error)
	GetReservationCoverage(ctx context.Context, in *costexplorer.GetReservationCoverageInput, opts ...func(*costexplorer.Options)) (*costexplorer.GetReservationCoverageOutput, error)
}

// Assumer returns a Cost Explorer client acting as roleARN with the given ExternalId.
type Assumer func(ctx context.Context, roleARN, externalID string) (CEAPI, error)

// KeyAssumer returns a Cost Explorer client that signs with a customer's IAM access key.
type KeyAssumer func(ctx context.Context, accessKeyID, secretAccessKey string) (CEAPI, error)

// KeyS3 is the S3 counterpart of KeyAssumer.
type KeyS3 func(ctx context.Context, accessKeyID, secretAccessKey, region string) (S3API, error)

type Provider struct {
	Assume Assumer
	// AssumeS3 is needed only for connections that read a FOCUS data export from S3.
	AssumeS3 S3Assumer
	// Key, KeyS3 and OpenSecret serve connections made with the customer's own IAM access key (credential_ref
	// "awskey://<id>"): the secret access key is stored sealed and opened per call.
	Key        KeyAssumer
	KeyS3      KeyS3
	OpenSecret func(domain.Connection) (string, error)
	Log        *slog.Logger // optional; nil uses slog.Default()
}

// New builds the provider from the default AWS credential chain (the platform's own identity,
// which only needs sts:AssumeRole on customer roles).
func New(ctx context.Context, openSecret func(domain.Connection) (string, error)) (*Provider, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"),
		// Cost Explorer throttles bursts (a few requests a second per account): the SDK backs off and retries
		// before the job fails and is retried later with a larger delay.
		config.WithRetryMaxAttempts(6))
	if err != nil {
		return nil, fmt.Errorf("aws: load config: %w", err)
	}
	stsClient := sts.NewFromConfig(cfg)
	// credentials returns a cached provider that assumes the customer's role with the connection's ExternalId.
	credentials := func(roleARN, externalID string) *awssdk.CredentialsCache {
		return awssdk.NewCredentialsCache(stscreds.NewAssumeRoleProvider(stsClient, roleARN, func(o *stscreds.AssumeRoleOptions) {
			o.ExternalID = awssdk.String(externalID)
			o.RoleSessionName = "reliabilix-greenops"
			o.Duration = 15 * time.Minute
		}))
	}
	static := func(id, secret, region string) awssdk.Config {
		c := cfg.Copy()
		c.Credentials = awssdk.NewCredentialsCache(awscreds.NewStaticCredentialsProvider(id, secret, ""))
		if region != "" {
			c.Region = region
		}
		return c
	}
	return &Provider{
		OpenSecret: openSecret,
		Key: func(_ context.Context, id, secret string) (CEAPI, error) {
			return costexplorer.NewFromConfig(static(id, secret, "")), nil
		},
		KeyS3: func(_ context.Context, id, secret, region string) (S3API, error) {
			return s3.NewFromConfig(static(id, secret, region)), nil
		},
		Assume: func(_ context.Context, roleARN, externalID string) (CEAPI, error) {
			c := cfg.Copy()
			c.Credentials = credentials(roleARN, externalID)
			return costexplorer.NewFromConfig(c), nil // Cost Explorer is a global service homed in us-east-1
		},
		AssumeS3: func(_ context.Context, roleARN, externalID, region string) (S3API, error) {
			c := cfg.Copy()
			c.Credentials = credentials(roleARN, externalID)
			c.Region = region // the bucket's region (validated against a strict pattern)
			return s3.NewFromConfig(c), nil
		},
	}, nil
}

func (*Provider) Provider() domain.ProviderType { return domain.AWS }

// keyOf returns the access key id and the opened secret access key of an access-key connection.
func (p *Provider) keyOf(c domain.Connection) (id, secret string, err error) {
	if id, err = domain.ParseAWSKeyRef(c.CredentialRef); err != nil {
		return "", "", err
	}
	if p.OpenSecret == nil {
		return "", "", errors.New("aws: no way to open the connection's secret")
	}
	if secret, err = p.OpenSecret(c); err != nil {
		// The API sealed the secret and the worker opens it: they must share the sealing key.
		return "", "", domain.PublicError{Message: "The stored secret access key cannot be opened by this service: every service (api, worker, scheduler) must run with the same NOTIFICATIONS_SECRET_KEY. Reconnect the account if the key was changed."}
	}
	return id, secret, nil
}

// ce returns the Cost Explorer client of a connection: the customer's access key when it has one, otherwise the
// role the platform assumes.
func (p *Provider) ce(ctx context.Context, c domain.Connection) (CEAPI, error) {
	if domain.IsAWSKeyRef(c.CredentialRef) {
		id, secret, err := p.keyOf(c)
		if err != nil {
			return nil, err
		}
		return p.Key(ctx, id, secret)
	}
	return p.Assume(ctx, c.CredentialRef, c.ExternalID)
}

// s3 returns the S3 client for the connection's FOCUS export, authenticated like ce.
func (p *Provider) s3(ctx context.Context, c domain.Connection, region string) (S3API, error) {
	if domain.IsAWSKeyRef(c.CredentialRef) {
		if p.KeyS3 == nil {
			return nil, errors.New("aws: S3 access is not configured")
		}
		id, secret, err := p.keyOf(c)
		if err != nil {
			return nil, err
		}
		return p.KeyS3(ctx, id, secret, region)
	}
	if p.AssumeS3 == nil {
		return nil, errors.New("aws: S3 access is not configured")
	}
	return p.AssumeS3(ctx, c.CredentialRef, c.ExternalID, region)
}

// classify turns AWS failures into errors the customer can act on. Authorization failures are domain.ErrAccessDenied
// (not retryable) and say which step was refused; throttling and other API errors carry AWS's error code and are
// retried. Messages from AWS are not passed on (they carry ARNs and account ids); the error code is.
func classify(err error) error {
	var ae smithy.APIError
	if !errors.As(err, &ae) {
		if err != nil && strings.Contains(err.Error(), "failed to refresh cached credentials") {
			// No answer from AWS at all: the platform's own identity (the one that assumes customer roles) has no
			// usable credentials. Not the customer's doing, so say so instead of "retried automatically".
			return domain.PublicError{Message: "The platform's own AWS identity could not be used to assume your role (platform configuration, not your account); the sync is retried automatically."}
		}
		return err
	}
	code, msg := ae.ErrorCode(), strings.ToLower(ae.ErrorMessage())
	switch code {
	case "AccessDenied", "AccessDeniedException", "UnauthorizedOperation", "InvalidClientTokenId", "UnrecognizedClientException":
		switch {
		case strings.Contains(msg, "sts:assumerole"):
			return domain.PublicError{Cause: domain.ErrAccessDenied, Message: "AWS refused to assume the role (" + code + "): check the role's trust policy (the platform's account and the ExternalId shown in the setup steps) and the role ARN."}
		case strings.Contains(msg, "cost explorer"):
			return domain.PublicError{Cause: domain.ErrAccessDenied, Message: "Cost Explorer is not enabled for this AWS account or not allowed for the role: enable Cost Explorer in the Billing console (the first data can take up to 24 hours), then verify again."}
		case strings.Contains(msg, "s3:") || code == "AccessDenied":
			return domain.PublicError{Cause: domain.ErrAccessDenied, Message: "AWS denied the request (" + code + "): check that the role has the read permissions of the setup steps (ce:GetCostAndUsage, or s3:GetObject and s3:ListBucket on the export bucket)."}
		}
		return domain.PublicError{Cause: domain.ErrAccessDenied, Message: "AWS denied the request (" + code + "): check the role and its read permissions from the setup steps."}
	case "ThrottlingException", "Throttling", "LimitExceededException", "RequestLimitExceeded", "TooManyRequestsException", "SlowDown", "RequestThrottled":
		return domain.PublicError{Message: "AWS is throttling requests (" + code + "); the sync is retried automatically."}
	case "DataUnavailableException":
		return domain.PublicError{Message: "Cost Explorer has no data for this account yet (it can take up to 24 hours after it was enabled); the sync is retried automatically."}
	case "BillExpirationException":
		return domain.PublicError{Message: "Cost Explorer could not return data for this account (BillExpirationException): the bill of the period is being finalized; the sync is retried automatically."}
	case "NoSuchBucket":
		return domain.PublicError{Cause: domain.ErrExportNotFound, Message: "The export bucket does not exist (NoSuchBucket): check the bucket name and region of the connection's export."}
	}
	return domain.PublicError{Message: "AWS answered " + code + "; the sync is retried automatically."}
}

// Validate proves the role can be assumed with the ExternalId and may read the data source of the connection:
// ce:GetCostAndUsage, or the FOCUS export in S3 when the connection has one.
func (p *Provider) Validate(ctx context.Context, c domain.Connection) error {
	if c.Export != nil {
		return p.validateExport(ctx, c)
	}
	ce, err := p.ce(ctx, c)
	if err != nil {
		return classify(err)
	}
	end := time.Now().UTC().Truncate(24 * time.Hour)
	_, err = ce.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{
		TimePeriod:  &types.DateInterval{Start: awssdk.String(end.AddDate(0, 0, -2).Format(time.DateOnly)), End: awssdk.String(end.AddDate(0, 0, -1).Format(time.DateOnly))},
		Granularity: types.GranularityDaily,
		Metrics:     []string{"UnblendedCost"},
	})
	return classify(err)
}

// GetUsage returns the raw billing data for [From, To): the customer's FOCUS export when the connection has
// one, otherwise daily Cost Explorer cost grouped by SERVICE and REGION. Cost Explorer costs one Cost Explorer
// request ($0.01) per result page, so callers keep the window small (incremental syncs).
func (p *Provider) GetUsage(ctx context.Context, req domain.UsageRequest) ([]focus.RawBatch, error) {
	if req.Connection.Export != nil {
		// The export is the complete, invoice-level source: Cost Explorer would only add duplicate cost lines.
		b, err := p.exportBatch(ctx, req.Connection, req.From, req.To)
		if err != nil {
			return nil, err
		}
		return []focus.RawBatch{b}, nil
	}
	ce, err := p.ce(ctx, req.Connection)
	if err != nil {
		return nil, classify(err)
	}
	in := &costexplorer.GetCostAndUsageInput{
		TimePeriod:  &types.DateInterval{Start: awssdk.String(req.From.Format(time.DateOnly)), End: awssdk.String(req.To.Format(time.DateOnly))},
		Granularity: types.GranularityDaily,
		Metrics:     []string{"UnblendedCost", "AmortizedCost"},
		GroupBy: []types.GroupDefinition{
			{Type: types.GroupDefinitionTypeDimension, Key: awssdk.String("SERVICE")},
			{Type: types.GroupDefinitionTypeDimension, Key: awssdk.String("REGION")},
		},
	}
	results, err := fetchAll(ctx, ce, in)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(map[string]any{"ResultsByTime": results})
	if err != nil {
		return nil, err
	}
	batches := []focus.RawBatch{{Provider: "aws", Source: SourceCostExplorer, Payload: payload}}

	// Measured usage is an enrichment: when it cannot be fetched the cost data still flows and the carbon
	// engine falls back to the cost-based estimate, so a failure here must not block the sync.
	if usage, err := p.usageBatch(ctx, ce, req); err != nil {
		if errors.Is(err, domain.ErrAccessDenied) {
			return nil, err
		}
		p.logger().Warn("aws usage quantities unavailable, carbon stays cost-based", "err", err)
	} else {
		batches = append(batches, usage)
	}
	return batches, nil
}

func (p *Provider) logger() *slog.Logger {
	if p.Log != nil {
		return p.Log
	}
	return slog.Default()
}

// maxCEPages stops a runaway token loop from billing forever. Reaching it is an ERROR, never a quiet end of the
// data: a truncated result would advance the sync cursor past costs that were never read.
const maxCEPages = 200

var errTooManyPages = errors.New("aws: Cost Explorer returned too many pages; the window is too large for one request")

// fetchAll follows Cost Explorer pagination.
func fetchAll(ctx context.Context, ce CEAPI, in *costexplorer.GetCostAndUsageInput) ([]types.ResultByTime, error) {
	var results []types.ResultByTime
	for page := 0; page < maxCEPages; page++ {
		out, err := ce.GetCostAndUsage(ctx, in)
		if err != nil {
			return nil, classify(err)
		}
		results = append(results, out.ResultsByTime...)
		if out.NextPageToken == nil || *out.NextPageToken == "" {
			return results, nil
		}
		in.NextPageToken = out.NextPageToken
	}
	return nil, errTooManyPages
}

// GetControlTotals implements domain.ControlProvider: the day totals as Cost Explorer itself reports them
// (no grouping), final days only, to be compared with what was ingested. Connections that read a FOCUS export
// are not checked this way (the export is the invoice-level source).
func (p *Provider) GetControlTotals(ctx context.Context, req domain.UsageRequest) ([]focus.ControlTotal, error) {
	if req.Connection.Export != nil {
		return nil, nil
	}
	ce, err := p.ce(ctx, req.Connection)
	if err != nil {
		return nil, classify(err)
	}
	results, err := fetchAll(ctx, ce, &costexplorer.GetCostAndUsageInput{
		TimePeriod:  &types.DateInterval{Start: awssdk.String(req.From.Format(time.DateOnly)), End: awssdk.String(req.To.Format(time.DateOnly))},
		Granularity: types.GranularityDaily,
		Metrics:     []string{"UnblendedCost"},
	})
	if err != nil {
		return nil, err
	}
	var out []focus.ControlTotal
	for _, r := range results {
		if r.Estimated || r.TimePeriod == nil || r.TimePeriod.Start == nil {
			continue // the ingestion skips estimated days too: they become final one sync later
		}
		m, ok := r.Total["UnblendedCost"]
		if !ok || m.Amount == nil || m.Unit == nil {
			continue
		}
		amount, err := strconv.ParseFloat(*m.Amount, 64)
		if err != nil || math.IsNaN(amount) || math.IsInf(amount, 0) {
			return nil, fmt.Errorf("aws: unreadable control total %q", *m.Amount)
		}
		out = append(out, focus.ControlTotal{Day: *r.TimePeriod.Start, Currency: strings.ToUpper(*m.Unit), Billed: amount})
	}
	return out, nil
}

// usageBatch fetches daily UsageQuantity for EC2 running hours (by instance type) and S3 storage.
func (p *Provider) usageBatch(ctx context.Context, ce CEAPI, req domain.UsageRequest) (focus.RawBatch, error) {
	period := &types.DateInterval{Start: awssdk.String(req.From.Format(time.DateOnly)), End: awssdk.String(req.To.Format(time.DateOnly))}
	query := func(groups []string, extra ...string) *costexplorer.GetCostAndUsageInput {
		gb := []types.GroupDefinition{{Type: types.GroupDefinitionTypeDimension, Key: awssdk.String("REGION")}}
		for _, k := range extra {
			gb = append(gb, types.GroupDefinition{Type: types.GroupDefinitionTypeDimension, Key: awssdk.String(k)})
		}
		return &costexplorer.GetCostAndUsageInput{
			TimePeriod: period, Granularity: types.GranularityDaily, Metrics: []string{"UsageQuantity"}, GroupBy: gb,
			Filter: &types.Expression{Dimensions: &types.DimensionValues{Key: types.DimensionUsageTypeGroup, Values: groups}},
		}
	}
	ec2, err := fetchAll(ctx, ce, query(ec2HoursGroups, "INSTANCE_TYPE"))
	if err != nil {
		return focus.RawBatch{}, err
	}
	s3, err := fetchAll(ctx, ce, query(s3StorageGroups))
	if err != nil {
		return focus.RawBatch{}, err
	}
	payload, err := json.Marshal(map[string]any{"EC2Hours": ec2, "S3Storage": s3})
	if err != nil {
		return focus.RawBatch{}, err
	}
	return focus.RawBatch{Provider: "aws", Source: SourceCostExplorerUsage, Payload: payload}, nil
}
