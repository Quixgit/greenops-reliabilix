package app

import (
	"context"

	cloudapp "github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/application"
	finopsdomain "github.com/quixgit/greenops-reliabilix/backend/internal/finops/domain"
)

// existingCommitments gives the finops report what the connected cloud accounts hold, read by the cloud accounts module (ADR-0054). It is an
// adapter in the composition root because the two domains never import each other.
type existingCommitments struct{ cloud cloudapp.Service }

func (e existingCommitments) ExistingCommitments(ctx context.Context, tenantID string) ([]finopsdomain.Existing, error) {
	snaps, err := e.cloud.ExistingCommitments(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]finopsdomain.Existing, 0, len(snaps))
	for _, s := range snaps {
		out = append(out, finopsdomain.Existing{Provider: s.Provider, Kind: s.Kind, UtilizationPct: s.UtilizationPct, CoveragePct: s.CoveragePct, Commitment: s.Commitment, Unused: s.Unused})
	}
	return out, nil
}
