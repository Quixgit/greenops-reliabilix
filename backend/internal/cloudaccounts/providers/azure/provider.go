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

// Provider reads Azure costs in one of two ways, told apart by the connection's credential_ref:
//
//	azcost://<tenant>@<client>   the customer's service principal and sealed secret, Cost Management Query API
//	azblob://<tenant>@<account>/… a Cost Management export in Blob Storage, read with the platform's own application
type Provider struct {
	Blobs Blobs // export mode; nil when the platform application is not configured
	Cost  *CostAPI
	// OpenSecret returns the plaintext client secret of a service-principal connection.
	OpenSecret func(domain.Connection) (string, error)
	HTTP       *http.Client
	Authority  string // token endpoint base; "" = Microsoft's
}

// New builds the provider. The platform application (clientID, clientSecret) is only needed for export mode and may
// be empty; service-principal connections need nothing from the platform but the key that opens their secret.
func New(clientID, clientSecret string, openSecret func(domain.Connection) (string, error)) *Provider {
	h := NoRedirects(&http.Client{Timeout: 3 * time.Minute})
	p := &Provider{Cost: &CostAPI{HTTP: h, Pace: queryPacing}, OpenSecret: openSecret, HTTP: h}
	if clientID != "" && clientSecret != "" {
		p.Blobs = &RESTBlobs{HTTP: h, Tokens: &ClientCredentials{ClientID: clientID, Secret: clientSecret, HTTP: h}}
	}
	return p
}

// errNoApplication: export mode was asked for on a platform without its Entra application.
var errNoApplication = errors.New("azure: reading an export needs the platform's Microsoft Entra application, which is not configured")

// token returns a Cost Management token for a service-principal connection, with the customer's own secret.
func (p *Provider) token(ctx context.Context, c domain.Connection) (string, error) {
	acc, err := domain.ParseAzureCostRef(c.CredentialRef)
	if err != nil {
		return "", err
	}
	if p.OpenSecret == nil {
		return "", errors.New("azure: no way to open the connection's secret")
	}
	secret, err := p.OpenSecret(c)
	if err != nil {
		// The worker and the API must share the sealing key (NOTIFICATIONS_SECRET_KEY): the API sealed the secret,
		// the worker opens it. Say so instead of a generic "sync failed".
		return "", domain.PublicError{Message: "The stored client secret cannot be opened by this service: every service (api, worker, scheduler) must run with the same NOTIFICATIONS_SECRET_KEY. Reconnect the account if the key was changed."}
	}
	cc := &ClientCredentials{ClientID: acc.ClientID, Secret: secret, Authority: p.Authority, HTTP: p.HTTP, Scope: "https://management.azure.com/.default"}
	return cc.Token(ctx, acc.TenantID)
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
	if domain.IsAzureCostRef(c.CredentialRef) {
		return p.validateCost(ctx, c)
	}
	if p.Blobs == nil {
		return errNoApplication
	}
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
	if domain.IsAzureCostRef(c.CredentialRef) {
		tok, err := p.token(ctx, c)
		if err != nil {
			return nil, err
		}
		b, err := p.Cost.usage(ctx, tok, c.AccountRef, req.From, req.To)
		if err != nil {
			return nil, err
		}
		return []focus.RawBatch{b}, nil
	}
	if p.Blobs == nil {
		return nil, errNoApplication
	}
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

// validateCost proves the service principal can read the subscription's costs: it signs in with the stored secret
// and runs a one-day, ungrouped query (cheap, and exactly the permission the sync needs).
func (p *Provider) validateCost(ctx context.Context, c domain.Connection) error {
	tok, err := p.token(ctx, c)
	if err != nil {
		return err
	}
	day := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -1)
	_, err = p.Cost.controlTotals(ctx, tok, c.AccountRef, day, day.AddDate(0, 0, 1))
	return err
}

// GetControlTotals implements domain.ControlProvider. Through the Query API the day totals come from a second,
// ungrouped query, so a row dropped or mis-parsed between the detailed query and the database shows up as a
// difference. An invoice-level export is its own source of truth, so there is nothing to compare it with (nil).
func (p *Provider) GetControlTotals(ctx context.Context, req domain.UsageRequest) ([]focus.ControlTotal, error) {
	c := req.Connection
	if !domain.IsAzureCostRef(c.CredentialRef) {
		return nil, nil
	}
	tok, err := p.token(ctx, c)
	if err != nil {
		return nil, err
	}
	return p.Cost.controlTotals(ctx, tok, c.AccountRef, req.From, req.To)
}
