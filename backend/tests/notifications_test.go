//go:build integration

package tests

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"github.com/quixgit/greenops-reliabilix/backend/internal/app"
	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/repository"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/secretbox"
)

// recordingChannel stands in for a real channel: it records what would be sent and fails on demand.
type recordingChannel struct {
	kind domain.Kind
	err  error
	sent []domain.Message
	to   []string
}

func (c *recordingChannel) Kind() domain.Kind { return c.kind }
func (c *recordingChannel) Prepare(a string) (string, string, error) {
	return strings.ToLower(a), "hint", nil
}
func (c *recordingChannel) Send(_ context.Context, t domain.Target, m domain.Message) error {
	c.sent, c.to = append(c.sent, m), append(c.to, t.Address)
	return c.err
}

type notifEnv struct {
	apiSvc        application.Service // what the API process runs
	workerSvc     application.Service // what the worker process runs
	apiRepo       repository.Postgres
	workerRepo    repository.Postgres
	ch            *recordingChannel
	queue         *taskRec
	tenantA, tenB string
}

func newNotifEnv(t *testing.T) (*notifEnv, context.Context) {
	t.Helper()
	ctx := context.Background()
	ownerPool, apiPool, workerPool := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_API_DSN"), pool(t, "TEST_WORKER_DSN")
	box, err := secretbox.New([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	ch := &recordingChannel{kind: domain.KindWebhook}
	q := &taskRec{}
	mk := func(r repository.Postgres) application.Service {
		return application.Service{Repo: r, Channels: domain.NewRegistry(ch), Box: box, Queue: q, BaseURL: "https://app.example.com"}
	}
	e := &notifEnv{ch: ch, queue: q, apiRepo: repository.Postgres{Pool: apiPool}, workerRepo: repository.Postgres{Pool: workerPool}}
	e.apiSvc, e.workerSvc = mk(e.apiRepo), mk(e.workerRepo)
	e.tenantA, e.tenB = org(t, ownerPool, "NA-"+t.Name()), org(t, ownerPool, "NB-"+t.Name())
	t.Cleanup(func() {
		for _, tn := range []string{e.tenantA, e.tenB} {
			_, _ = ownerPool.Exec(ctx, `DELETE FROM notifications.deliveries WHERE tenant_id = $1`, tn)
			_, _ = ownerPool.Exec(ctx, `DELETE FROM notifications.destinations WHERE tenant_id = $1`, tn)
			_, _ = ownerPool.Exec(ctx, `DELETE FROM audit.audit_logs WHERE tenant_id = $1`, tn)
			_, _ = ownerPool.Exec(ctx, `DELETE FROM tenants.organizations WHERE id = $1`, tn)
		}
	})
	return e, ctx
}

func adminOf(tenant string) auth.Claims {
	return auth.Claims{Subject: "auth0|admin", TenantID: tenant, Role: auth.RoleAdmin}
}

// The whole path on a real database: create (API role) -> publish (worker role) -> deliver (worker role).
func TestNotificationsOutboxEndToEnd(t *testing.T) {
	e, ctx := newNotifEnv(t)
	actor := adminOf(e.tenantA)
	created, err := e.apiSvc.CreateDestination(auth.WithClaims(ctx, actor), e.tenantA, actor,
		application.CreateInput{Kind: domain.KindWebhook, Name: "ops", Target: "https://Hooks.example.com/in/TOKEN"})
	if err != nil {
		t.Fatal(err)
	}

	// The address is stored sealed: nothing in the table reveals it.
	var sealed, hint string
	ownerPool := pool(t, "TEST_OWNER_DSN")
	if err := ownerPool.QueryRow(ctx, `SELECT target_sealed, target_hint FROM notifications.destinations WHERE id = $1`, created.Destination.ID).Scan(&sealed, &hint); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sealed, "example") || strings.Contains(sealed, "TOKEN") || !strings.HasPrefix(sealed, "v1:") {
		t.Errorf("target stored in the clear: %q", sealed)
	}

	ev := notify.Event{Type: notify.TypeReportReady, Severity: notify.Info, Title: "Report ready", Path: "/reports", DedupeKey: "report:1:ready"}
	for range 3 { // a job redelivered three times must still produce one delivery
		if err := e.workerSvc.Publish(ctx, e.tenantA, ev); err != nil {
			t.Fatal(err)
		}
	}
	task, ok := e.queue.pop()
	if !ok || task.Type != "notifications:deliver" {
		t.Fatalf("expected a deliver task, got %v", e.queue.types())
	}
	if _, more := e.queue.pop(); more {
		t.Error("the duplicate publishes must not queue more tasks")
	}
	list, err := e.workerRepo.ListDeliveries(ctx, e.tenantA, 10)
	if err != nil || len(list) != 1 || list[0].Status != domain.Pending {
		t.Fatalf("deliveries = %+v, %v", list, err)
	}

	if err := e.workerSvc.Deliver(ctx, e.tenantA, list[0].ID); err != nil {
		t.Fatal(err)
	}
	if len(e.ch.sent) != 1 || e.ch.to[0] != "https://hooks.example.com/in/token" { // the test channel lowercases in Prepare
		t.Fatalf("sent to %v", e.ch.to)
	}
	if e.ch.sent[0].Event.Link != "https://app.example.com/reports" {
		t.Errorf("link = %q", e.ch.sent[0].Event.Link)
	}
	// Delivering again is a no-op.
	if err := e.workerSvc.Deliver(ctx, e.tenantA, list[0].ID); err != nil || len(e.ch.sent) != 1 {
		t.Errorf("second Deliver: err=%v sent=%d", err, len(e.ch.sent))
	}
	got, _ := e.apiRepo.ListDeliveries(ctx, e.tenantA, 10)
	if len(got) != 1 || got[0].Status != domain.Sent || got[0].SentAt == nil || got[0].Attempts != 1 {
		t.Errorf("after delivery: %+v", got)
	}
}

func TestNotificationsRetryBackoffAndSweep(t *testing.T) {
	e, ctx := newNotifEnv(t)
	actor := adminOf(e.tenantA)
	if _, err := e.apiSvc.CreateDestination(auth.WithClaims(ctx, actor), e.tenantA, actor, application.CreateInput{Kind: domain.KindWebhook, Name: "ops", Target: "https://x.example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := e.workerSvc.Publish(ctx, e.tenantA, notify.Event{Type: notify.TypeReportFailed, Severity: notify.Warning, Title: "Failed"}); err != nil {
		t.Fatal(err)
	}
	list, _ := e.workerRepo.ListDeliveries(ctx, e.tenantA, 10)
	id := list[0].ID

	e.ch.err = errors.New("endpoint answered HTTP 503")
	if err := e.workerSvc.Deliver(ctx, e.tenantA, id); err != nil {
		t.Fatal(err)
	}
	d, _ := e.workerRepo.ListDeliveries(ctx, e.tenantA, 10)
	if d[0].Status != domain.Pending || d[0].Attempts != 1 || d[0].LastError == nil || !d[0].NextAttemptAt.After(time.Now().Add(20*time.Second)) {
		t.Fatalf("after a transient failure: %+v", d[0])
	}
	// Not due: a premature job must not send again.
	before := len(e.ch.sent)
	_ = e.workerSvc.Deliver(ctx, e.tenantA, id)
	if len(e.ch.sent) != before {
		t.Error("sent before the backoff elapsed")
	}

	// The sweep ignores it until it is due (plus the 30 s grace), then finds it.
	due, err := e.workerRepo.ListDue(ctx, 1000)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range due {
		if x.DeliveryID == id {
			t.Fatal("a delivery in backoff must not be due")
		}
	}
	owner := pool(t, "TEST_OWNER_DSN")
	if _, err := owner.Exec(ctx, `UPDATE notifications.deliveries SET next_attempt_at = now() - interval '1 minute' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	due, _ = e.workerRepo.ListDue(ctx, 1000)
	found := false
	for _, x := range due {
		found = found || (x.DeliveryID == id && x.TenantID == e.tenantA)
	}
	if !found {
		t.Fatal("the sweep must find a due delivery across tenants")
	}

	// A held lease keeps the sweep and other workers away.
	if _, err := owner.Exec(ctx, `UPDATE notifications.deliveries SET lease_until = now() + interval '1 minute' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	due, _ = e.workerRepo.ListDue(ctx, 1000)
	for _, x := range due {
		if x.DeliveryID == id {
			t.Fatal("a leased delivery must not be swept")
		}
	}
	before = len(e.ch.sent)
	_ = e.workerSvc.Deliver(ctx, e.tenantA, id)
	if len(e.ch.sent) != before {
		t.Error("a leased delivery must not be sent by a second worker")
	}

	// Lease expired + endpoint healthy again: the delivery completes.
	if _, err := owner.Exec(ctx, `UPDATE notifications.deliveries SET lease_until = now() - interval '1 second' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	e.ch.err = nil
	if err := e.workerSvc.Deliver(ctx, e.tenantA, id); err != nil {
		t.Fatal(err)
	}
	d, _ = e.workerRepo.ListDeliveries(ctx, e.tenantA, 10)
	if d[0].Status != domain.Sent || d[0].Attempts != 2 || d[0].LastError != nil {
		t.Errorf("after recovery: %+v", d[0])
	}
}

func TestNotificationsTenantIsolation(t *testing.T) {
	e, ctx := newNotifEnv(t)
	actorA := adminOf(e.tenantA)
	a, err := e.apiSvc.CreateDestination(auth.WithClaims(ctx, actorA), e.tenantA, actorA, application.CreateInput{Kind: domain.KindWebhook, Name: "A", Target: "https://a.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if l, _ := e.apiSvc.ListDestinations(ctx, e.tenB); len(l) != 0 {
		t.Errorf("tenant B sees %d destinations of tenant A", len(l))
	}
	if _, err := e.apiRepo.GetDestination(ctx, e.tenB, a.Destination.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("tenant B reads A's destination: %v", err)
	}
	if err := e.apiSvc.DeleteDestination(auth.WithClaims(ctx, adminOf(e.tenB)), e.tenB, a.Destination.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("tenant B deletes A's destination: %v", err)
	}
	if _, err := e.apiSvc.Test(ctx, e.tenB, a.Destination.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("tenant B tests A's destination: %v", err)
	}
	// An event of tenant B never reaches A's destination, and A's deliveries cannot be claimed as B.
	if err := e.workerSvc.Publish(ctx, e.tenB, notify.Event{Type: notify.TypeReportReady, Severity: notify.Info, Title: "B"}); err != nil {
		t.Fatal(err)
	}
	if l, _ := e.workerRepo.ListDeliveries(ctx, e.tenantA, 10); len(l) != 0 {
		t.Errorf("tenant A got %d deliveries from B's event", len(l))
	}
	if _, err := e.apiSvc.Test(ctx, e.tenantA, a.Destination.ID); err != nil {
		t.Fatal(err)
	}
	l, _ := e.workerRepo.ListDeliveries(ctx, e.tenantA, 10)
	if err := e.workerSvc.Deliver(ctx, e.tenB, l[0].ID); err != nil || len(e.ch.sent) != 0 {
		t.Errorf("delivering A's row as tenant B: err=%v sent=%d", err, len(e.ch.sent))
	}
}

func TestNotificationsDeleteKeepsHistoryAndSkipsPending(t *testing.T) {
	e, ctx := newNotifEnv(t)
	actor := adminOf(e.tenantA)
	c, err := e.apiSvc.CreateDestination(auth.WithClaims(ctx, actor), e.tenantA, actor, application.CreateInput{Kind: domain.KindWebhook, Name: "gone", Target: "https://g.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := e.apiSvc.Test(ctx, e.tenantA, c.Destination.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.apiSvc.DeleteDestination(auth.WithClaims(ctx, actor), e.tenantA, c.Destination.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.workerSvc.Deliver(ctx, e.tenantA, id); err != nil {
		t.Fatal(err)
	}
	l, _ := e.apiSvc.ListDeliveries(ctx, e.tenantA, 10)
	if len(l) != 1 || l[0].Status != domain.Skipped || l[0].DestinationID != nil || l[0].DestinationName != "gone" || len(e.ch.sent) != 0 {
		t.Errorf("history after delete: %+v sent=%d", l, len(e.ch.sent))
	}
}

func TestNotificationsRolePrivileges(t *testing.T) {
	e, ctx := newNotifEnv(t)
	actor := adminOf(e.tenantA)
	c, err := e.apiSvc.CreateDestination(auth.WithClaims(ctx, actor), e.tenantA, actor, application.CreateInput{Kind: domain.KindWebhook, Name: "p", Target: "https://p.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	// The worker publishes and sends, but cannot create, change or delete destinations.
	if _, err := e.workerRepo.SetEnabled(ctx, e.tenantA, c.Destination.ID, false); err == nil {
		t.Error("the worker role must not update destinations")
	}
	if err := e.workerRepo.DeleteDestination(ctx, e.tenantA, c.Destination.ID); err == nil {
		t.Error("the worker role must not delete destinations")
	}
	// The API inserts deliveries (test messages) but never edits or deletes them.
	id, err := e.apiSvc.Test(ctx, e.tenantA, c.Destination.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.apiRepo.MarkSent(ctx, e.tenantA, id); err == nil {
		t.Error("the API role must not update deliveries")
	}
	// Cross-tenant maintenance is reachable by the worker only through the SECURITY DEFINER functions.
	if _, err := e.workerRepo.Purge(ctx, time.Hour); err == nil {
		t.Error("purging with less than 7 days of retention must be refused by the function")
	}
	if n, err := e.workerRepo.Purge(ctx, 30*24*time.Hour); err != nil || n < 0 {
		t.Errorf("Purge = %d, %v", n, err)
	}
	if _, err := e.apiRepo.ListDue(ctx, 10); err == nil {
		t.Error("the API role must not call the fan-out function")
	}
}

func TestNotificationsPurgeRemovesOnlyOldFinishedDeliveries(t *testing.T) {
	e, ctx := newNotifEnv(t)
	actor := adminOf(e.tenantA)
	c, _ := e.apiSvc.CreateDestination(auth.WithClaims(ctx, actor), e.tenantA, actor, application.CreateInput{Kind: domain.KindWebhook, Name: "p", Target: "https://p.example.com"})
	oldDone, _ := e.apiSvc.Test(ctx, e.tenantA, c.Destination.ID)
	e.queue.tasks = nil
	owner := pool(t, "TEST_OWNER_DSN")
	if _, err := owner.Exec(ctx, `UPDATE notifications.deliveries SET status = 'sent', created_at = now() - interval '200 days' WHERE id = $1`, oldDone); err != nil {
		t.Fatal(err)
	}
	// An old but still pending delivery must survive: it is not finished.
	if _, err := e.apiSvc.Test(ctx, e.tenantA, c.Destination.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, `UPDATE notifications.deliveries SET created_at = now() - interval '200 days' WHERE tenant_id = $1 AND status = 'pending'`, e.tenantA); err != nil {
		t.Fatal(err)
	}
	if _, err := e.workerSvc.Purge(ctx); err != nil {
		t.Fatal(err)
	}
	l, _ := e.workerRepo.ListDeliveries(ctx, e.tenantA, 10)
	if len(l) != 1 || l[0].Status != domain.Pending {
		t.Errorf("after purge: %+v", l)
	}
}

// The HTTP surface and a real producer: the API manages destinations, the worker renders a report, the report
// module publishes report.ready through the notify port, and a signed webhook arrives at the receiver.
func TestNotificationsAPIAndReportProducer(t *testing.T) {
	ctx := context.Background()
	apiPool, workerPool := pool(t, "TEST_API_DSN"), pool(t, "TEST_WORKER_DSN")

	type hit struct {
		hdr  http.Header
		body []byte
	}
	var mu sync.Mutex
	var hits []hit
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		hits = append(hits, hit{r.Header.Clone(), b})
		mu.Unlock()
	}))
	defer receiver.Close()
	received := func() []hit { mu.Lock(); defer mu.Unlock(); return append([]hit(nil), hits...) }

	cfg := devConfig(t.TempDir())
	cfg.PublicBaseURL = "http://localhost:3000"
	recA, recW := &taskRec{}, &taskRec{}
	apiC, err := app.Build(ctx, cfg, quietLog(), apiPool, recA)
	if err != nil {
		t.Fatal(err)
	}
	workerC, err := app.Build(ctx, cfg, quietLog(), workerPool, recW)
	if err != nil {
		t.Fatal(err)
	}
	mux := asynq.NewServeMux()
	for _, j := range workerC.JobRegistrars() {
		j.RegisterJobs(mux)
	}
	srv := httptest.NewServer(app.NewRouter(cfg, quietLog(), apiC.Verifier, apiC.Resolver, apiPool.Ping, apiC.Modules()...))
	defer srv.Close()
	c := api{t: t, srv: srv}

	rid := fmt.Sprint(time.Now().UnixNano())
	aliceEmail := "alice-" + rid + "@example.com"
	alice := fmt.Sprintf("dev:alice-%s:-:-:%s", rid, aliceEmail)
	c.must("POST", "/api/v1/onboarding", alice, `{"organization_name":"Notif Co"}`, 201)

	// authentication and authorization
	c.must("GET", "/api/v1/notifications/destinations", "", "", 401)
	ci := str(c.must("POST", "/api/v1/api-keys", alice, `{"name":"ci","role":"ci"}`, 201), "key")
	c.must("GET", "/api/v1/notifications/destinations", ci, "", 403)
	c.must("POST", "/api/v1/notifications/destinations", ci, `{"kind":"webhook","name":"x","target":"https://x.example.com"}`, 403)

	// catalog: every channel this deployment offers, every event type
	cat := c.must("GET", "/api/v1/notifications/catalog", alice, "", 200)
	if cat["configured"] != true || len(cat["channels"].([]any)) != 3 || len(cat["event_types"].([]any)) < 3 {
		t.Fatalf("catalog = %v", cat)
	}

	// validation
	for name, body := range map[string]string{
		"unknown channel":     `{"kind":"carrier-pigeon","name":"x","target":"coop"}`,
		"slack on a bad host": `{"kind":"slack","name":"x","target":"https://evil.example.com/services/T/B/X"}`,
		"e-mail to stranger":  `{"kind":"email","name":"x","target":"stranger@example.com"}`,
		"bad event type":      `{"kind":"webhook","name":"x","target":"https://x.example.com","event_types":["nope"]}`,
		"unknown field":       `{"kind":"webhook","name":"x","target":"https://x.example.com","admin":true}`,
		"no name":             `{"kind":"webhook","target":"https://x.example.com"}`,
	} {
		if code, _, raw := c.do("POST", "/api/v1/notifications/destinations", alice, body); code != 422 {
			t.Errorf("%s: status %d, want 422: %s", name, code, raw)
		}
	}

	// create a webhook (the dev build lets the receiver run on localhost) and an e-mail destination
	wh := c.must("POST", "/api/v1/notifications/destinations", alice,
		fmt.Sprintf(`{"kind":"webhook","name":"ops-webhook","target":"%s/hook?token=SECRET-TOKEN"}`, receiver.URL), 201)
	secret := str(wh, "signing_secret")
	whID := str(sub(wh, "destination"), "id")
	if len(secret) != 64 || whID == "" || str(sub(wh, "destination"), "target_hint") != "127.0.0.1" {
		t.Fatalf("webhook destination = %v", wh)
	}
	em := c.must("POST", "/api/v1/notifications/destinations", alice,
		fmt.Sprintf(`{"kind":"email","name":"me","target":"%s","event_types":["report.ready"]}`, strings.ToUpper(aliceEmail)), 201)
	if em["signing_secret"] != nil || !strings.HasSuffix(str(sub(em, "destination"), "target_hint"), "@example.com") {
		t.Fatalf("email destination = %v", em)
	}

	// the list never exposes an address or a token
	listRaw := c.must("GET", "/api/v1/notifications/destinations", alice, "", 200)
	if len(items(listRaw)) != 2 {
		t.Fatalf("destinations = %v", listRaw)
	}
	if _, _, raw := c.do("GET", "/api/v1/notifications/destinations", alice, ""); strings.Contains(string(raw), "SECRET-TOKEN") || strings.Contains(string(raw), "target_sealed") {
		t.Fatalf("the list leaks the address: %s", raw)
	}

	// a real producer: the worker renders a report and announces it
	today := time.Now().UTC().Format(time.DateOnly)
	rep := c.must("POST", "/api/v1/reports", alice, fmt.Sprintf(`{"kind":"carbon","format":"csv","period_start":"%s","period_end":"%s"}`, today, today), 202)
	pump(t, mux, recA, recW)
	if r := c.must("GET", "/api/v1/reports/"+str(rep, "id"), alice, "", 200); str(r, "status") != "ready" {
		t.Fatalf("report = %v", r)
	}
	got := received()
	if len(got) != 1 {
		t.Fatalf("receiver got %d requests, want the report.ready webhook", len(got))
	}
	h := got[0]
	if h.hdr.Get("X-Reliabilix-Event") != "report.ready" || !strings.HasPrefix(h.hdr.Get("User-Agent"), "Reliabilix") {
		t.Errorf("headers = %v", h.hdr)
	}
	var payload map[string]any
	if err := json.Unmarshal(h.body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["type"] != "report.ready" || payload["link"] != "http://localhost:3000/reports" || sub(payload, "attrs")["report_id"] != str(rep, "id") ||
		payload["id"] != h.hdr.Get("X-Reliabilix-Delivery") || payload["tenant_id"] == "" {
		t.Errorf("payload = %v", payload)
	}
	// the receiver verifies the signature with the secret from the creation response
	var ts, sig string
	for _, part := range strings.Split(h.hdr.Get("X-Reliabilix-Signature"), ",") {
		if v, ok := strings.CutPrefix(part, "t="); ok {
			ts = v
		}
		if v, ok := strings.CutPrefix(part, "v1="); ok {
			sig = v
		}
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "." + string(h.body)))
	if ts == "" || sig != hex.EncodeToString(mac.Sum(nil)) {
		t.Errorf("signature %q does not verify", h.hdr.Get("X-Reliabilix-Signature"))
	}

	// both destinations were served and the history says so
	dels := items(c.must("GET", "/api/v1/notifications/deliveries", alice, "", 200))
	if len(dels) != 2 {
		t.Fatalf("deliveries = %v", dels)
	}
	for _, d := range dels {
		m := d.(map[string]any)
		if m["status"] != "sent" || m["event_type"] != "report.ready" || m["attempts"] != float64(1) {
			t.Errorf("delivery = %v", m)
		}
	}
	c.must("GET", "/api/v1/notifications/deliveries?limit=0", alice, "", 400)

	// test message, disable, delete
	tm := c.must("POST", "/api/v1/notifications/destinations/"+whID+"/test", alice, "", 202)
	if str(tm, "delivery_id") == "" {
		t.Fatalf("test = %v", tm)
	}
	pump(t, mux, recA, recW)
	if got := received(); len(got) != 2 || got[1].hdr.Get("X-Reliabilix-Event") != "notification.test" {
		t.Fatalf("test message not received: %d requests", len(got))
	}
	if d := c.must("POST", "/api/v1/notifications/destinations/"+whID+"/disable", alice, "", 200); d["enabled"] != false {
		t.Fatalf("disable = %v", d)
	}
	c.must("POST", "/api/v1/reports", alice, fmt.Sprintf(`{"kind":"carbon","format":"json","period_start":"%s","period_end":"%s"}`, today, today), 202)
	pump(t, mux, recA, recW)
	if got := received(); len(got) != 2 {
		t.Errorf("a disabled destination received %d requests", len(got))
	}
	if d := c.must("POST", "/api/v1/notifications/destinations/"+whID+"/enable", alice, "", 200); d["enabled"] != true {
		t.Fatalf("enable = %v", d)
	}
	c.must("DELETE", "/api/v1/notifications/destinations/"+whID, alice, "", 204)
	c.must("DELETE", "/api/v1/notifications/destinations/"+whID, alice, "", 404)
	c.must("POST", "/api/v1/notifications/destinations/not-a-uuid/test", alice, "", 404)
	if len(items(c.must("GET", "/api/v1/notifications/destinations", alice, "", 200))) != 1 {
		t.Error("only the e-mail destination should remain")
	}

	// the audit log records who changed destinations, without the address
	audit := c.must("GET", "/api/v1/audit-log?action=notification_destination.created", alice, "", 200)
	if len(items(audit)) != 2 {
		t.Errorf("audit entries = %v", audit)
	}
	if _, _, raw := c.do("GET", "/api/v1/audit-log", alice, ""); strings.Contains(string(raw), "SECRET-TOKEN") {
		t.Error("the audit log leaks the address")
	}
}
