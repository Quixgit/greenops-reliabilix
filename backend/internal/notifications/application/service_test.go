package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/secretbox"
)

// ---- fakes -------------------------------------------------------------------------------------------------

type memRepo struct {
	mu     sync.Mutex
	dests  map[string]domain.Destination
	dels   map[string]*domain.Delivery
	keys   map[string]bool // destination|dedupe
	order  []string
	now    func() time.Time
	nextID int
}

func newRepo(now func() time.Time) *memRepo {
	return &memRepo{dests: map[string]domain.Destination{}, dels: map[string]*domain.Delivery{}, keys: map[string]bool{}, now: now}
}

func (r *memRepo) CountDestinations(context.Context, string) (int, error) { return len(r.dests), nil }
func (r *memRepo) InsertDestination(_ context.Context, _ string, d domain.Destination) (domain.Destination, error) {
	d.CreatedAt = r.now()
	r.dests[d.ID] = d
	return d, nil
}
func (r *memRepo) ListDestinations(context.Context, string) ([]domain.Destination, error) {
	out := []domain.Destination{}
	for _, d := range r.dests {
		out = append(out, d)
	}
	return out, nil
}
func (r *memRepo) GetDestination(_ context.Context, _, id string) (domain.Destination, error) {
	d, ok := r.dests[id]
	if !ok {
		return d, domain.ErrNotFound
	}
	return d, nil
}
func (r *memRepo) SetEnabled(_ context.Context, _, id string, en bool) (domain.Destination, error) {
	d, ok := r.dests[id]
	if !ok {
		return d, domain.ErrNotFound
	}
	d.Enabled = en
	r.dests[id] = d
	return d, nil
}
func (r *memRepo) DeleteDestination(_ context.Context, _, id string) error {
	if _, ok := r.dests[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.dests, id)
	return nil
}
func (r *memRepo) InsertDeliveries(_ context.Context, _ string, ds []domain.Destination, e notify.Event) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var ids []string
	for _, d := range ds {
		if e.DedupeKey != "" {
			k := d.ID + "|" + e.DedupeKey
			if r.keys[k] {
				continue
			}
			r.keys[k] = true
		}
		r.nextID++
		id := fmt.Sprintf("delivery-%d", r.nextID)
		did := d.ID
		r.dels[id] = &domain.Delivery{ID: id, DestinationID: &did, Kind: d.Kind, DestinationName: d.Name, EventType: e.Type, Severity: e.Severity,
			Status: domain.Pending, NextAttemptAt: r.now(), CreatedAt: r.now(), Event: e}
		r.order = append(r.order, id)
		ids = append(ids, id)
	}
	return ids, nil
}
func (r *memRepo) ListDeliveries(context.Context, string, int) ([]domain.Delivery, error) {
	var out []domain.Delivery
	for _, id := range r.order {
		out = append(out, *r.dels[id])
	}
	return out, nil
}
func (r *memRepo) Claim(_ context.Context, _, id string, _ time.Duration) (domain.Delivery, *domain.Destination, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.dels[id]
	if !ok || d.Status != domain.Pending || d.NextAttemptAt.After(r.now()) {
		return domain.Delivery{}, nil, false, nil
	}
	d.Attempts++
	var dest *domain.Destination
	if d.DestinationID != nil {
		if x, ok := r.dests[*d.DestinationID]; ok {
			dest = &x
		}
	}
	return *d, dest, true, nil
}
func (r *memRepo) MarkSent(_ context.Context, _, id string) error {
	r.dels[id].Status = domain.Sent
	return nil
}
func (r *memRepo) MarkRetry(_ context.Context, _, id string, next time.Time, reason string) error {
	r.dels[id].NextAttemptAt, r.dels[id].LastError = next, &reason
	return nil
}
func (r *memRepo) MarkFailed(_ context.Context, _, id string, st domain.Status, reason string) error {
	r.dels[id].Status, r.dels[id].LastError = st, &reason
	return nil
}
func (r *memRepo) ListDue(context.Context, int) ([]domain.DueRef, error) {
	var out []domain.DueRef
	for _, id := range r.order {
		if d := r.dels[id]; d.Status == domain.Pending && !d.NextAttemptAt.After(r.now()) {
			out = append(out, domain.DueRef{TenantID: "t1", DeliveryID: id})
		}
	}
	return out, nil
}
func (r *memRepo) Purge(context.Context, time.Duration) (int64, error) { return 0, nil }

type fakeChannel struct {
	kind     domain.Kind
	personal bool
	signing  bool
	sendErr  error
	sent     []domain.Message
	targets  []domain.Target
}

func (f *fakeChannel) Kind() domain.Kind { return f.kind }
func (f *fakeChannel) Prepare(a string) (string, string, error) {
	if a == "" || strings.Contains(a, "bad") {
		return "", "", domain.ErrInvalid
	}
	return strings.ToLower(a), "hint:" + strings.ToLower(a)[:1], nil
}
func (f *fakeChannel) Send(_ context.Context, t domain.Target, m domain.Message) error {
	f.targets = append(f.targets, t)
	f.sent = append(f.sent, m)
	return f.sendErr
}
func (f *fakeChannel) PersonalAddress() bool   { return f.personal }
func (f *fakeChannel) UsesSigningSecret() bool { return f.signing }

type recQueue struct{ tasks []queue.TenantPayload }

func (q *recQueue) Enqueue(_ context.Context, typ string, p any, _ ...asynq.Option) error {
	if typ == queue.TaskDeliverNotification {
		q.tasks = append(q.tasks, p.(queue.TenantPayload))
	}
	return nil
}

type dir map[string]bool

func (d dir) IsMemberEmail(_ context.Context, _, email string) (bool, error) { return d[email], nil }

// ---- harness ------------------------------------------------------------------------------------------------

type env struct {
	svc  Service
	repo *memRepo
	q    *recQueue
	hook *fakeChannel
	mail *fakeChannel
	now  time.Time
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{now: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC), q: &recQueue{}}
	e.repo = newRepo(func() time.Time { return e.now })
	e.hook = &fakeChannel{kind: domain.KindWebhook, signing: true}
	e.mail = &fakeChannel{kind: domain.KindEmail, personal: true}
	box, err := secretbox.New([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	e.svc = Service{Repo: e.repo, Channels: domain.NewRegistry(e.hook, e.mail), Box: box, Queue: e.q, Directory: dir{"ann@example.com": true},
		BaseURL: "https://app.example.com/", Now: func() time.Time { return e.now }}
	return e
}

var admin = auth.Claims{Subject: "auth0|admin", TenantID: "t1"}

func (e *env) create(t *testing.T, kind domain.Kind, target string, types ...string) Created {
	t.Helper()
	c, err := e.svc.CreateDestination(context.Background(), "t1", admin, CreateInput{Kind: kind, Name: "dest", Target: target, EventTypes: types})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

var reportReady = notify.Event{Type: notify.TypeReportReady, Severity: notify.Info, Title: "Report ready", Path: "/reports", DedupeKey: "report:1:ready"}

// ---- destinations -------------------------------------------------------------------------------------------

func TestCreateDestinationSealsTheAddressAndReturnsTheSecretOnce(t *testing.T) {
	e := newEnv(t)
	c := e.create(t, domain.KindWebhook, "https://Hooks.example.com/x")
	if c.SigningSecret == "" || len(c.SigningSecret) != 64 {
		t.Errorf("a signing channel must return a derived secret, got %q", c.SigningSecret)
	}
	stored := e.repo.dests[c.Destination.ID]
	if strings.Contains(stored.SealedTarget, "example.com") || !strings.HasPrefix(stored.SealedTarget, "v1:") {
		t.Errorf("the address must be stored sealed, got %q", stored.SealedTarget)
	}
	if got, err := e.svc.Box.Open(stored.SealedTarget, "t1:"+stored.ID); err != nil || got != "https://hooks.example.com/x" {
		t.Errorf("sealed address = %q, %v", got, err)
	}
	if _, err := e.svc.Box.Open(stored.SealedTarget, "t2:"+stored.ID); err == nil {
		t.Error("the address must be bound to its tenant")
	}
	if c.Destination.Hint != "hint:h" || !c.Destination.Enabled || c.Destination.CreatedBy != "auth0|admin" {
		t.Errorf("destination = %+v", c.Destination)
	}
	// defaults: every event, all severities
	if len(c.Destination.EventTypes) != 1 || c.Destination.EventTypes[0] != "*" || c.Destination.MinSeverity != notify.Info {
		t.Errorf("defaults = %v %v", c.Destination.EventTypes, c.Destination.MinSeverity)
	}
	if m := e.create(t, domain.KindEmail, "ann@example.com"); m.SigningSecret != "" {
		t.Error("channels that do not sign must not get a secret")
	}
}

func TestCreateDestinationRules(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	in := func(kind domain.Kind, target string) CreateInput {
		return CreateInput{Kind: kind, Name: "n", Target: target}
	}
	cases := []struct {
		name string
		in   CreateInput
		want error
	}{
		{"unknown channel", in("sms", "123"), domain.ErrUnknownKind},
		{"bad address", in(domain.KindWebhook, "bad"), domain.ErrInvalid},
		{"e-mail to a stranger", in(domain.KindEmail, "stranger@example.com"), domain.ErrInvalid},
		{"empty name", CreateInput{Kind: domain.KindWebhook, Target: "https://x.example.com"}, domain.ErrInvalid},
		{"unknown event", CreateInput{Kind: domain.KindWebhook, Name: "n", Target: "https://x.example.com", EventTypes: []string{"nope"}}, domain.ErrInvalid},
		{"bad severity", CreateInput{Kind: domain.KindWebhook, Name: "n", Target: "https://x.example.com", MinSeverity: "loud"}, domain.ErrInvalid},
	}
	for _, c := range cases {
		if _, err := e.svc.CreateDestination(ctx, "t1", admin, c.in); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
	if len(e.repo.dests) != 0 {
		t.Error("nothing may be stored for a rejected request")
	}
	if _, err := e.svc.CreateDestination(ctx, "t1", admin, in(domain.KindEmail, "Ann@Example.com")); err != nil {
		t.Errorf("member e-mail (any case) must be accepted: %v", err)
	}

	for i := len(e.repo.dests); i < domain.MaxDestinationsPerTenant; i++ {
		e.create(t, domain.KindWebhook, "https://x.example.com")
	}
	if _, err := e.svc.CreateDestination(ctx, "t1", admin, in(domain.KindWebhook, "https://x.example.com")); !errors.Is(err, domain.ErrLimit) {
		t.Errorf("over the limit: %v", err)
	}

	e.svc.Box = (*secretbox.Box)(nil)
	if _, err := e.svc.CreateDestination(ctx, "t1", admin, in(domain.KindWebhook, "https://x.example.com")); !errors.Is(err, domain.ErrNotConfigured) {
		t.Errorf("without a key: %v", err)
	}
	if e.svc.Catalog(context.Background()).Configured {
		t.Error("catalog must report an unconfigured deployment")
	}
}

// ---- publish ------------------------------------------------------------------------------------------------

func TestPublishRoutesByTypeSeverityAndEnabled(t *testing.T) {
	e := newEnv(t)
	all := e.create(t, domain.KindWebhook, "https://all.example.com")
	_, _ = e.svc.CreateDestination(context.Background(), "t1", admin, CreateInput{Kind: domain.KindWebhook, Name: "f", Target: "https://f.example.com",
		EventTypes: []string{notify.TypeReportFailed}})
	_, _ = e.svc.CreateDestination(context.Background(), "t1", admin, CreateInput{Kind: domain.KindWebhook, Name: "w", Target: "https://w.example.com",
		MinSeverity: notify.Warning})
	off := e.create(t, domain.KindWebhook, "https://off.example.com")
	if _, err := e.svc.SetEnabled(context.Background(), "t1", off.Destination.ID, false); err != nil {
		t.Fatal(err)
	}

	if err := e.svc.Publish(context.Background(), "t1", reportReady); err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, d := range e.repo.dels {
		got[*d.DestinationID] = true
	}
	if len(e.repo.dels) != 1 || !got[all.Destination.ID] {
		t.Fatalf("only the catch-all destination wants an info report.ready: %v", got)
	}
	if len(e.q.tasks) != 1 || e.q.tasks[0].TenantID != "t1" || e.q.tasks[0].RefID == "" {
		t.Errorf("queued = %+v", e.q.tasks)
	}
	for _, d := range e.repo.dels {
		if d.Event.Link != "https://app.example.com/reports" {
			t.Errorf("link = %q, want the absolute app URL", d.Event.Link)
		}
	}

	// a warning also reaches the warning-level destination, and the failed-only one for its type
	e.repo.dels, e.repo.order, e.q.tasks = map[string]*domain.Delivery{}, nil, nil
	if err := e.svc.Publish(context.Background(), "t1", notify.Event{Type: notify.TypeReportFailed, Severity: notify.Warning, Title: "Report failed"}); err != nil {
		t.Fatal(err)
	}
	if len(e.repo.dels) != 3 {
		t.Errorf("a warning report.failed reaches the catch-all, the failed-only and the warning destinations, got %d", len(e.repo.dels))
	}
}

func TestPublishIsIdempotentPerDedupeKey(t *testing.T) {
	e := newEnv(t)
	e.create(t, domain.KindWebhook, "https://all.example.com")
	for range 3 {
		if err := e.svc.Publish(context.Background(), "t1", reportReady); err != nil {
			t.Fatal(err)
		}
	}
	if len(e.repo.dels) != 1 || len(e.q.tasks) != 1 {
		t.Errorf("the same event must be stored and queued once, got %d deliveries / %d tasks", len(e.repo.dels), len(e.q.tasks))
	}
}

func TestPublishRejectsBadInputAndNoDestinationIsNotAnError(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if err := e.svc.Publish(ctx, "", reportReady); err == nil {
		t.Error("an empty tenant must be refused")
	}
	if err := e.svc.Publish(ctx, "t1", notify.Event{Type: "made.up", Severity: notify.Info, Title: "x"}); !errors.Is(err, notify.ErrInvalidEvent) {
		t.Errorf("unknown type: %v", err)
	}
	if err := e.svc.Publish(ctx, "t1", reportReady); err != nil {
		t.Errorf("no destinations configured is the normal case: %v", err)
	}
	if len(e.repo.dels) != 0 {
		t.Error("nothing to deliver")
	}
}

// ---- deliver ------------------------------------------------------------------------------------------------

func (e *env) publishOne(t *testing.T) string {
	t.Helper()
	if err := e.svc.Publish(context.Background(), "t1", reportReady); err != nil {
		t.Fatal(err)
	}
	return e.q.tasks[len(e.q.tasks)-1].RefID
}

func TestDeliverSendsWithDecryptedAddressAndSecret(t *testing.T) {
	e := newEnv(t)
	c := e.create(t, domain.KindWebhook, "https://all.example.com/hook")
	id := e.publishOne(t)
	if err := e.svc.Deliver(context.Background(), "t1", id); err != nil {
		t.Fatal(err)
	}
	if e.repo.dels[id].Status != domain.Sent {
		t.Fatalf("status = %s", e.repo.dels[id].Status)
	}
	tg, m := e.hook.targets[0], e.hook.sent[0]
	if tg.Address != "https://all.example.com/hook" || tg.Secret != c.SigningSecret || tg.DestinationID != c.Destination.ID {
		t.Errorf("target = %+v", tg)
	}
	if m.DeliveryID != id || m.TenantID != "t1" || m.Attempt != 1 || m.Event.Title != "Report ready" || m.Event.Link != "https://app.example.com/reports" {
		t.Errorf("message = %+v", m)
	}
}

func TestDeliverIsANoOpWhenAlreadyHandled(t *testing.T) {
	e := newEnv(t)
	e.create(t, domain.KindWebhook, "https://all.example.com")
	id := e.publishOne(t)
	for range 3 {
		if err := e.svc.Deliver(context.Background(), "t1", id); err != nil {
			t.Fatal(err)
		}
	}
	if len(e.hook.sent) != 1 {
		t.Errorf("a redelivered job must not send twice, sent %d", len(e.hook.sent))
	}
	if err := e.svc.Deliver(context.Background(), "t1", "missing"); err != nil {
		t.Errorf("an unknown delivery is not an error: %v", err)
	}
}

func TestDeliverRetriesWithBackoffThenGivesUp(t *testing.T) {
	e := newEnv(t)
	e.create(t, domain.KindWebhook, "https://all.example.com")
	id := e.publishOne(t)
	e.hook.sendErr = errors.New("endpoint answered HTTP 503")

	for attempt := 1; attempt <= domain.MaxAttempts; attempt++ {
		if err := e.svc.Deliver(context.Background(), "t1", id); err != nil {
			t.Fatal(err)
		}
		d := e.repo.dels[id]
		if attempt < domain.MaxAttempts {
			if d.Status != domain.Pending || !d.NextAttemptAt.Equal(e.now.Add(domain.Backoff(attempt))) {
				t.Fatalf("attempt %d: status=%s next=%v, want retry in %v", attempt, d.Status, d.NextAttemptAt, domain.Backoff(attempt))
			}
			// not due yet: a premature job must not send
			before := len(e.hook.sent)
			_ = e.svc.Deliver(context.Background(), "t1", id)
			if len(e.hook.sent) != before {
				t.Fatalf("attempt %d: sent before the backoff elapsed", attempt)
			}
			e.now = e.now.Add(domain.Backoff(attempt))
		}
	}
	d := e.repo.dels[id]
	if d.Status != domain.Failed || d.LastError == nil || !strings.Contains(*d.LastError, "gave up after 8 attempts") || !strings.Contains(*d.LastError, "503") {
		t.Errorf("final state = %s / %v", d.Status, d.LastError)
	}
	if len(e.hook.sent) != domain.MaxAttempts {
		t.Errorf("sent %d times, want %d", len(e.hook.sent), domain.MaxAttempts)
	}
}

func TestDeliverRecoversAfterATransientFailure(t *testing.T) {
	e := newEnv(t)
	e.create(t, domain.KindWebhook, "https://all.example.com")
	id := e.publishOne(t)
	e.hook.sendErr = errors.New("request timed out")
	_ = e.svc.Deliver(context.Background(), "t1", id)
	e.hook.sendErr = nil
	e.now = e.now.Add(time.Minute)
	if err := e.svc.Deliver(context.Background(), "t1", id); err != nil {
		t.Fatal(err)
	}
	if d := e.repo.dels[id]; d.Status != domain.Sent || d.Attempts != 2 {
		t.Errorf("status=%s attempts=%d", d.Status, d.Attempts)
	}
}

func TestDeliverPermanentFailureStopsImmediately(t *testing.T) {
	e := newEnv(t)
	e.create(t, domain.KindWebhook, "https://all.example.com")
	id := e.publishOne(t)
	e.hook.sendErr = domain.Permanent(errors.New("endpoint rejected the request with HTTP 410"))
	if err := e.svc.Deliver(context.Background(), "t1", id); err != nil {
		t.Fatal(err)
	}
	if d := e.repo.dels[id]; d.Status != domain.Failed || d.Attempts != 1 {
		t.Errorf("status=%s attempts=%d", d.Status, d.Attempts)
	}
}

func TestDeliverSkipsARemovedDestination(t *testing.T) {
	e := newEnv(t)
	c := e.create(t, domain.KindWebhook, "https://all.example.com")
	id := e.publishOne(t)
	if err := e.svc.DeleteDestination(context.Background(), "t1", c.Destination.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Deliver(context.Background(), "t1", id); err != nil {
		t.Fatal(err)
	}
	if d := e.repo.dels[id]; d.Status != domain.Skipped || len(e.hook.sent) != 0 {
		t.Errorf("status=%s sent=%d", d.Status, len(e.hook.sent))
	}
}

func TestDeliverWithTheWrongKeyRetriesAndNeverSendsGarbage(t *testing.T) {
	e := newEnv(t)
	e.create(t, domain.KindWebhook, "https://all.example.com")
	id := e.publishOne(t)
	other, _ := secretbox.New([]byte(strings.Repeat("z", 32)))
	e.svc.Box = other
	if err := e.svc.Deliver(context.Background(), "t1", id); err != nil {
		t.Fatal(err)
	}
	d := e.repo.dels[id]
	if d.Status != domain.Pending || d.LastError == nil || !strings.Contains(*d.LastError, "cannot be decrypted") || len(e.hook.sent) != 0 {
		t.Errorf("status=%s err=%v sent=%d", d.Status, d.LastError, len(e.hook.sent))
	}
}

func TestTruncateRemovesControlCharactersAndBoundsLength(t *testing.T) {
	got := truncate("line1\r\nline2\x00" + strings.Repeat("x", 1000))
	if len(got) != maxErrorLen || strings.ContainsAny(got, "\r\n\x00") {
		t.Errorf("truncate: len=%d %q", len(got), got[:20])
	}
}

// ---- test message and sweep ---------------------------------------------------------------------------------

func TestTestBypassesFiltersButNotOwnership(t *testing.T) {
	e := newEnv(t)
	c, _ := e.svc.CreateDestination(context.Background(), "t1", admin, CreateInput{Kind: domain.KindWebhook, Name: "crit", Target: "https://c.example.com",
		EventTypes: []string{notify.TypeConnectionError}, MinSeverity: notify.Critical})
	id, err := e.svc.Test(context.Background(), "t1", c.Destination.ID)
	if err != nil || id == "" {
		t.Fatalf("Test = %q, %v", id, err)
	}
	if err := e.svc.Deliver(context.Background(), "t1", id); err != nil {
		t.Fatal(err)
	}
	if len(e.hook.sent) != 1 || e.hook.sent[0].Event.Type != notify.TypeNotificationTest || !strings.Contains(e.hook.sent[0].Event.Body, "crit") {
		t.Errorf("sent = %+v", e.hook.sent)
	}
	if _, err := e.svc.Test(context.Background(), "t1", "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown destination: %v", err)
	}
}

func TestSweepQueuesEveryDueDelivery(t *testing.T) {
	e := newEnv(t)
	e.create(t, domain.KindWebhook, "https://all.example.com")
	e.publishOne(t)
	e.q.tasks = nil // pretend the queue lost the message
	n, err := e.svc.Sweep(context.Background())
	if err != nil || n != 1 || len(e.q.tasks) != 1 {
		t.Errorf("Sweep = %d, %v, queued %d", n, err, len(e.q.tasks))
	}
}
