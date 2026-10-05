package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/quixgit/greenops-reliabilix/pkg/config"
)

// DevVerifier accepts tokens of the form "dev:<subject>:<tenant>:<role>".
// It exists only for local development and is refused unless ENV=dev.
type DevVerifier struct{}

func (DevVerifier) Verify(_ context.Context, token string) (Claims, error) {
	p := strings.Split(token, ":")
	if len(p) != 4 || p[0] != "dev" {
		return Claims{}, ErrUnauthenticated
	}
	return Claims{Subject: p[1], TenantID: p[2], Role: Role(p[3])}, nil
}

// VerifierFromEnv builds the token verifier. It fails closed: outside ENV=dev an
// OIDC issuer must be configured.
//
// TODO(ADR-0008): implement OIDCVerifier (JWKS cache, iss/aud/exp/nbf checks,
// tenant + role claim mapping) using github.com/coreos/go-oidc/v3.
func VerifierFromEnv() (Verifier, error) {
	if config.IsDev() {
		return DevVerifier{}, nil
	}
	if config.String("OIDC_ISSUER", "") == "" {
		return nil, errors.New("OIDC_ISSUER is required outside ENV=dev")
	}
	return nil, fmt.Errorf("OIDC verifier not implemented yet")
}
