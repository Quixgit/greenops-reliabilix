package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/operator/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/config"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/integrations"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/storage"
)

// diagnostics are the platform's self-tests: each looks at one dependency or one invariant and says ok, warn or fail with a short
// message that is safe to show (no secret, payload or tenant data). They are read-only, except the storage round trip, which
// writes one small object under "diagnostics/" and deletes it.
func diagnostics(cfg config.Config, pool *pgxpool.Pool, ints *integrations.Store, port domain.IntegrationsPort, insp *queue.Inspector, store storage.Store) []domain.DiagCheck {
	ok, warn, fail, skip := domain.DiagOK, domain.DiagWarn, domain.DiagFail, domain.DiagSkipped
	var out []domain.DiagCheck
	add := func(id, group, name, desc string, run func(ctx context.Context) (domain.DiagStatus, string)) {
		out = append(out, domain.DiagCheck{ID: id, Group: group, Name: name, Description: desc, Run: run})
	}

	// ---- Core ----
	add("db.ping", "Core", "Database answers", "A round trip to PostgreSQL, and the database clock against this server's.", func(ctx context.Context) (domain.DiagStatus, string) {
		t0 := time.Now()
		var dbNow time.Time
		if err := pool.QueryRow(ctx, "SELECT now()").Scan(&dbNow); err != nil {
			return fail, "the database did not answer"
		}
		ms, skew := time.Since(t0).Milliseconds(), time.Since(dbNow)
		if skew < 0 {
			skew = -skew
		}
		switch {
		case skew > 10*time.Second:
			return warn, fmt.Sprintf("answered in %d ms, but the clock differs by %s", ms, skew.Round(time.Second))
		case ms > 300:
			return warn, fmt.Sprintf("slow: %d ms", ms)
		}
		return ok, fmt.Sprintf("%d ms, clock within %s", ms, skew.Round(time.Millisecond))
	})
	add("db.rls", "Core", "Row-level security on tenant tables", "Every table with a tenant_id that the API role can read has row-level security enabled.", func(ctx context.Context) (domain.DiagStatus, string) {
		rows, err := pool.Query(ctx, `SELECT n.nspname || '.' || c.relname FROM pg_class c
			JOIN pg_namespace n ON n.oid = c.relnamespace
			JOIN pg_attribute a ON a.attrelid = c.oid AND a.attname = 'tenant_id' AND NOT a.attisdropped
			WHERE c.relkind IN ('r', 'p') AND NOT c.relispartition AND NOT c.relrowsecurity
			  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
			  AND has_table_privilege(current_user, c.oid, 'SELECT') -- a table this role cannot read cannot leak through it
			ORDER BY 1`)
		if err != nil {
			return fail, "the catalog could not be read"
		}
		defer rows.Close()
		var bad []string
		for rows.Next() {
			var t string
			if rows.Scan(&t) == nil {
				bad = append(bad, t)
			}
		}
		if len(bad) > 0 {
			return fail, fmt.Sprintf("%d table(s) with tenant_id and no row-level security: %s", len(bad), strings.Join(bad[:min(len(bad), 5)], ", "))
		}
		return ok, "every table with a tenant_id has it enabled"
	})
	add("db.partitions", "Core", "Partitions exist", "The partitions of this month and the next exist for usage and calculations.", func(ctx context.Context) (domain.DiagStatus, string) {
		now := time.Now().UTC()
		var missing []string
		for _, t := range []string{"usage.usage_records", "carbon.calculations"} {
			for _, m := range []time.Time{now, now.AddDate(0, 1, 0)} {
				var n int
				suffix := m.Format("2006_01")
				err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
					WHERE i.inhparent = $1::regclass AND c.relname LIKE '%\_' || $2`, t, suffix).Scan(&n)
				if err != nil {
					return fail, "the catalog could not be read"
				}
				if n == 0 {
					missing = append(missing, t+" "+suffix)
				}
			}
		}
		if len(missing) > 0 {
			return warn, "missing: " + strings.Join(missing, ", ") + " (the scheduler creates them daily)"
		}
		return ok, "this month and the next exist"
	})
	add("queue.redis", "Core", "Queue answers", "Redis answers, and no queue is paused or waiting longer than five minutes.", func(ctx context.Context) (domain.DiagStatus, string) {
		if insp == nil {
			return skip, "no queue inspector"
		}
		t0 := time.Now()
		qs, err := insp.Queues()
		if err != nil {
			return fail, "Redis did not answer"
		}
		for _, q := range qs {
			if q.Paused {
				return warn, "queue " + q.Name + " is paused"
			}
			if q.LatencySeconds > 300 {
				return warn, fmt.Sprintf("queue %s: the oldest task waited %.0f s", q.Name, q.LatencySeconds)
			}
		}
		return ok, fmt.Sprintf("%d queue(s), answered in %d ms", len(qs), time.Since(t0).Milliseconds())
	})
	add("queue.workers", "Core", "Worker and scheduler run", "At least one worker is listening and the scheduler has registered its periodic jobs.", func(ctx context.Context) (domain.DiagStatus, string) {
		if insp == nil {
			return skip, "no queue inspector"
		}
		n, err := insp.Workers()
		if err != nil {
			return fail, "Redis did not answer"
		}
		sc, err := insp.Schedules()
		switch {
		case err != nil:
			return fail, "Redis did not answer"
		case n == 0:
			return fail, "no worker is connected: queued jobs will not run"
		case len(sc) == 0:
			return warn, fmt.Sprintf("%d worker(s), but the scheduler has registered nothing", n)
		}
		return ok, fmt.Sprintf("%d worker(s), %d periodic job(s) registered", n, len(sc))
	})
	add("queue.failed", "Core", "No tasks are stuck", "Tasks that are waiting for a retry or gave up.", func(ctx context.Context) (domain.DiagStatus, string) {
		if insp == nil {
			return skip, "no queue inspector"
		}
		f, err := insp.Failed(200)
		if err != nil {
			return fail, "Redis did not answer"
		}
		if len(f) == 0 {
			return ok, "none"
		}
		gaveUp := 0
		for _, t := range f {
			if t.State == "archived" {
				gaveUp++
			}
		}
		if gaveUp > 0 {
			return warn, fmt.Sprintf("%d task(s) gave up, %d waiting for a retry (see Jobs & scheduler)", gaveUp, len(f)-gaveUp)
		}
		return ok, fmt.Sprintf("%d waiting for a retry", len(f))
	})
	add("storage.roundtrip", "Core", "Object storage works", "Writes a small object, reads it back and deletes it.", func(ctx context.Context) (domain.DiagStatus, string) {
		if store == nil {
			return skip, "no object storage"
		}
		key := "diagnostics/" + uuid.NewString()
		want := []byte("reliabilix diagnostics " + time.Now().UTC().Format(time.RFC3339))
		t0 := time.Now()
		if err := store.Put(ctx, key, bytes.NewReader(want)); err != nil {
			return fail, "the object could not be written"
		}
		defer func() { _ = store.Delete(context.WithoutCancel(ctx), key) }()
		r, err := store.Get(ctx, key)
		if err != nil {
			return fail, "the object could not be read back"
		}
		defer func() { _ = r.Close() }()
		got, err := io.ReadAll(io.LimitReader(r, 4096))
		if err != nil || !bytes.Equal(got, want) {
			return fail, "the object read back differs from what was written"
		}
		if cfg.S3Bucket == "" {
			return warn, fmt.Sprintf("works (%d ms), but it is a local directory, not S3", time.Since(t0).Milliseconds())
		}
		return ok, fmt.Sprintf("write, read and delete in %d ms", time.Since(t0).Milliseconds())
	})

	// ---- Configuration and security ----
	add("config.production", "Security", "Production configuration", "A production or staging deployment has what it must have: a notification key, S3, no development sign-in.", func(ctx context.Context) (domain.DiagStatus, string) {
		if cfg.IsDev() {
			return skip, "this is a development deployment"
		}
		var bad []string
		if cfg.NotificationsKey == "" {
			bad = append(bad, "NOTIFICATIONS_SECRET_KEY is not set")
		}
		if cfg.S3Bucket == "" {
			bad = append(bad, "S3_BUCKET is not set")
		}
		if cfg.SMTPAllowPlain {
			bad = append(bad, "plain SMTP is allowed")
		}
		if len(bad) > 0 {
			return fail, strings.Join(bad, "; ")
		}
		return ok, "as required"
	})
	add("config.sealed", "Security", "Secrets are sealed", "The key that seals stored secrets is configured, so keys entered in the console are never kept in clear text.", func(ctx context.Context) (domain.DiagStatus, string) {
		if cfg.NotificationsKey == "" {
			if cfg.IsDev() {
				return warn, "a fixed development key is in use"
			}
			return fail, "no sealing key"
		}
		return ok, "configured"
	})

	// ---- Services and data sources (the same tests as the Test buttons) ----
	add("mail.smtp", "Services", "E-mail server", "Connects to the mail server and signs in. Sends nothing.", func(ctx context.Context) (domain.DiagStatus, string) {
		if ints.Value(ctx, "smtp", "host") == "" {
			return skip, "not configured: e-mail is only logged"
		}
		msg, err := port.Test(ctx, "smtp")
		if err != nil {
			return fail, err.Error()
		}
		return ok, msg
	})
	for _, id := range []string{"electricitymaps", "ember", "eia", "entsoe", "climatetrace", "ons", "gcp_pricing"} {
		meta, found := integrationMeta(id)
		if !found {
			continue
		}
		add("source."+id, "Data sources", meta, "The integration's own test with the values saved in the console.", func(ctx context.Context) (domain.DiagStatus, string) {
			if needsKey(ints, ctx, id) {
				return skip, "no key is configured"
			}
			msg, err := port.Test(ctx, id)
			if err != nil {
				return fail, err.Error()
			}
			return ok, msg
		})
	}
	for _, s := range []struct{ id, name, url string }{
		{"uk_carbon_intensity", "UK Carbon Intensity", "https://api.carbonintensity.org.uk/intensity"},
		{"energy_charts", "Energy-Charts", "https://api.energy-charts.info/public_power?country=de&start=" + time.Now().UTC().Format("2006-01-02")},
		{"ieso", "IESO", "https://reports-public.ieso.ca/public/GenOutputbyFuelHourly/"},
		{"hydroquebec", "Hydro-Québec", "https://donnees.hydroquebec.com/api/explore/v2.1/catalog/datasets"},
		{"azure_prices", "Azure Retail Prices", "https://prices.azure.com/api/retail/prices?$top=1"},
		{"aws_prices", "AWS price list", "https://b0.p.awsstatic.com/pricing/2.0/meteredUnitMaps/ec2/USD/current/ec2-ondemand-without-sec-sel/US%20East%20(N.%20Virginia)/Linux/index.json"},
	} {
		add("reach."+s.id, "Data sources", s.name+" is reachable", "An HTTPS request to the public endpoint answers (no key needed).", func(ctx context.Context) (domain.DiagStatus, string) {
			return reach(ctx, s.url)
		})
	}
	return out
}

// reach asks a public endpoint with a short timeout and reports how it answered. It follows no redirect and reads no body.
func reach(ctx context.Context, url string) (domain.DiagStatus, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return domain.DiagFail, "bad address"
	}
	c := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	t0 := time.Now()
	resp, err := c.Do(req)
	if err != nil {
		return domain.DiagFail, "no answer"
	}
	_ = resp.Body.Close()
	ms := time.Since(t0).Milliseconds()
	switch {
	case resp.StatusCode >= 500:
		return domain.DiagFail, fmt.Sprintf("HTTP %d in %d ms", resp.StatusCode, ms)
	case resp.StatusCode >= 400 && resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed:
		return domain.DiagWarn, fmt.Sprintf("HTTP %d in %d ms", resp.StatusCode, ms)
	}
	return domain.DiagOK, fmt.Sprintf("HTTP %d in %d ms", resp.StatusCode, ms)
}

// integrationMeta is the display name of an integration of the catalog.
func integrationMeta(id string) (string, bool) {
	for _, i := range integrations.Catalog() {
		if i.ID == id {
			return i.Name, true
		}
	}
	return "", false
}

// needsKey reports whether an integration has a required secret that is not set (so a test would only say "no key").
func needsKey(ints *integrations.Store, ctx context.Context, id string) bool {
	for _, i := range integrations.Catalog() {
		if i.ID != id {
			continue
		}
		// A source whose only secret is optional (a price list key) is skipped without it, as one whose required key is missing.
		secrets, set := 0, 0
		for _, f := range i.Fields {
			if f.Secret {
				secrets++
				if ints.Value(ctx, id, f.Key) != "" {
					set++
				}
			}
		}
		return secrets > 0 && set == 0
	}
	return false
}
