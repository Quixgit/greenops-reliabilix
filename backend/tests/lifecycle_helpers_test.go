//go:build integration

package tests

import (
	"bytes"
	"io"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type poolT = *pgxpool.Pool

// wrap lets seedOrg take a pool without exposing it as an interface everywhere.
type wrap struct{ p *pgxpool.Pool }

func (w wrap) pool() poolT { return w.p }

func jsonReader(s string) io.Reader { return strings.NewReader(s) }

func contains(b []byte, s string) bool { return bytes.Contains(b, []byte(s)) }
