package app

import (
	"io"
	"log/slog"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
)

func nil2() *slog.Logger         { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
func devVerifier() auth.Verifier { return auth.DevVerifier{} }
