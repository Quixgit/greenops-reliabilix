// Package aws will implement CloudProvider using STS AssumeRole (external id)
// and the Cost Explorer API, later CUR via S3/Athena.
package aws

import (
	"context"
	"errors"

	"github.com/quixgit/greenops-reliabilix/services/cloudintegration/internal/domain"
)

type Provider struct{}

func (Provider) Name() string { return "aws" }

func (Provider) Validate(context.Context, domain.Account) error {
	return errors.New("aws: not implemented")
}

func (Provider) FetchUsage(context.Context, domain.UsageRequest) ([]domain.RawUsage, error) {
	return nil, errors.New("aws: not implemented")
}
