package storage

import (
	"bytes"
	"context"
	"io"
)

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

// Memory is an in-process Store for tests.
type Memory struct{ M map[string][]byte }

func NewMemory() *Memory { return &Memory{M: map[string][]byte{}} }

func (m *Memory) Put(_ context.Context, key string, r io.Reader) error {
	b, err := io.ReadAll(r)
	m.M[key] = b
	return err
}

func (m *Memory) Get(_ context.Context, key string) (io.ReadCloser, error) {
	b, ok := m.M[key]
	if !ok {
		return nil, io.ErrUnexpectedEOF
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}
