// Package aws implements domain.CloudProvider on the AWS Cost Explorer API. Access is by STS
// AssumeRole into the customer's read-only role with a per-connection ExternalId; the platform never
// holds a customer access key. Cost Explorer first (ADR-0008); CUR/FOCUS exports via S3+Athena follow.
package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

// SourceCostExplorer is the focus.RawBatch.Source of payloads produced here.
const SourceCostExplorer = "cost_explorer"

// CEAPI is the slice of the Cost Explorer client the provider needs (faked in tests).
type CEAPI interface {
	GetCostAndUsage(ctx context.Context, in *costexplorer.GetCostAndUsageInput, opts ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageOutput, error)
}

// Assumer returns a Cost Explorer client acting as roleARN with the given ExternalId.
type Assumer func(ctx context.Context, roleARN, externalID string) (CEAPI, error)

type Provider struct{ Assume Assumer }

// New builds the provider from the default AWS credential chain (the platform's own identity,
// which only needs sts:AssumeRole on customer roles).
func New(ctx context.Context) (*Provider, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		return nil, fmt.Errorf("aws: load config: %w", err)
	}
	stsClient := sts.NewFromConfig(cfg)
	return &Provider{Assume: func(_ context.Context, roleARN, externalID string) (CEAPI, error) {
		creds := stscreds.NewAssumeRoleProvider(stsClient, roleARN, func(o *stscreds.AssumeRoleOptions) {
			o.ExternalID = awssdk.String(externalID)
			o.RoleSessionName = "reliabilix-greenops"
			o.Duration = 15 * time.Minute
		})
		c := cfg.Copy()
		c.Credentials = awssdk.NewCredentialsCache(creds)
		return costexplorer.NewFromConfig(c), nil // Cost Explorer is a global service homed in us-east-1
	}}, nil
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

// Validate proves the role can be assumed with the ExternalId and may call ce:GetCostAndUsage.
func (p *Provider) Validate(ctx context.Context, c domain.Connection) error {
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

// GetUsage returns daily cost grouped by SERVICE and REGION for [From, To). It costs one Cost Explorer
// request ($0.01) per result page, so callers keep the window small (incremental syncs).
func (p *Provider) GetUsage(ctx context.Context, req domain.UsageRequest) ([]focus.RawBatch, error) {
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
	var results []types.ResultByTime
	for page := 0; page < 200; page++ { // hard cap: a runaway token loop must not bill indefinitely
		out, err := ce.GetCostAndUsage(ctx, in)
		if err != nil {
			return nil, classify(err)
		}
		results = append(results, out.ResultsByTime...)
		if out.NextPageToken == nil || *out.NextPageToken == "" {
			break
		}
		in.NextPageToken = out.NextPageToken
	}
	payload, err := json.Marshal(map[string]any{"ResultsByTime": results})
	if err != nil {
		return nil, err
	}
	return []focus.RawBatch{{Provider: "aws", Source: SourceCostExplorer, Payload: payload}}, nil
}
