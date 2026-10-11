// Package application holds the notification use-cases: publish an event (outbox), deliver it (job), sweep
// stuck deliveries, and manage destinations.
//
// Delivery is at-least-once. A delivery is a database row; a worker takes a short lease on it, sends, then records
// the result. If the process dies after sending but before recording, the lease expires and the message is sent
// again: webhook receivers can deduplicate on the delivery id (X-Reliabilix-Delivery).
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/entitlements"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/errlog"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/observability"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
)

// Sealer protects destination addresses at rest (platform/secretbox.Box).
type Sealer interface {
	Configured() bool
	Seal(plaintext, aad string) (string, error)
	Open(sealed, aad string) (string, error)
	Derive(purpose, id string) string
}

// Directory answers who belongs to a tenant. It is a port: the composition root satisfies it from the tenants
// module, so this domain never imports it.
type Directory interface {
	IsMemberEmail(ctx context.Context, tenantID, email string) (bool, error)
}

const (
	secretPurpose = "destination-secret"
	leaseFor      = 2 * time.Minute
	sweepBatch    = 500
	maxErrorLen   = 300
)

// Inbox keeps the in-product copy of every event and the per-person read mark. Optional: without it there is no bell feed.
type Inbox interface {
	InboxAdd(ctx context.Context, tenantID string, e notify.Event) (created bool, err error)
	InboxList(ctx context.Context, tenantID, userID string, limit int) ([]domain.InboxItem, time.Time, error)
	InboxMarkRead(ctx context.Context, tenantID, userID string) error
}

type Service struct {
	Inbox     Inbox
	Repo      domain.Repository
	Channels  *domain.Registry
	Box       Sealer
	Queue     queue.Enqueuer
	Directory Directory
	// BaseURL is the public address of the web app ("https://app.example.com"); empty = events carry no link.
	BaseURL string
	// Stubbed lists channels that run on a development stand-in (e-mail that is only logged).
	Stubbed []domain.Kind
	Log     *slog.Logger
	// Errors is optional: a delivery that fails for good is reported to the operator error log.
	Errors errlog.Recorder
	// Settings and Limits are optional (catalog defaults; the built-in destination cap) - ADR-0023.
	Settings settings.Reader
	Limits   entitlements.Gate
	Now      func() time.Time
}

func (s Service) cfg() settings.Reader {
	if s.Settings != nil {
		return s.Settings
	}
	return settings.Defaults{}
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s Service) logger() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.New(slog.DiscardHandler)
}

var _ notify.Notifier = Service{}

// Publish implements notify.Notifier: it stores one pending delivery per matching destination, then queues them.
// Rows are written first, so an enqueue failure is harmless: the sweep picks them up.
func (s Service) Publish(ctx context.Context, tenantID string, e notify.Event) error {
	return s.publish(ctx, tenantID, e, false)
}

// publish keeps the event in the inbox (unless the caller already did) and queues it for the matching destinations.
func (s Service) publish(ctx context.Context, tenantID string, e notify.Event, inboxDone bool) error {
	if tenantID == "" {
		return errors.New("notifications: empty tenant id")
	}
	if err := e.Validate(); err != nil {
		return err
	}
	if !s.cfg().Bool(ctx, tenantID, settings.KeyNotifEnabled) {
		return nil // the organization switched notifications off: events are not queued, not delivered later
	}
	if s.Inbox != nil && !inboxDone {
		created, err := s.Inbox.InboxAdd(ctx, tenantID, e)
		if err != nil {
			s.logger().Warn("inbox: item not kept", "tenant", tenantID, "type", e.Type, "err", err)
		} else if !created && e.DedupeKey != "" {
			return nil // this event was announced before: nothing goes out a second time
		}
	}
	if e.Path != "" && s.BaseURL != "" {
		e.Link = strings.TrimRight(s.BaseURL, "/") + e.Path
	}
	all, err := s.Repo.ListDestinations(ctx, tenantID)
	if err != nil {
		return err
	}
	var targets []domain.Destination
	for _, d := range all {
		if _, err := s.Channels.Get(d.Kind); err == nil && d.Matches(e) {
			targets = append(targets, d)
		}
	}
	if len(targets) == 0 {
		return nil
	}
	return s.store(ctx, tenantID, targets, e)
}

func (s Service) store(ctx context.Context, tenantID string, targets []domain.Destination, e notify.Event) error {
	ids, err := s.Repo.InsertDeliveries(ctx, tenantID, targets, e)
	if err != nil {
		return err
	}
	for _, id := range ids {
		s.enqueue(ctx, tenantID, id)
	}
	return nil
}

func (s Service) enqueue(ctx context.Context, tenantID, deliveryID string) {
	if s.Queue == nil {
		return
	}
	// The queue is a fast path only: if this fails the sweep re-queues the row within a minute or two.
	if err := s.Queue.Enqueue(ctx, queue.TaskDeliverNotification, queue.TenantPayload{TenantID: tenantID, RefID: deliveryID},
		asynq.MaxRetry(2), asynq.Unique(time.Minute)); err != nil {
		s.logger().Warn("notification not queued; the sweep will pick it up", "delivery", deliveryID, "err", err)
	}
}

// Deliver is the notifications:deliver job.
func (s Service) Deliver(ctx context.Context, tenantID, deliveryID string) error {
	d, dest, ok, err := s.Repo.Claim(ctx, tenantID, deliveryID, leaseFor)
	if err != nil || !ok {
		return err
	}
	channel := string(d.Kind)
	if dest == nil {
		observability.NotificationDeliveries.WithLabelValues(channel, "skipped").Inc()
		return s.Repo.MarkFailed(ctx, tenantID, deliveryID, domain.Skipped, "the destination was removed")
	}

	sendErr := s.send(ctx, tenantID, d, *dest)
	if sendErr == nil {
		observability.NotificationDeliveries.WithLabelValues(channel, "sent").Inc()
		return s.Repo.MarkSent(ctx, tenantID, deliveryID)
	}
	reason := truncate(sendErr.Error())
	switch {
	case domain.IsPermanent(sendErr):
		observability.NotificationDeliveries.WithLabelValues(channel, "failed").Inc()
		s.reportFailure(tenantID, channel, errlog.Warning, reason) // usually a wrong or revoked endpoint: the tenant's to fix
		return s.Repo.MarkFailed(ctx, tenantID, deliveryID, domain.Failed, reason)
	case int64(d.Attempts) >= s.cfg().Int(ctx, "", settings.KeyNotifMaxAttempts):
		observability.NotificationDeliveries.WithLabelValues(channel, "failed").Inc()
		s.reportFailure(tenantID, channel, errlog.Error, "gave up after retries: "+reason)
		return s.Repo.MarkFailed(ctx, tenantID, deliveryID, domain.Failed, fmt.Sprintf("gave up after %d attempts: %s", d.Attempts, reason))
	default:
		observability.NotificationDeliveries.WithLabelValues(channel, "retry").Inc()
		base := time.Duration(s.cfg().Int(ctx, "", settings.KeyNotifRetryBaseSeconds)) * time.Second
		return s.Repo.MarkRetry(ctx, tenantID, deliveryID, s.now().Add(domain.BackoffFrom(base, d.Attempts)), reason)
	}
}

func (s Service) reportFailure(tenantID, channel string, sev errlog.Severity, reason string) {
	if s.Errors != nil {
		s.Errors.Record(errlog.Event{Service: "notifications", Component: "channel " + channel, Severity: sev, Message: reason, TenantID: tenantID})
	}
}

func (s Service) send(ctx context.Context, tenantID string, d domain.Delivery, dest domain.Destination) error {
	ch, err := s.Channels.Get(dest.Kind)
	if err != nil {
		return errors.New("this channel is not available on this deployment")
	}
	if !s.Box.Configured() {
		return errors.New("notifications are not configured on this deployment")
	}
	addr, err := s.Box.Open(dest.SealedTarget, aad(tenantID, dest.ID))
	if err != nil {
		return errors.New("the stored address cannot be decrypted (was the key changed?)")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg().Int(ctx, "", settings.KeyNotifSendTimeoutSeconds))*time.Second)
	defer cancel()
	return ch.Send(ctx, domain.Target{DestinationID: dest.ID, Address: addr, Secret: s.Box.Derive(secretPurpose, dest.ID)},
		domain.Message{DeliveryID: d.ID, TenantID: tenantID, Attempt: d.Attempts, CreatedAt: d.CreatedAt, Event: d.Event})
}

// Sweep queues every due delivery (the notifications:sweep job). It is what makes the outbox reliable: retries,
// deliveries whose queue message was lost, and deliveries abandoned by a crashed worker all come back here.
func (s Service) Sweep(ctx context.Context) (int, error) {
	due, err := s.Repo.ListDue(ctx, sweepBatch)
	if err != nil {
		return 0, err
	}
	for _, d := range due {
		s.enqueue(ctx, d.TenantID, d.DeliveryID)
	}
	return len(due), nil
}

// Purge deletes finished deliveries past the retention period (the notifications:purge job).
func (s Service) Purge(ctx context.Context) (int64, error) {
	return s.Repo.Purge(ctx, time.Duration(s.cfg().Int(ctx, "", settings.KeyNotifRetentionDays))*24*time.Hour)
}

func truncate(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	if len(s) > maxErrorLen {
		return s[:maxErrorLen]
	}
	return s
}

func aad(tenantID, destinationID string) string { return tenantID + ":" + destinationID }

// ---- destinations -----------------------------------------------------------------------------------------

// CreateInput is what a tenant supplies for a new destination.
type CreateInput struct {
	Kind        domain.Kind
	Name        string
	Target      string
	EventTypes  []string
	MinSeverity notify.Severity
}

// Created is the new destination plus, for channels that sign their requests, the signing secret. The secret is
// derived from the platform key, never stored, and returned on creation only.
type Created struct {
	Destination   domain.Destination `json:"destination"`
	SigningSecret string             `json:"signing_secret,omitempty"`
}

func (s Service) CreateDestination(ctx context.Context, tenantID string, actor auth.Claims, in CreateInput) (Created, error) {
	if !s.Box.Configured() {
		return Created{}, domain.ErrNotConfigured
	}
	ch, err := s.Channels.Get(in.Kind)
	if err != nil {
		return Created{}, err
	}
	if a, ok := ch.(domain.AvailableChannel); ok && !a.Available(ctx) {
		return Created{}, domain.ErrUnknownKind // not set up on this platform (yet): the same answer as a channel that does not exist
	}
	d := domain.Destination{Kind: in.Kind, Name: strings.TrimSpace(in.Name), EventTypes: in.EventTypes, MinSeverity: in.MinSeverity, Enabled: true, CreatedBy: actor.Subject}
	if len(d.EventTypes) == 0 {
		d.EventTypes = []string{notify.Wildcard}
	}
	if d.MinSeverity == "" {
		d.MinSeverity = notify.Info
	}
	if err := d.Validate(); err != nil {
		return Created{}, err
	}
	addr, hint, err := ch.Prepare(in.Target)
	if err != nil {
		return Created{}, err
	}
	if pc, ok := ch.(domain.PersonalChannel); ok && pc.PersonalAddress() {
		member, err := s.Directory.IsMemberEmail(ctx, tenantID, addr)
		if err != nil {
			return Created{}, err
		}
		if !member {
			return Created{}, fmt.Errorf("%w: the address must belong to a member of your organization", domain.ErrInvalid)
		}
	}
	if s.Limits != nil {
		if err := s.Limits.Allow(ctx, tenantID, entitlements.Destinations); err != nil {
			return Created{}, err
		}
	} else if n, err := s.Repo.CountDestinations(ctx, tenantID); err != nil {
		return Created{}, err
	} else if n >= domain.MaxDestinationsPerTenant {
		return Created{}, domain.ErrLimit
	}
	d.ID = uuid.NewString()
	d.Hint = hint
	if d.SealedTarget, err = s.Box.Seal(addr, aad(tenantID, d.ID)); err != nil {
		return Created{}, err
	}
	stored, err := s.Repo.InsertDestination(ctx, tenantID, d)
	if err != nil {
		return Created{}, err
	}
	out := Created{Destination: stored}
	if sc, ok := ch.(domain.SigningChannel); ok && sc.UsesSigningSecret() {
		out.SigningSecret = s.Box.Derive(secretPurpose, stored.ID)
	}
	return out, nil
}

func (s Service) ListDestinations(ctx context.Context, tenantID string) ([]domain.Destination, error) {
	return s.Repo.ListDestinations(ctx, tenantID)
}

func (s Service) SetEnabled(ctx context.Context, tenantID, id string, enabled bool) (domain.Destination, error) {
	return s.Repo.SetEnabled(ctx, tenantID, id, enabled)
}

func (s Service) DeleteDestination(ctx context.Context, tenantID, id string) error {
	return s.Repo.DeleteDestination(ctx, tenantID, id)
}

func (s Service) ListDeliveries(ctx context.Context, tenantID string, limit int) ([]domain.Delivery, error) {
	return s.Repo.ListDeliveries(ctx, tenantID, limit)
}

// Test queues a test message to one destination, whatever its filters say, and returns the delivery id.
func (s Service) Test(ctx context.Context, tenantID, id string) (string, error) {
	d, err := s.Repo.GetDestination(ctx, tenantID, id)
	if err != nil {
		return "", err
	}
	if _, err := s.Channels.Get(d.Kind); err != nil {
		return "", err
	}
	e := notify.Event{Type: notify.TypeNotificationTest, Severity: notify.Info, Title: "Reliabilix test notification",
		Body: "If you can read this, the destination \"" + d.Name + "\" works.", Path: "/"}
	if s.BaseURL != "" {
		e.Link = strings.TrimRight(s.BaseURL, "/") + e.Path
	}
	ids, err := s.Repo.InsertDeliveries(ctx, tenantID, []domain.Destination{d}, e)
	if err != nil || len(ids) == 0 {
		return "", err
	}
	s.enqueue(ctx, tenantID, ids[0])
	return ids[0], nil
}

// Catalog describes what can be configured: the channels this deployment offers and the event types.
type Catalog struct {
	Channels   []domain.Kind     `json:"channels"`
	EventTypes []notify.TypeInfo `json:"event_types"`
	Configured bool              `json:"configured"`
	Stubbed    []domain.Kind     `json:"stubbed"`
}

func (s Service) Catalog(ctx context.Context) Catalog {
	stubbed := s.Stubbed
	if stubbed == nil {
		stubbed = []domain.Kind{}
	}
	return Catalog{Channels: s.Channels.KindsAvailable(ctx), EventTypes: notify.Catalog(), Configured: s.Box.Configured(), Stubbed: stubbed}
}

// PublishWithCopy publishes the event (destinations and inbox) and, the first time only, e-mails it to the given
// members. A person who is not on the list never gets it, and an address is never taken from the event.
func (s Service) PublishWithCopy(ctx context.Context, tenantID string, e notify.Event, to []string) error {
	if err := e.Validate(); err != nil {
		return err
	}
	if !s.cfg().Bool(ctx, tenantID, settings.KeyNotifEnabled) {
		return nil
	}
	if s.Inbox != nil {
		created, err := s.Inbox.InboxAdd(ctx, tenantID, e)
		if err != nil {
			return err
		}
		if !created && e.DedupeKey != "" {
			return nil // announced before: neither the destinations nor the mail go out again
		}
	}
	if err := s.publish(ctx, tenantID, e, true); err != nil {
		return err
	}
	if len(to) == 0 {
		return nil
	}
	ch, err := s.Channels.Get(domain.KindEmail)
	if err != nil {
		return nil // e-mail is not available on this deployment
	}
	if a, ok := ch.(domain.AvailableChannel); ok && !a.Available(ctx) {
		return nil
	}
	if e.Path != "" && s.BaseURL != "" && e.Link == "" {
		e.Link = strings.TrimRight(s.BaseURL, "/") + e.Path
	}
	for _, addr := range to {
		norm, _, err := ch.Prepare(addr)
		if err != nil {
			continue
		}
		sctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		err = ch.Send(sctx, domain.Target{Address: norm}, domain.Message{TenantID: tenantID, Attempt: 1, CreatedAt: s.now(), Event: e})
		cancel()
		if err != nil {
			s.logger().Warn("billing copy not sent", "tenant", tenantID, "err", err)
		}
	}
	return nil
}

// InboxFeed is what the bell shows: the kept items filtered by the organization's display settings, with the
// unread mark of this person.
func (s Service) InboxFeed(ctx context.Context, tenantID, userID string, limit int) (items []domain.InboxItem, unread int, err error) {
	items = []domain.InboxItem{}
	if s.Inbox == nil {
		return items, 0, nil
	}
	cfg := s.cfg()
	if !cfg.Bool(ctx, tenantID, settings.KeyInboxEnabled) {
		return items, 0, nil
	}
	min := notify.Severity(cfg.String(ctx, tenantID, settings.KeyInboxMinSeverity))
	muted := map[string]bool{}
	for _, t := range cfg.Strings(ctx, tenantID, settings.KeyInboxMutedTypes) {
		muted[t] = true
	}
	all, readUntil, err := s.Inbox.InboxList(ctx, tenantID, userID, 200)
	if err != nil {
		return nil, 0, err
	}
	for _, it := range all {
		// An alert is shown by the alerts feed, with its own acknowledge and mute: not twice.
		if it.Type == notify.TypeAlertOpened || muted[it.Type] {
			continue
		}
		essential := strings.HasPrefix(it.Type, "billing.")
		if !essential && it.Severity.Rank() < min.Rank() {
			continue
		}
		it.Unread = it.CreatedAt.After(readUntil)
		if it.Unread {
			unread++
		}
		if len(items) < limit {
			items = append(items, it)
		}
	}
	return items, unread, nil
}

// InboxRead marks everything seen for this person.
func (s Service) InboxRead(ctx context.Context, tenantID, userID string) error {
	if s.Inbox == nil {
		return nil
	}
	return s.Inbox.InboxMarkRead(ctx, tenantID, userID)
}

// Setting exposes the settings reader the service uses (the bell reads the sound switch through it).
func (s Service) Setting() settings.Reader { return s.cfg() }
