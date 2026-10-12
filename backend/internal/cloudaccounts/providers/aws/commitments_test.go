package aws

import (
	"context"
	"errors"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/aws/smithy-go"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
)

var (
	win0 = time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	win1 = time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	cnx  = domain.Connection{ID: "c1", AccountRef: "123456789012", CredentialRef: "arn:aws:iam::123456789012:role/ReliabilixReadOnly"}
)

func str(v string) *string { return awssdk.String(v) }

func awsWith(ce *fakeCE) *Provider {
	return &Provider{Assume: func(context.Context, string, string) (CEAPI, error) { return ce, nil }}
}

func TestSavingsPlansAreReadWithUtilizationAndCoverage(t *testing.T) {
	ce := &fakeCE{
		spUtil: &costexplorer.GetSavingsPlansUtilizationOutput{Total: &types.SavingsPlansUtilizationAggregates{
			Utilization: &types.SavingsPlansUtilization{TotalCommitment: str("1000.00"), UsedCommitment: str("820.00"), UnusedCommitment: str("180.00"), UtilizationPercentage: str("82.04")},
			Savings:     &types.SavingsPlansSavings{NetSavings: str("340.5"), OnDemandCostEquivalent: str("1160.5")},
		}},
		spCov: &costexplorer.GetSavingsPlansCoverageOutput{SavingsPlansCoverages: []types.SavingsPlansCoverage{
			{Coverage: &types.SavingsPlansCoverageData{SpendCoveredBySavingsPlans: str("600"), TotalCost: str("1000")}},
			{Coverage: &types.SavingsPlansCoverageData{SpendCoveredBySavingsPlans: str("200"), TotalCost: str("1000")}},
		}},
	}
	got, err := awsWith(ce).GetCommitments(context.Background(), cnx, win0, win1)
	if err != nil || len(got) != 1 {
		t.Fatalf("%v %+v", err, got)
	}
	sp := got[0]
	if sp.Kind != domain.CommitmentSavingsPlans || *sp.UtilizationPct != 82 || *sp.CoveragePct != 40 || *sp.Unused != 180 || *sp.NetSavings != 340.5 {
		t.Errorf("%+v", sp)
	}
	if sp.Account != "123456789012" || sp.ConnectionID != "c1" {
		t.Errorf("the snapshot must name its connection: %+v", sp)
	}
}

func TestReservationsAreReadFromTheirOwnTotals(t *testing.T) {
	ce := &fakeCE{
		riUtil: &costexplorer.GetReservationUtilizationOutput{Total: &types.ReservationAggregates{PurchasedHours: str("7200"), UtilizationPercentage: str("91.3"),
			AmortizedRecurringFee: str("400"), AmortizedUpfrontFee: str("100"), RICostForUnusedHours: str("43.5"), NetRISavings: str("210")}},
		riCov: &costexplorer.GetReservationCoverageOutput{Total: &types.Coverage{CoverageHours: &types.CoverageHours{CoverageHoursPercentage: str("55.5")}}},
	}
	got, err := awsWith(ce).GetCommitments(context.Background(), cnx, win0, win1)
	if err != nil || len(got) != 1 || got[0].Kind != domain.CommitmentReservations {
		t.Fatalf("%v %+v", err, got)
	}
	r := got[0]
	if *r.UtilizationPct != 91.3 || *r.CoveragePct != 55.5 || *r.Commitment != 500 || *r.Unused != 43.5 {
		t.Errorf("%+v", r)
	}
}

func TestAnAccountWithoutCommitmentsHasNoSnapshots(t *testing.T) {
	got, err := awsWith(&fakeCE{}).GetCommitments(context.Background(), cnx, win0, win1)
	if err != nil || len(got) != 0 {
		t.Fatalf("an empty answer is not zero utilization: %v %+v", err, got)
	}
	// the cloud's "no data" error is also just nothing to read
	ce := &fakeCE{spUtilErr: &smithy.GenericAPIError{Code: "DataUnavailableException", Message: "no data"}}
	if got, err := awsWith(ce).GetCommitments(context.Background(), cnx, win0, win1); err != nil || len(got) != 0 {
		t.Fatalf("%v %+v", err, got)
	}
}

func TestMissingPermissionIsAccessDeniedNotEmpty(t *testing.T) {
	ce := &fakeCE{spUtilErr: &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "not authorized to perform: ce:GetSavingsPlansUtilization"}}
	_, err := awsWith(ce).GetCommitments(context.Background(), cnx, win0, win1)
	if !errors.Is(err, domain.ErrAccessDenied) {
		t.Fatalf("a role without the permission must not look like an account without commitments: %v", err)
	}
}
