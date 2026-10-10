// Package application holds the operator console use-cases. Every call first proves the caller is an operator
// (from the database, never from the token), records the access, then runs.
package application

import (
	"context"
	"regexp"

	"github.com/quixgit/greenops-reliabilix/backend/internal/operator/domain"
)

type Service struct {
	Repo     domain.Repository
	System   func() domain.System
	Settings domain.SettingsPort
}

// Authorize resolves the operator behind a verified subject. write=true additionally demands an admin operator.
// The attempt is logged before anything is returned, so the access log also shows who looked.
func (s Service) Authorize(ctx context.Context, subject, email, action string, write bool) (domain.Operator, error) {
	role, ok, err := s.Repo.Role(ctx, subject)
	if err != nil {
		return domain.Operator{}, err
	}
	if !ok || !role.Valid() {
		return domain.Operator{}, domain.ErrNotOperator
	}
	if write && !role.CanWrite() {
		return domain.Operator{}, domain.ErrReadOnly
	}
	if err := s.Repo.LogAccess(ctx, subject, action); err != nil {
		return domain.Operator{}, err // no access without a record
	}
	return domain.Operator{Subject: subject, Email: email, Role: role}, nil
}

func (s Service) Tenants(ctx context.Context) ([]domain.Tenant, error) {
	return s.Repo.Tenants(ctx, 500)
}

func (s Service) Deliveries(ctx context.Context, status string, limit int) ([]domain.Delivery, error) {
	if status != "" && !domain.DeliveryStatuses[status] {
		return nil, domain.ErrInvalid
	}
	return s.Repo.Deliveries(ctx, status, limit)
}

func (s Service) DeliveryStats(ctx context.Context, hours int) ([]domain.DeliveryStat, error) {
	if hours < 1 || hours > 24*30 {
		return nil, domain.ErrInvalid
	}
	return s.Repo.DeliveryStats(ctx, hours)
}

func (s Service) UnhealthyConnections(ctx context.Context) ([]domain.Connection, error) {
	return s.Repo.UnhealthyConnections(ctx, 200)
}

var serviceRe = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,31}$`)

// Errors lists error groups. status "" means all; service "" means every service.
func (s Service) Errors(ctx context.Context, status, service string, limit int) ([]domain.ErrorGroup, error) {
	if status != "" && !domain.ErrorStatuses[status] {
		return nil, domain.ErrInvalid
	}
	if service != "" && !serviceRe.MatchString(service) {
		return nil, domain.ErrInvalid
	}
	return s.Repo.Errors(ctx, status, service, limit)
}

// SetErrorStatus marks an error group open, resolved or ignored.
func (s Service) SetErrorStatus(ctx context.Context, op domain.Operator, id, status string) error {
	if !domain.ErrorStatuses[status] {
		return domain.ErrInvalid
	}
	done, err := s.Repo.SetErrorStatus(ctx, op.Subject, id, status)
	if err == nil && !done {
		return domain.ErrNotFound
	}
	return err
}

// RetryDelivery puts a failed delivery back in the queue (the sweep picks it up within about a minute).
func (s Service) RetryDelivery(ctx context.Context, op domain.Operator, tenantID, deliveryID string) error {
	done, err := s.Repo.RetryDelivery(ctx, op.Subject, tenantID, deliveryID)
	if err == nil && !done {
		return domain.ErrNotFound
	}
	return err
}

func (s Service) SetDestinationEnabled(ctx context.Context, op domain.Operator, tenantID, destinationID string, enabled bool) error {
	done, err := s.Repo.SetDestinationEnabled(ctx, op.Subject, tenantID, destinationID, enabled)
	if err == nil && !done {
		return domain.ErrNotFound
	}
	return err
}

func (s Service) SystemInfo() domain.System { return s.System() }
