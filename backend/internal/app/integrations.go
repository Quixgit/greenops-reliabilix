package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/providers/electricitymaps"
	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/channels/email"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/config"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/integrations"
)

// integrationEnv maps a console field to the environment value it falls back to (already loaded into the config).
func integrationEnv(cfg config.Config) func(integration, key string) string {
	return func(integration, key string) string {
		switch integration + "/" + key {
		case "electricitymaps/api_key":
			return cfg.ElectricityMapsKey
		case "electricitymaps/zone_overrides":
			return cfg.ElectricityMapsZoneOverrides
		case "smtp/host":
			return cfg.SMTPHost
		case "smtp/port":
			if cfg.SMTPHost != "" && cfg.SMTPPort > 0 {
				return strconv.Itoa(cfg.SMTPPort)
			}
		case "smtp/username":
			return cfg.SMTPUsername
		case "smtp/password":
			return cfg.SMTPPassword
		case "smtp/from":
			return cfg.SMTPFrom
		}
		return ""
	}
}

// smtpFrom builds the transport from the effective values, or reports that e-mail is not configured.
func smtpFrom(ctx context.Context, ints *integrations.Store, allowPlain bool) (email.SMTP, string, bool) {
	host, from := ints.Value(ctx, "smtp", "host"), ints.Value(ctx, "smtp", "from")
	if host == "" || from == "" {
		return email.SMTP{}, "", false
	}
	port, err := strconv.Atoi(ints.ValueOrDefault(ctx, "smtp", "port"))
	if err != nil || port < 1 {
		port = 587
	}
	return email.SMTP{Host: host, Port: port, Username: ints.Value(ctx, "smtp", "username"), Password: ints.Value(ctx, "smtp", "password"),
		AllowPlain: allowPlain}, from, true
}

// integrationsPort is the operator console's view of the credential store plus the connection tests.
type integrationsPort struct {
	store      *integrations.Store
	allowPlain bool
}

func (p integrationsPort) Statuses(ctx context.Context) ([]integrations.Status, error) {
	return p.store.Statuses(ctx)
}

func (p integrationsPort) Set(ctx context.Context, actor, integration string, values map[string]*string) error {
	return p.store.Set(ctx, actor, integration, values)
}

// Test never echoes a provider's own error text: it can carry the customer-visible account or key fragments.
func (p integrationsPort) Test(ctx context.Context, integration string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	switch integration {
	case "electricitymaps":
		key := p.store.Value(ctx, "electricitymaps", "api_key")
		if key == "" {
			return "", errors.New("no API key is configured")
		}
		overrides, err := electricitymaps.ParseOverrides(p.store.Value(ctx, "electricitymaps", "zone_overrides"))
		if err != nil {
			return "", errors.New("the zone overrides are not valid")
		}
		zones, err := electricitymaps.New(key, overrides).Zones(ctx)
		if err != nil {
			return "", errors.New("Electricity Maps did not accept the key, or could not be reached")
		}
		return fmt.Sprintf("The key is accepted: %d zones are available.", len(zones)), nil
	case "smtp":
		s, _, ok := smtpFrom(ctx, p.store, p.allowPlain)
		if !ok {
			return "", errors.New("the server and the sender address are required")
		}
		if err := s.Check(ctx); err != nil {
			return "", err // the SMTP checker's messages are fixed strings
		}
		return "Connected to the mail server and signed in. No message was sent.", nil
	}
	return "", integrations.ErrUnknown
}
