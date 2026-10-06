package aws

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
)

// regionCode matches a region id such as eu-central-1. Anything else (for example a display name) is not
// trusted as a region, because the carbon estimate depends on it.
var regionCode = regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-\d$`)

// maxRightsizingPages bounds pagination; at 100 findings per page this is far beyond a realistic account.
const maxRightsizingPages = 20

// GetRightsizing lists AWS's own EC2 rightsizing findings (Cost Explorer). It needs the
// ce:GetRightsizingRecommendation permission and the "Rightsizing recommendations" opt-in in the customer's
// Cost Explorer preferences; without them the call is denied or returns nothing.
func (p *Provider) GetRightsizing(ctx context.Context, c domain.Connection) ([]domain.RightsizingFinding, error) {
	ce, err := p.Assume(ctx, c.CredentialRef, c.ExternalID)
	if err != nil {
		return nil, classify(err)
	}
	in := &costexplorer.GetRightsizingRecommendationInput{
		Service:  awssdk.String("AmazonEC2"),
		PageSize: 100,
		Configuration: &types.RightsizingRecommendationConfiguration{
			BenefitsConsidered:   true,
			RecommendationTarget: types.RecommendationTargetCrossInstanceFamily,
		},
	}
	var out []domain.RightsizingFinding
	for page := 0; page < maxRightsizingPages; page++ {
		res, err := ce.GetRightsizingRecommendation(ctx, in)
		if err != nil {
			return nil, classify(err)
		}
		for _, r := range res.RightsizingRecommendations {
			if f, ok := finding(r); ok {
				out = append(out, f)
			}
		}
		if res.NextPageToken == nil || *res.NextPageToken == "" {
			break
		}
		in.NextPageToken = res.NextPageToken
	}
	return out, nil
}

// finding converts one SDK recommendation; incomplete ones are dropped rather than guessed.
func finding(r types.RightsizingRecommendation) (domain.RightsizingFinding, bool) {
	cur := r.CurrentInstance
	if cur == nil || cur.ResourceId == nil || cur.ResourceDetails == nil || cur.ResourceDetails.EC2ResourceDetails == nil {
		return domain.RightsizingFinding{}, false
	}
	d := cur.ResourceDetails.EC2ResourceDetails
	region := strings.ToLower(deref(d.Region))
	if !regionCode.MatchString(region) || deref(d.InstanceType) == "" {
		return domain.RightsizingFinding{}, false
	}
	f := domain.RightsizingFinding{
		ResourceID: *cur.ResourceId, Region: region, CurrentType: deref(d.InstanceType), Current: shape(d),
	}
	switch r.RightsizingType {
	case types.RightsizingTypeTerminate:
		t := r.TerminateRecommendationDetail
		if t == nil {
			return domain.RightsizingFinding{}, false
		}
		f.Action = domain.RightsizingTerminate
		f.EstimatedMonthlySavings, f.Currency = number(t.EstimatedMonthlySavings), strings.ToUpper(deref(t.CurrencyCode))
	case types.RightsizingTypeModify:
		best, ok := bestTarget(r.ModifyRecommendationDetail)
		if !ok {
			return domain.RightsizingFinding{}, false
		}
		td := best.ResourceDetails.EC2ResourceDetails
		f.Action, f.TargetType, f.Target = domain.RightsizingModify, deref(td.InstanceType), shape(td)
		f.EstimatedMonthlySavings, f.Currency = number(best.EstimatedMonthlySavings), strings.ToUpper(deref(best.CurrencyCode))
	default:
		return domain.RightsizingFinding{}, false
	}
	return f, true
}

// bestTarget picks the target with the highest estimated savings that names an instance type.
func bestTarget(m *types.ModifyRecommendationDetail) (types.TargetInstance, bool) {
	var best types.TargetInstance
	found := false
	if m == nil {
		return best, false
	}
	for _, t := range m.TargetInstances {
		if t.ResourceDetails == nil || t.ResourceDetails.EC2ResourceDetails == nil || deref(t.ResourceDetails.EC2ResourceDetails.InstanceType) == "" {
			continue
		}
		if !found || number(t.EstimatedMonthlySavings) > number(best.EstimatedMonthlySavings) {
			best, found = t, true
		}
	}
	return best, found
}

func shape(d *types.EC2ResourceDetails) domain.InstanceShape {
	return domain.InstanceShape{VCPU: number(d.Vcpu), MemoryGB: number(d.Memory)}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// number parses an AWS decimal string; anything unparsable or negative is 0 (not reported).
func number(s *string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(deref(s)), 64)
	if err != nil || v < 0 {
		return 0
	}
	return v
}
