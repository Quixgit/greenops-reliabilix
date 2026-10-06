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
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
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
}

// Assumer returns a Cost Explorer client acting as roleARN with the given ExternalId.
type Assumer func(ctx context.Context, roleARN, externalID string) (CEAPI, error)

type Provider struct {
	Assume Assumer
	// AssumeS3 is needed only for connections that read a FOCUS data export from S3.
	AssumeS3 S3Assumer
	Log      *slog.Logger // optional; nil uses slog.Default()
}

// New builds the provider from the default AWS credential chain (the platform's own identity,
// which only needs sts:AssumeRole on customer roles).
func New(ctx context.Context) (*Provider, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
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
	return &Provider{
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

// classify maps AWS authorization failures to domain.ErrAccessDenied (not retryable).
func classify(err error) error {
	var ae smithy.APIError
	if errors.As(err, &ae) {
		switch ae.ErrorCode() {
		case "AccessDenied", "AccessDeniedException", "UnauthorizedOperation", "InvalidClientTokenId", "UnrecognizedClientException":
			return fmt.Errorf("%w: %s", domain.ErrAccessDenied, ae.ErrorCode())
		}
	}
	return err
}

// Validate proves the role can be assumed with the ExternalId and may read the data source of the connection:
// ce:GetCostAndUsage, or the FOCUS export in S3 when the connection has one.
func (p *Provider) Validate(ctx context.Context, c domain.Connection) error {
	if c.Export != nil {
		return p.validateExport(ctx, c)
	}
	ce, err := p.Assume(ctx, c.CredentialRef, c.ExternalID)
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
	ce, err := p.Assume(ctx, req.Connection.CredentialRef, req.Connection.ExternalID)
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

// fetchAll follows Cost Explorer pagination. The page cap stops a runaway token loop from billing forever.
func fetchAll(ctx context.Context, ce CEAPI, in *costexplorer.GetCostAndUsageInput) ([]types.ResultByTime, error) {
	var results []types.ResultByTime
	for page := 0; page < 200; page++ {
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
	return results, nil
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
