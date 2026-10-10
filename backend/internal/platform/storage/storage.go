// Package storage is the S3-compatible object-store abstraction for reports,
// exports and raw billing files. Layout: tenants/{tenant_id}/{raw|reports|exports}/...
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Store keeps objects. Keys must be built with Key so tenants cannot collide.
type Store interface {
	Put(ctx context.Context, key string, r io.Reader) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	// DeletePrefix removes every object whose key starts with prefix (a tenant erasure). A prefix that matches
	// nothing is not an error. The prefix must end in "/" so "tenants/ab" cannot match "tenants/abc/...".
	DeletePrefix(ctx context.Context, prefix string) error
}

// ErrBadPrefix is returned by DeletePrefix for a prefix that could match more than one tenant.
var ErrBadPrefix = errors.New("storage: prefix must be a directory-like key ending in /")

func checkPrefix(prefix string) error {
	if prefix == "" || !strings.HasSuffix(prefix, "/") || strings.Contains(prefix, "..") || strings.HasPrefix(prefix, "/") || strings.Count(strings.Trim(prefix, "/"), "/") < 1 {
		return ErrBadPrefix
	}
	return nil
}

// TenantPrefix is the prefix of everything one tenant owns in the store.
func TenantPrefix(tenantID string) (string, error) {
	k, err := Key(tenantID, "x", "x")
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(k, "x/x"), nil
}

// Key builds a tenant-prefixed object key and rejects path traversal.
func Key(tenantID, kind, name string) (string, error) {
	for _, s := range []string{tenantID, kind, name} {
		if s == "" || strings.Contains(s, "..") || strings.ContainsAny(s, `\`) || strings.HasPrefix(s, "/") {
			return "", errors.New("storage: invalid key component")
		}
	}
	return path.Join("tenants", tenantID, kind, name), nil
}

// FSStore stores objects on local disk (development and tests only).
// TODO: S3Store (S3 / Cloudflare R2 / MinIO) via the AWS SDK v2.
type FSStore struct{ Root string }

func (s FSStore) abs(key string) (string, error) {
	p := filepath.Join(s.Root, filepath.FromSlash(key))
	if !strings.HasPrefix(p, filepath.Clean(s.Root)+string(os.PathSeparator)) {
		return "", fmt.Errorf("storage: key escapes root")
	}
	return p, nil
}

func (s FSStore) Put(_ context.Context, key string, r io.Reader) error {
	p, err := s.abs(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640) //nolint:gosec // p is validated by abs() to stay inside Root
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = io.Copy(f, r)
	return err
}

func (s FSStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	p, err := s.abs(key)
	if err != nil {
		return nil, err
	}
	return os.Open(p) //nolint:gosec // p is validated by abs() to stay inside Root
}

func (s FSStore) Delete(_ context.Context, key string) error {
	p, err := s.abs(key)
	if err != nil {
		return err
	}
	return os.Remove(p)
}

func (s FSStore) DeletePrefix(_ context.Context, prefix string) error {
	if err := checkPrefix(prefix); err != nil {
		return err
	}
	p, err := s.abs(strings.TrimSuffix(prefix, "/") + "/x")
	if err != nil {
		return err
	}
	return os.RemoveAll(filepath.Dir(p))
}
