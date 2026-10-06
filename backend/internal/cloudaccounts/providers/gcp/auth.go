package gcp

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// oauthClient returns an http.Client that signs requests with the platform's own credentials. The timeout is a
// backstop; individual BigQuery calls also carry their own deadlines.
func oauthClient(ctx context.Context, creds *google.Credentials) *http.Client {
	c := oauth2.NewClient(ctx, creds.TokenSource)
	c.Timeout = 3 * time.Minute
	return c
}
