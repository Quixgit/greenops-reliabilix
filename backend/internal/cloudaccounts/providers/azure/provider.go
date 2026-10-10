// Package azure implements domain.CloudProvider on the customer's Cost Management export (FOCUS cost and usage,
// CSV) in Blob Storage. The platform reads it with its own Microsoft Entra application: the customer creates that
// application's service principal in their tenant, grants it Storage Blob Data Reader on the container and marks
// the container with the connection's ownership token. The platform stores no customer credential (ADR-0041).
package azure

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

// SourceFocusExport is the focus.RawBatch.Source of the aggregate built from a Cost Management export.
const SourceFocusExport = "focus_export"

// ownershipValue is the value of the container metadata entry whose KEY is the connection's token.
const ownershipValue = "1"

type Provider struct {
	Blobs Blobs
}

// New builds the provider from the platform application's credentials.
func New(clientID, clientSecret string) (*Provider, error) {
	if clientID == "" || clientSecret == "" {
		return nil, errors.New("azure: the platform application id and secret are required")
	}
	h := NoRedirects(&http.Client{Timeout: 3 * time.Minute})
	tokens := &ClientCredentials{ClientID: clientID, Secret: clientSecret, HTTP: h}
	return &Provider{Blobs: &RESTBlobs{HTTP: h, Tokens: tokens}}, nil
}

func (*Provider) Provider() domain.ProviderType { return domain.Azure }

// proveOwnership requires the container metadata entry "<token>: 1". Only someone who can write to the container
// can set it, which stops one tenant from pointing a connection at another customer's export.
func (p *Provider) proveOwnership(ctx context.Context, e domain.AzureExport, externalID string) error {
	meta, err := p.Blobs.ContainerMetadata(ctx, e)
	if err != nil {
		return err
	}
	if externalID == "" || meta[strings.ToLower(domain.AzureOwnershipKey(externalID))] != ownershipValue {
		return domain.ErrOwnershipNotProven
	}
	return nil
}

// Validate proves ownership, then checks that the export has delivered at least one data file.
func (p *Provider) Validate(ctx context.Context, c domain.Connection) error {
	e, err := domain.ParseAzureExportRef(c.CredentialRef)
	if err != nil {
		return err
	}
	if err := p.proveOwnership(ctx, e, c.ExternalID); err != nil {
		return err
	}
	blobs, err := p.Blobs.List(ctx, e, e.Dir+"/")
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	files, err := selectFiles(e.Dir, blobs, time.Time{}, now.AddDate(0, 0, 2))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return domain.ErrExportNotFound
	}
	return nil
}

// GetUsage reads the date ranges overlapping [From, To) of the export and aggregates the subscription's rows by
// day, service, category, region and currency (plus measured vCPU hours and storage GB-months).
func (p *Provider) GetUsage(ctx context.Context, req domain.UsageRequest) ([]focus.RawBatch, error) {
	c := req.Connection
	e, err := domain.ParseAzureExportRef(c.CredentialRef)
	if err != nil {
		return nil, err
	}
	if err := p.proveOwnership(ctx, e, c.ExternalID); err != nil {
		return nil, err
	}
	blobs, err := p.Blobs.List(ctx, e, e.Dir+"/")
	if err != nil {
		return nil, err
	}
	files, err := selectFiles(e.Dir, blobs, req.From, req.To)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, domain.ErrExportNotFound
	}
	agg := newAggregate(c.AccountRef, req.From, req.To)
	for _, f := range files {
		if err := readDataFile(ctx, p.Blobs, e, f, agg); err != nil {
			return nil, err
		}
	}
	if !agg.matched {
		return nil, domain.ErrSubscriptionNotInExport
	}
	payload, err := json.Marshal(agg.payload())
	if err != nil {
		return nil, err
	}
	return []focus.RawBatch{{Provider: "azure", Source: SourceFocusExport, Payload: payload}}, nil
}

// GetControlTotals implements domain.ControlProvider: an invoice-level export is its own source of truth, so there
// is no independent total to compare with (the same as the AWS data export).
func (*Provider) GetControlTotals(context.Context, domain.UsageRequest) ([]focus.ControlTotal, error) {
	return nil, nil
}
