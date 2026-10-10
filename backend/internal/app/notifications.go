package app

import (
	"context"
	"crypto/sha256"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications"
	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/channels/email"
	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/channels/safehttp"
	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/channels/slack"
	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/channels/webhook"
	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/config"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/entitlements"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/errlog"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/secretbox"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
	tenantsapp "github.com/quixgit/greenops-reliabilix/backend/internal/tenants/application"
)

// buildNotifications registers the channels this deployment offers and the key that protects destination
// addresses. This is the one place that knows every channel: adding one means implementing domain.Channel and
// appending it here.
func buildNotifications(cfg config.Config, log *slog.Logger, pool *pgxpool.Pool, q queue.Enqueuer, members tenantsapp.Service, errs errlog.Recorder, st settings.Reader, limits entitlements.Gate) (*notifications.Module, error) {
	box, err := notificationsBox(cfg, log)
	if err != nil {
		return nil, err
	}
	// Webhook and Slack call URLs chosen by tenants: public https endpoints only, except in development.
	opts := safehttp.Options{AllowPrivate: cfg.IsDev(), AllowHTTP: cfg.IsDev()}
	channels := []domain.Channel{webhook.New(opts), slack.New(safehttp.Options{})}
	var stubbed []domain.Kind // channels running on a development stand-in: shown as such in the UI
	switch {
	case cfg.SMTPHost != "":
		channels = append(channels, email.New(cfg.SMTPFrom, email.SMTP{Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword, AllowPlain: cfg.IsDev() && cfg.SMTPAllowPlain}))
	case cfg.IsDev():
		log.Warn("SMTP_HOST not set: e-mail notifications are only logged (development)")
		channels = append(channels, email.New("Reliabilix <notifications@localhost>", email.Log{L: log}))
		stubbed = append(stubbed, domain.KindEmail)
	default:
		log.Warn("SMTP_HOST not set: the e-mail notification channel is unavailable")
	}
	return notifications.New(notifications.Deps{Pool: pool, Log: log, Queue: q, Channels: domain.NewRegistry(channels...), Box: box,
		Directory: memberDirectory{tenants: members}, BaseURL: cfg.PublicBaseURL, Stubbed: stubbed, Errors: errs, Settings: st, Limits: limits}), nil
}

// notificationsBox returns the sealing box. Outside development a missing key leaves it unconfigured: the API then
// refuses to create destinations (503) instead of storing addresses unprotected.
func notificationsBox(cfg config.Config, log *slog.Logger) (*secretbox.Box, error) {
	switch {
	case cfg.NotificationsKey != "":
		k, err := secretbox.ParseKey(cfg.NotificationsKey)
		if err != nil {
			return nil, err
		}
		return secretbox.New(k)
	case cfg.IsDev():
		log.Warn("NOTIFICATIONS_SECRET_KEY not set: using a fixed development key")
		k := sha256.Sum256([]byte("reliabilix-development-notifications-key"))
		return secretbox.New(k[:])
	default:
		log.Warn("NOTIFICATIONS_SECRET_KEY not set: notification destinations cannot be created")
		return nil, nil
	}
}

// memberDirectory lets the notifications domain ask who belongs to a tenant without importing the tenants module.
type memberDirectory struct{ tenants tenantsapp.Service }

func (d memberDirectory) IsMemberEmail(ctx context.Context, tenantID, address string) (bool, error) {
	ms, err := d.tenants.Members(ctx, tenantID)
	if err != nil {
		return false, err
	}
	for _, m := range ms {
		if strings.EqualFold(m.Email, address) {
			return true, nil
		}
	}
	return false, nil
}
