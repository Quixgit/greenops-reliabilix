package azure

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
)

const (
	blobAPIVersion = "2023-11-03"
	maxListPages   = 50
	maxListBytes   = 8 << 20
)

// BlobInfo is one blob of a listing.
type BlobInfo struct {
	Name string
	Size int64
}

// Blobs is the slice of Blob Storage needed to read an export (faked in tests).
type Blobs interface {
	// ContainerMetadata returns the container's metadata (keys lower-cased).
	ContainerMetadata(ctx context.Context, e domain.AzureExport) (map[string]string, error)
	// List returns every blob whose name starts with prefix.
	List(ctx context.Context, e domain.AzureExport, prefix string) ([]BlobInfo, error)
	// Open streams one blob; the caller closes it.
	Open(ctx context.Context, e domain.AzureExport, name string) (io.ReadCloser, int64, error)
}

// TokenSource returns an access token for Azure Storage, acting as the platform's application in the customer's
// Entra tenant.
type TokenSource interface {
	Token(ctx context.Context, tenantID string) (string, error)
}

// ClientCredentials requests tokens with the platform application's client secret (a multi-tenant application:
// the customer creates its service principal in their own tenant). Tokens are cached until a minute before expiry.
type ClientCredentials struct {
	ClientID, Secret string
	Authority        string // default https://login.microsoftonline.com
	Scope            string // default https://storage.azure.com/.default
	HTTP             *http.Client
	Now              func() time.Time

	mu    sync.Mutex
	cache map[string]cachedToken
}

type cachedToken struct {
	value   string
	expires time.Time
}

func (c *ClientCredentials) scope() string {
	if c.Scope != "" {
		return c.Scope
	}
	return "https://storage.azure.com/.default"
}

func (c *ClientCredentials) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *ClientCredentials) Token(ctx context.Context, tenantID string) (string, error) {
	c.mu.Lock()
	t, ok := c.cache[tenantID]
	c.mu.Unlock()
	if ok && c.now().Before(t.expires) {
		return t.value, nil
	}
	authority := c.Authority
	if authority == "" {
		authority = "https://login.microsoftonline.com"
	}
	form := url.Values{"client_id": {c.ClientID}, "client_secret": {c.Secret}, "grant_type": {"client_credentials"},
		"scope": {c.scope()}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authority+"/"+url.PathEscape(tenantID)+"/oauth2/v2.0/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("azure: token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized {
		// The application does not exist in that tenant (or has no service principal there), the secret is wrong or
		// expired, or the tenant id is wrong. Only the customer can fix that; retrying cannot help.
		return "", fmt.Errorf("%w: Microsoft Entra refused the sign-in (check the tenant id, the application id and that the secret is its value and has not expired)", domain.ErrAccessDenied)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("azure: token request: status %d", resp.StatusCode)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.AccessToken == "" {
		return "", errors.New("azure: unreadable token response")
	}
	ttl := time.Duration(tok.ExpiresIn)*time.Second - time.Minute
	if ttl < 0 {
		ttl = 0
	}
	c.mu.Lock()
	if c.cache == nil {
		c.cache = map[string]cachedToken{}
	}
	c.cache[tenantID] = cachedToken{tok.AccessToken, c.now().Add(ttl)}
	c.mu.Unlock()
	return tok.AccessToken, nil
}

// RESTBlobs reads Blob Storage over its REST API with a bearer token.
type RESTBlobs struct {
	HTTP   *http.Client
	Tokens TokenSource
	// Endpoint returns the base URL of a storage account; nil = https://<account>.blob.core.windows.net. The account
	// name is validated by domain.ParseAzureExportRef before it gets here, so the host is never attacker-chosen.
	Endpoint func(account string) string
}

func (b *RESTBlobs) base(account string) string {
	if b.Endpoint != nil {
		return b.Endpoint(account)
	}
	return "https://" + account + ".blob.core.windows.net"
}

func (b *RESTBlobs) do(ctx context.Context, e domain.AzureExport, rawURL string) (*http.Response, error) {
	tok, err := b.Tokens.Token(ctx, e.TenantID)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("x-ms-version", blobAPIVersion)
	resp, err := b.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure: storage request: %w", err)
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%w: the platform application may not read this storage container", domain.ErrAccessDenied)
	case resp.StatusCode == http.StatusNotFound:
		_ = resp.Body.Close()
		return nil, domain.ErrExportNotFound
	case resp.StatusCode/100 == 3: // never follow: the token must not leave the storage account
		_ = resp.Body.Close()
		return nil, errors.New("azure: unexpected redirect from storage")
	case resp.StatusCode != http.StatusOK:
		_ = resp.Body.Close()
		return nil, fmt.Errorf("azure: storage request: status %d", resp.StatusCode)
	}
	return resp, nil
}

func (b *RESTBlobs) ContainerMetadata(ctx context.Context, e domain.AzureExport) (map[string]string, error) {
	resp, err := b.do(ctx, e, b.base(e.Account)+"/"+e.Container+"?restype=container")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	out := map[string]string{}
	for k, v := range resp.Header {
		if name, ok := strings.CutPrefix(strings.ToLower(k), "x-ms-meta-"); ok && len(v) > 0 {
			out[name] = v[0]
		}
	}
	return out, nil
}

type listResult struct {
	Blobs []struct {
		Name       string `xml:"Name"`
		Properties struct {
			Length int64 `xml:"Content-Length"`
		} `xml:"Properties"`
	} `xml:"Blobs>Blob"`
	NextMarker string `xml:"NextMarker"`
}

func (b *RESTBlobs) List(ctx context.Context, e domain.AzureExport, prefix string) ([]BlobInfo, error) {
	var out []BlobInfo
	marker := ""
	for page := 0; page < maxListPages; page++ {
		q := url.Values{"restype": {"container"}, "comp": {"list"}, "prefix": {prefix}}
		if marker != "" {
			q.Set("marker", marker)
		}
		resp, err := b.do(ctx, e, b.base(e.Account)+"/"+e.Container+"?"+q.Encode())
		if err != nil {
			return nil, err
		}
		var res listResult
		err = xml.NewDecoder(io.LimitReader(resp.Body, maxListBytes)).Decode(&res)
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("azure: unreadable blob listing: %w", err)
		}
		for _, bl := range res.Blobs {
			out = append(out, BlobInfo{Name: bl.Name, Size: bl.Properties.Length})
		}
		if res.NextMarker == "" {
			return out, nil
		}
		marker = res.NextMarker
	}
	return nil, errors.New("azure: export has too many files to list (limit exceeded)")
}

func (b *RESTBlobs) Open(ctx context.Context, e domain.AzureExport, name string) (io.ReadCloser, int64, error) {
	u := url.URL{Path: "/" + e.Container + "/" + name}
	resp, err := b.do(ctx, e, b.base(e.Account)+u.EscapedPath())
	if err != nil {
		return nil, 0, err
	}
	return resp.Body, resp.ContentLength, nil
}

// NoRedirects makes an http.Client refuse redirects (the platform's token is bound to one host).
func NoRedirects(c *http.Client) *http.Client {
	cc := *c
	cc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &cc
}
