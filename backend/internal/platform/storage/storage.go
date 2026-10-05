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
