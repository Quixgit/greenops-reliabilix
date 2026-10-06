package aws

import (
	"context"
	"errors"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
)

func ec2(itype, region, vcpu, mem string) *types.ResourceDetails {
	return &types.ResourceDetails{EC2ResourceDetails: &types.EC2ResourceDetails{
		InstanceType: awssdk.String(itype), Region: awssdk.String(region), Vcpu: awssdk.String(vcpu), Memory: awssdk.String(mem)}}
}

func current(id, itype, region, vcpu, mem string) *types.CurrentInstance {
	return &types.CurrentInstance{ResourceId: awssdk.String(id), ResourceDetails: ec2(itype, region, vcpu, mem)}
}

func provider(ce *fakeCE) *Provider {
	return &Provider{Assume: func(context.Context, string, string) (CEAPI, error) { return ce, nil }}
}

func TestGetRightsizingMapsModifyAndTerminate(t *testing.T) {
	ce := &fakeCE{rs: []*costexplorer.GetRightsizingRecommendationOutput{{
		NextPageToken: awssdk.String("next"),
		RightsizingRecommendations: []types.RightsizingRecommendation{{
			RightsizingType: types.RightsizingTypeModify, CurrentInstance: current("i-1", "m5.2xlarge", "eu-central-1", "8", "32"),
			ModifyRecommendationDetail: &types.ModifyRecommendationDetail{TargetInstances: []types.TargetInstance{
				{EstimatedMonthlySavings: awssdk.String("10"), CurrencyCode: awssdk.String("USD"), ResourceDetails: ec2("m5.xlarge", "eu-central-1", "4", "16")},
				{EstimatedMonthlySavings: awssdk.String("55.5"), CurrencyCode: awssdk.String("USD"), ResourceDetails: ec2("t3.large", "eu-central-1", "2", "8")},
			}},
		}},
	}, {
		RightsizingRecommendations: []types.RightsizingRecommendation{{
			RightsizingType: types.RightsizingTypeTerminate, CurrentInstance: current("i-2", "c5.large", "us-east-1", "2", "4"),
			TerminateRecommendationDetail: &types.TerminateRecommendationDetail{EstimatedMonthlySavings: awssdk.String("30"), CurrencyCode: awssdk.String("USD")},
		}},
	}}}
	got, err := provider(ce).GetRightsizing(context.Background(), conn())
	if err != nil {
		t.Fatal(err)
	}
	if len(ce.rsCalls) != 2 || ce.rsCalls[1].NextPageToken == nil || *ce.rsCalls[1].NextPageToken != "next" {
		t.Fatalf("pagination not followed: %d calls", len(ce.rsCalls))
	}
	if *ce.rsCalls[0].Service != "AmazonEC2" {
		t.Errorf("service = %q", *ce.rsCalls[0].Service)
	}
	if len(got) != 2 {
		t.Fatalf("got %d findings, want 2", len(got))
	}
	m := got[0]
	if m.Action != domain.RightsizingModify || m.TargetType != "t3.large" || m.EstimatedMonthlySavings != 55.5 ||
		m.Current.VCPU != 8 || m.Target.MemoryGB != 8 || m.Region != "eu-central-1" || m.Currency != "USD" {
		t.Errorf("modify finding wrong (the target with the highest savings must win): %+v", m)
	}
	if x := got[1]; x.Action != domain.RightsizingTerminate || x.TargetType != "" || x.EstimatedMonthlySavings != 30 || x.ResourceID != "i-2" {
		t.Errorf("terminate finding wrong: %+v", x)
	}
}

func TestIncompleteFindingsAreDropped(t *testing.T) {
	ce := &fakeCE{rs: []*costexplorer.GetRightsizingRecommendationOutput{{RightsizingRecommendations: []types.RightsizingRecommendation{
		{RightsizingType: types.RightsizingTypeModify, CurrentInstance: current("i-1", "m5.large", "EU (Frankfurt)", "2", "8")}, // region is a display name
		{RightsizingType: types.RightsizingTypeModify, CurrentInstance: current("i-2", "m5.large", "eu-west-1", "2", "8")},      // no target details
		{RightsizingType: types.RightsizingTypeTerminate, CurrentInstance: current("i-3", "m5.large", "eu-west-1", "2", "8")},   // no terminate details
		{RightsizingType: types.RightsizingTypeTerminate},                                                                       // no instance
	}}}}
	got, err := provider(ce).GetRightsizing(context.Background(), conn())
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; want nothing", got, err)
	}
}

func TestRightsizingAccessDeniedIsClassified(t *testing.T) {
	_, err := provider(&fakeCE{err: apiErr{"AccessDeniedException"}}).GetRightsizing(context.Background(), conn())
	if !errors.Is(err, domain.ErrAccessDenied) {
		t.Fatalf("err = %v", err)
	}
}
