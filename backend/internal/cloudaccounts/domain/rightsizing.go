package domain

import (
	"context"
	"errors"
)

// RightsizingAction is what the cloud provider advises for an instance.
type RightsizingAction string

const (
	RightsizingModify    RightsizingAction = "modify"
	RightsizingTerminate RightsizingAction = "terminate"
)

// InstanceShape is the hardware of an instance type as reported by the provider (0 = not reported).
type InstanceShape struct {
	VCPU     float64
	MemoryGB float64
}

// RightsizingFinding is one provider recommendation for a single instance. It carries facts only; the
// recommendations domain decides what they are worth in carbon terms.
type RightsizingFinding struct {
	ResourceID  string
	Region      string
	Action      RightsizingAction
	CurrentType string
	TargetType  string // empty for terminate
	Current     InstanceShape
	Target      InstanceShape
	// EstimatedMonthlySavings is the provider's own estimate, in Currency.
	EstimatedMonthlySavings float64
	Currency                string
}

// RightsizingProvider is implemented by providers that can list rightsizing findings (AWS Cost Explorer).
type RightsizingProvider interface {
	GetRightsizing(ctx context.Context, c Connection) ([]RightsizingFinding, error)
}

// RightsizingSink receives the findings of one connection (implemented by the recommendations module
// through an adapter in the composition root: domains never import each other).
type RightsizingSink interface {
	SubmitRightsizing(ctx context.Context, tenantID, projectID string, findings []RightsizingFinding) error
}

// ErrRightsizingUnsupported is returned when the connection's provider cannot list findings.
var ErrRightsizingUnsupported = errors.New("provider has no rightsizing findings")
