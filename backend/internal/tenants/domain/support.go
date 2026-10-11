package domain

import (
	"context"
	"time"
)

// SupportGrant is a time-limited permission for the platform's operators to look at the working detail of an organization.
type SupportGrant struct {
	ID        string     `json:"id"`
	GrantedBy string     `json:"granted_by"`
	GrantedAt time.Time  `json:"granted_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at"`
	Note      string     `json:"note"`
	Active    bool       `json:"active"`
}

// SupportView is one look an operator took while a grant was active.
type SupportView struct {
	Operator string    `json:"operator"`
	What     string    `json:"what"`
	At       time.Time `json:"at"`
}

// Support access limits: at least an hour, at most a week, so access is never open-ended.
const (
	MinSupportHours = 1
	MaxSupportHours = 168
)

// SupportStore keeps the grants of an organization and what was looked at.
type SupportStore interface {
	// Grant opens support access for hours, replacing any grant still open.
	Grant(ctx context.Context, tenantID, by string, hours int, note string) (SupportGrant, error)
	// Revoke closes the open grant; false when there was none.
	Revoke(ctx context.Context, tenantID string) (bool, error)
	// History returns the newest grants (the first may be active) and the newest looks.
	History(ctx context.Context, tenantID string, grants, views int) ([]SupportGrant, []SupportView, error)
}
