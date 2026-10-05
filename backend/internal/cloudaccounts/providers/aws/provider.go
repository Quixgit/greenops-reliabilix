// Package aws will implement domain.CloudProvider using STS AssumeRole with an
// External ID and the Cost Explorer API; CUR via S3/Athena follows.
package aws

import (
	"context"
	"errors"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
)

var ErrNotImplemented = errors.New("aws provider: not implemented")

type Provider struct{}

func (Provider) Provider() domain.ProviderType { return domain.AWS }

func (Provider) Validate(context.Context, domain.Connection) error { return ErrNotImplemented }

func (Provider) GetUsage(context.Context, domain.UsageRequest) ([]domain.RawUsage, error) {
	return nil, ErrNotImplemented
}
