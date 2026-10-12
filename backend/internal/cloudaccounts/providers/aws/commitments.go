package aws

import (
	"context"
	"math"
	"strconv"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
)

// GetCommitments reads the Savings Plans and reservations of the account from Cost Explorer: utilization (how much of the commitment was
// used) and coverage (how much of the eligible spend it covered) over [from, to). It needs ce:GetSavingsPlansUtilization,
// ce:GetSavingsPlansCoverage, ce:GetReservationUtilization and ce:GetReservationCoverage; a role without them gets an access-denied
// error the caller treats as "not permitted", and an account without commitments gets an empty answer, not zeros.
func (p *Provider) GetCommitments(ctx context.Context, c domain.Connection, from, to time.Time) ([]domain.CommitmentSnapshot, error) {
	ce, err := p.ce(ctx, c)
	if err != nil {
		return nil, classify(err)
	}
	period := &types.DateInterval{Start: awssdk.String(from.UTC().Format(time.DateOnly)), End: awssdk.String(to.UTC().Format(time.DateOnly))}
	var out []domain.CommitmentSnapshot
	base := func(kind string) domain.CommitmentSnapshot {
		return domain.CommitmentSnapshot{ConnectionID: c.ID, Provider: "aws", Account: c.AccountRef, Kind: kind, PeriodFrom: from, PeriodTo: to, Currency: "USD"}
	}

	// Savings Plans
	sp := base(domain.CommitmentSavingsPlans)
	found := false
	if u, err := ce.GetSavingsPlansUtilization(ctx, &costexplorer.GetSavingsPlansUtilizationInput{TimePeriod: period}); err != nil {
		if !noCommitments(err) {
			return nil, classify(err)
		}
	} else if u.Total != nil && u.Total.Utilization != nil && amount(u.Total.Utilization.TotalCommitment) > 0 {
		found = true
		ut := u.Total.Utilization
		sp.UtilizationPct = pct(ut.UtilizationPercentage)
		sp.Commitment, sp.Unused = optional(ut.TotalCommitment), optional(ut.UnusedCommitment)
		if u.Total.Savings != nil {
			sp.NetSavings = optional(u.Total.Savings.NetSavings)
		}
	}
	if found {
		if cov, err := ce.GetSavingsPlansCoverage(ctx, &costexplorer.GetSavingsPlansCoverageInput{TimePeriod: period}); err == nil {
			var covered, total float64
			for _, x := range cov.SavingsPlansCoverages {
				if x.Coverage != nil {
					covered += amount(x.Coverage.SpendCoveredBySavingsPlans)
					total += amount(x.Coverage.TotalCost)
				}
			}
			if total > 0 {
				v := round1(covered / total * 100)
				sp.CoveragePct = &v
			}
		} else if !noCommitments(err) {
			return nil, classify(err)
		}
		out = append(out, sp)
	}

	// Reservations
	ri := base(domain.CommitmentReservations)
	found = false
	if u, err := ce.GetReservationUtilization(ctx, &costexplorer.GetReservationUtilizationInput{TimePeriod: period}); err != nil {
		if !noCommitments(err) {
			return nil, classify(err)
		}
	} else if u.Total != nil && amount(u.Total.PurchasedHours) > 0 {
		found = true
		t := u.Total
		ri.UtilizationPct = pct(t.UtilizationPercentage)
		// reservations are bought in hours: the commitment is what the purchased hours cost, the unused part the cost of the idle hours
		cost := amount(t.AmortizedRecurringFee) + amount(t.AmortizedUpfrontFee)
		ri.Commitment = &cost
		ri.Unused = optional(t.RICostForUnusedHours)
		ri.NetSavings = optional(t.NetRISavings)
	}
	if found {
		if cov, err := ce.GetReservationCoverage(ctx, &costexplorer.GetReservationCoverageInput{TimePeriod: period}); err == nil {
			if cov.Total != nil && cov.Total.CoverageHours != nil {
				ri.CoveragePct = pct(cov.Total.CoverageHours.CoverageHoursPercentage)
			}
		} else if !noCommitments(err) {
			return nil, classify(err)
		}
		out = append(out, ri)
	}
	return out, nil
}

// noCommitments is the answer of an account that has never had a Savings Plan or reservation, or has no data in the window: the cloud
// says so as an error, which here only means "nothing to read".
func noCommitments(err error) bool {
	m := strings.ToLower(err.Error())
	return strings.Contains(m, "dataunavailable") || strings.Contains(m, "no savings plans") || strings.Contains(m, "no reservation")
}

func amount(s *string) float64 {
	if s == nil {
		return 0
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(*s), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

// optional is a money or hours figure the cloud gave, or nil when it did not (an absent figure is not zero).
func optional(s *string) *float64 {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	v := amount(s)
	return &v
}

func pct(s *string) *float64 {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	v := round1(amount(s))
	return &v
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
