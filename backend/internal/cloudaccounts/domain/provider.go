// Package domain defines cloud connections and the provider plugin contract.
package domain

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

type ProviderType string

const (
	AWS   ProviderType = "aws"
	Azure ProviderType = "azure"
	GCP   ProviderType = "gcp"
)

func (p ProviderType) Valid() bool { return p == AWS || p == Azure || p == GCP }

type SyncStatus string

const (
	StatusPending SyncStatus = "pending"
	StatusHealthy SyncStatus = "healthy"
	StatusError   SyncStatus = "error"
)

var (
	ErrInvalidConnection   = errors.New("invalid cloud connection")
	ErrUnsupportedProvider = errors.New("provider not supported yet")
	ErrNotFound            = errors.New("not found")
	ErrDuplicate           = errors.New("cloud account already connected")
	// ErrExportNotFound: the FOCUS export has no manifest for the requested months (wrong location, or the
	// first delivery has not happened yet). Retrying immediately cannot help.
	ErrExportNotFound = errors.New("FOCUS export not found")
	// ErrAccessDenied: the granted access is missing or too narrow. Retrying cannot help.
	ErrAccessDenied = errors.New("access denied by the cloud provider")
)

// Connection links a project to a cloud account. CredentialRef is a *reference* (IAM role ARN),
// never a secret. ExternalID is the per-connection STS ExternalId (confused-deputy protection).
type Connection struct {
	ID            string       `json:"id"`
	TenantID      string       `json:"tenant_id"`
	ProjectID     string       `json:"project_id"`
	Provider      ProviderType `json:"provider"`
	AccountRef    string       `json:"account_ref"` // AWS account id / subscription id / GCP project id
	CredentialRef string       `json:"credential_ref"`
	ExternalID    string       `json:"external_id"`
	SyncStatus    SyncStatus   `json:"sync_status"`
	LastError     *string      `json:"last_error"`
	LastSyncAt    *time.Time   `json:"last_sync_at"`
	SyncedThrough *time.Time   `json:"synced_through"`
	// Export, when set, makes the connection read the customer's FOCUS data export from S3 instead of
	// Cost Explorer (invoice-level data, exact instance hours). nil = Cost Explorer.
	Export *ExportConfig `json:"export"`
}

var (
	awsAccountRe = regexp.MustCompile(`^\d{12}$`)
	bucketRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)
	exportPartRe = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)
	regionRe     = regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-\d$`)
	// The platform identity is only allowed to assume roles named Reliabilix* (see deploy/aws/platform-policy.json);
	// enforcing the same rule here keeps a pasted ARN from ever steering the platform into an arbitrary role.
	awsRoleRe = regexp.MustCompile(`^arn:aws:iam::(\d{12}):role/(Reliabilix[A-Za-z0-9+=,.@_-]{0,100})$`)
	// secretLike catches raw credentials pasted into credential_ref.
	secretLike = regexp.MustCompile(`AKIA|ASIA|-----BEGIN|aws_secret_access_key`)
)

// Validate rejects unknown providers, malformed identifiers and raw secrets in credential_ref.
func (c Connection) Validate() error {
	if !c.Provider.Valid() {
		return fmt.Errorf("%w: unknown provider", ErrInvalidConnection)
	}
	if c.ProjectID == "" || c.AccountRef == "" || c.CredentialRef == "" {
		return fmt.Errorf("%w: project_id, account_ref, credential_ref required", ErrInvalidConnection)
	}
	if secretLike.MatchString(c.CredentialRef) {
		return fmt.Errorf("%w: credential_ref must be a reference, not a secret", ErrInvalidConnection)
	}
	if c.Provider == AWS {
		if !awsAccountRe.MatchString(c.AccountRef) {
			return fmt.Errorf("%w: AWS account id must be 12 digits", ErrInvalidConnection)
		}
		m := awsRoleRe.FindStringSubmatch(c.CredentialRef)
		if m == nil {
			return fmt.Errorf("%w: credential_ref must be an IAM role ARN whose name starts with \"Reliabilix\" (no path)", ErrInvalidConnection)
		}
		if m[1] != c.AccountRef {
			return fmt.Errorf("%w: role ARN belongs to a different AWS account", ErrInvalidConnection)
		}
	}
	return nil
}

// NewExternalID returns an unguessable ExternalId for a new connection.
func NewExternalID() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "rlx-" + hex.EncodeToString(b), nil
}

// UsageRequest asks a provider for usage in [From, To) (dates).
type UsageRequest struct {
	Connection Connection
	From, To   time.Time
}

// CloudProvider is the plugin contract implemented by AWS (now), Azure and GCP (phase 2).
type CloudProvider interface {
	Provider() ProviderType
	// Validate checks that the granted access works.
	Validate(ctx context.Context, c Connection) error
	// GetUsage returns raw, provider-specific billing data for ingestion to normalize into FOCUS.
	GetUsage(ctx context.Context, req UsageRequest) ([]focus.RawBatch, error)
}

// Registry resolves providers by type.
type Registry struct {
	m map[ProviderType]CloudProvider
}

func NewRegistry(ps ...CloudProvider) *Registry {
	r := &Registry{m: map[ProviderType]CloudProvider{}}
	for _, p := range ps {
		r.m[p.Provider()] = p
	}
	return r
}

func (r *Registry) Get(t ProviderType) (CloudProvider, error) {
	p, ok := r.m[t]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedProvider, t)
	}
	return p, nil
}

// SyncRun is one execution of a connection sync.
type SyncRun struct {
	ID         string     `json:"id"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	Status     string     `json:"status"`
	Records    int        `json:"records"`
	Error      *string    `json:"error"`
}

// JobRef identifies a connection for cross-tenant job fan-out.
type JobRef struct{ TenantID, ProjectID, ConnectionID string }

// Repository is tenant-scoped (explicit tenantID + RLS).
type Repository interface {
	List(ctx context.Context, tenantID string) ([]Connection, error)
	Get(ctx context.Context, tenantID, id string) (Connection, error)
	Create(ctx context.Context, c Connection) (Connection, error)
	Delete(ctx context.Context, tenantID, id string) error
	// SetExport sets or clears (nil) the FOCUS export location, resetting the sync cursor; audited.
	SetExport(ctx context.Context, tenantID, id string, e *ExportConfig) error
	SetStatus(ctx context.Context, tenantID, id string, status SyncStatus, lastError *string) error
	StartRun(ctx context.Context, tenantID, connectionID string) (string, error)
	FinishRun(ctx context.Context, tenantID, runID string, ok bool, records int, errMsg *string) error
	MarkSynced(ctx context.Context, tenantID, id string, through time.Time, records int) error
	ListRuns(ctx context.Context, tenantID, connectionID string) ([]SyncRun, error)
	ListForSync(ctx context.Context) ([]JobRef, error)
	// Audit appends a user-visible audit entry for the connection (verification outcome).
	Audit(ctx context.Context, tenantID, action, connectionID string, meta map[string]any) error
}
