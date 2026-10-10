// Package entitlements is the port domains use to ask "may this organization create one more of X?". The answer
// comes from the organization's limits (operator-managed settings today, subscription plans later: ADR-0022/0023);
// domains neither know nor care where a limit comes from.
package entitlements

import (
	"context"
	"errors"
	"fmt"
)

type Resource string

const (
	Projects     Resource = "projects"
	Connections  Resource = "cloud connections"
	Members      Resource = "members"
	Destinations Resource = "notification destinations"
)

// ErrLimitReached is returned when creating another resource would exceed the organization's limit.
var ErrLimitReached = errors.New("limit reached")

// LimitError names the resource and the limit; it satisfies errors.Is(err, ErrLimitReached).
type LimitError struct {
	Resource Resource
	Limit    int64
}

func (e LimitError) Error() string {
	return fmt.Sprintf("the limit of %d %s for your organization is reached", e.Limit, e.Resource)
}
func (e LimitError) Is(t error) bool { return t == ErrLimitReached }

// Gate decides whether one more resource may be created. Existing resources are never touched when a limit drops;
// only creation is refused.
type Gate interface {
	Allow(ctx context.Context, tenantID string, r Resource) error
}

// Unlimited allows everything (tests, modules wired without limits).
type Unlimited struct{}

func (Unlimited) Allow(context.Context, string, Resource) error { return nil }
