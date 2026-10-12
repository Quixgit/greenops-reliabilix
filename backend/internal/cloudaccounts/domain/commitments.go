package domain

import (
	"context"
	"time"
)

// Existing commitments (ADR-0054): what a cloud account already holds in Savings Plans and reservations, and how well it is used. The
// platform only reads them; it never buys, changes or cancels anything.

// Commitment kinds.
const (
	CommitmentSavingsPlans = "savings_plans"
	CommitmentReservations = "reservations"
)

// CommitmentSnapshot is the last reading of one kind for one connection over [PeriodFrom, PeriodTo). A figure the cloud did not give is nil:
// it is never filled with zero.
type CommitmentSnapshot struct {
	ConnectionID string    `json:"connection_id"`
	Provider     string    `json:"provider"`
	Account      string    `json:"account"`
	Kind         string    `json:"kind"`
	PeriodFrom   time.Time `json:"period_from"`
	PeriodTo     time.Time `json:"period_to"`
	Currency     string    `json:"currency"`
	// UtilizationPct is how much of the commitment was used (a commitment paid for and not used is waste); CoveragePct is how much of the
	// eligible spend or running hours the commitment covered.
	UtilizationPct *float64 `json:"utilization_pct"`
	CoveragePct    *float64 `json:"coverage_pct"`
	// Commitment is what was committed over the period (Savings Plans: the commitment; reservations: the cost of the purchased hours);
	// Unused is the part not used; NetSavings is what the commitment saved against on-demand prices.
	Commitment *float64  `json:"commitment"`
	Unused     *float64  `json:"unused"`
	NetSavings *float64  `json:"net_savings"`
	FetchedAt  time.Time `json:"fetched_at"`
}

// CommitmentProvider is implemented by providers that can read the commitments an account holds (AWS Cost Explorer).
type CommitmentProvider interface {
	// GetCommitments reads the commitments over [from, to). An account without any returns no snapshots and no error.
	GetCommitments(ctx context.Context, c Connection, from, to time.Time) ([]CommitmentSnapshot, error)
}

// CommitmentStore keeps the latest snapshot per connection and kind.
type CommitmentStore interface {
	SaveCommitments(ctx context.Context, tenantID, connectionID string, snaps []CommitmentSnapshot) error
	ListCommitments(ctx context.Context, tenantID string) ([]CommitmentSnapshot, error)
}
