// Package errlog records errors of the platform's own services (api, worker, integrations) in the database so an
// operator can see them without reading logs (ADR-0021). It sits next to logs and Sentry, it does not replace them:
// slog keeps every line, Sentry keeps stack traces, this keeps a small, grouped, queryable picture.
//
// Recording never blocks or fails the caller: events are counted in memory by fingerprint and flushed in the
// background; when the buffer is full new kinds of errors are dropped (and counted), never queued without bound.
package errlog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/observability"
)

type Severity string

const (
	Warning  Severity = "warning"  // expected-ish: a customer's credentials stopped working, a retryable hiccup
	Error    Severity = "error"    // a bug or an outage of something we depend on
	Critical Severity = "critical" // a panic
)

func (s Severity) rank() int {
	switch s {
	case Critical:
		return 3
	case Error:
		return 2
	}
	return 1
}

// Event is one error. Service is the process or integration ("api", "worker", "smtp"), Component the part of it
// ("POST /reports", "cloudaccounts:sync_aws_account").
type Event struct {
	Service   string
	Component string
	Severity  Severity
	Message   string
	TenantID  string
	RequestID string
}

// Recorder is what the rest of the code depends on.
type Recorder interface{ Record(Event) }

// Noop records nothing (tests, and processes without a database).
type Noop struct{}

func (Noop) Record(Event) {}

// Sink collects events in memory (tests).
type Sink struct {
	mu     sync.Mutex
	Events []Event
}

func (s *Sink) Record(e Event) { s.mu.Lock(); s.Events = append(s.Events, e); s.mu.Unlock() }

var (
	reURLCreds = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)[^/\s@]+@`)
	reBearer   = regexp.MustCompile(`(?i)(authorization|bearer|basic|token|apikey|api_key|secret|password|passwd)([=: ]+)(?:(?:bearer|basic)\s+)?\S+`)
	reEmail    = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	reAWSKey   = regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{16}\b`)
	reLongTok  = regexp.MustCompile(`\b[A-Za-z0-9_\-+/=]{32,}\b`)
	reQuery    = regexp.MustCompile(`\?[^\s"']*`)
	reUUID     = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	reHex      = regexp.MustCompile(`\b[0-9a-fA-F]{12,}\b`)
	reNum      = regexp.MustCompile(`\d+`)
	uuidShape  = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

const maxMessage = 500

// Sanitize removes what must never be stored: credentials in URLs, bearer/secret values, e-mail addresses, cloud
// access keys, long token-like strings and query strings. It also strips control characters and bounds the length.
func Sanitize(msg string) string {
	msg = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, msg)
	msg = reURLCreds.ReplaceAllString(msg, "${1}***@")
	msg = reQuery.ReplaceAllString(msg, "?…")
	msg = reBearer.ReplaceAllString(msg, "${1}${2}***")
	msg = reAWSKey.ReplaceAllString(msg, "***")
	msg = reEmail.ReplaceAllString(msg, "***@***")
	msg = reLongTok.ReplaceAllString(msg, "***")
	msg = strings.Join(strings.Fields(msg), " ")
	if len(msg) > maxMessage {
		msg = strings.ToValidUTF8(msg[:maxMessage], "")
	}
	return msg
}

// Fingerprint groups equal errors: ids, hashes and numbers are masked so "account 123 denied" and "account 456
// denied" are one problem.
func Fingerprint(service, component, message string) string {
	m := reUUID.ReplaceAllString(message, "<id>")
	m = reHex.ReplaceAllString(m, "<hex>")
	m = reNum.ReplaceAllString(m, "<n>")
	h := sha256.Sum256([]byte(service + "\x00" + reNum.ReplaceAllString(reUUID.ReplaceAllString(component, "<id>"), "<n>") + "\x00" + m))
	return hex.EncodeToString(h[:])
}

type pending struct {
	e     Event
	count int
	fp    string
}

// PG is the database-backed recorder.
type PG struct {
	pool    *pgxpool.Pool
	log     *slog.Logger
	service string

	mu      sync.Mutex
	buf     map[string]*pending
	stop    chan struct{}
	done    chan struct{}
	maxKeys int
}

// NewPG starts the background flusher. service is the default Event.Service. Call Close on shutdown to flush.
func NewPG(pool *pgxpool.Pool, log *slog.Logger, service string) *PG {
	p := &PG{pool: pool, log: log, service: service, buf: map[string]*pending{}, stop: make(chan struct{}), done: make(chan struct{}), maxKeys: 200}
	go p.loop()
	return p
}

func (p *PG) Record(e Event) {
	if e.Service == "" {
		e.Service = p.service
	}
	if e.Severity == "" {
		e.Severity = Error
	}
	if e.Component == "" {
		e.Component = "unknown"
	}
	e.Message = Sanitize(e.Message)
	e.Component = Sanitize(e.Component)
	if len(e.Component) > 120 {
		e.Component = e.Component[:120]
	}
	fp := Fingerprint(e.Service, e.Component, e.Message)
	p.mu.Lock()
	defer p.mu.Unlock()
	if cur, ok := p.buf[fp]; ok {
		cur.count++
		if e.Severity.rank() > cur.e.Severity.rank() {
			cur.e.Severity = e.Severity
		}
		if e.TenantID != "" {
			cur.e.TenantID = e.TenantID
		}
		if e.RequestID != "" {
			cur.e.RequestID = e.RequestID
		}
		return
	}
	if len(p.buf) >= p.maxKeys {
		observability.ErrorsRecorded.WithLabelValues(e.Service, "dropped").Inc()
		return
	}
	p.buf[fp] = &pending{e: e, count: 1, fp: fp}
}

func (p *PG) loop() {
	defer close(p.done)
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			p.Flush(context.Background())
		case <-p.stop:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			p.Flush(ctx)
			cancel()
			return
		}
	}
}

// Flush writes what has been collected. A failure is logged and the events are dropped: the error log must never
// become a second source of errors.
func (p *PG) Flush(ctx context.Context) {
	p.mu.Lock()
	batch := p.buf
	p.buf = map[string]*pending{}
	p.mu.Unlock()
	for _, x := range batch {
		var tenant, req *string
		if uuidShape.MatchString(x.e.TenantID) { // a malformed id must not cost us the error itself
			tenant = &x.e.TenantID
		}
		if x.e.RequestID != "" {
			req = &x.e.RequestID
		}
		cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		_, err := p.pool.Exec(cctx, `SELECT platform.record_error($1,$2,$3,$4,$5,$6,$7::uuid,$8)`,
			x.fp, x.e.Service, x.e.Component, string(x.e.Severity), x.e.Message, x.count, tenant, req)
		cancel()
		if err != nil {
			if p.log != nil {
				p.log.Warn("error log write failed", "err", err)
			}
			observability.ErrorsRecorded.WithLabelValues(x.e.Service, "failed").Add(float64(x.count))
			continue
		}
		observability.ErrorsRecorded.WithLabelValues(x.e.Service, "recorded").Add(float64(x.count))
	}
}

// Close flushes and stops the background loop.
func (p *PG) Close() {
	close(p.stop)
	<-p.done
}

// Purge deletes resolved and ignored groups older than retain (the platform:purge_errors job).
func (p *PG) Purge(ctx context.Context, retain time.Duration) (n int64, err error) {
	err = p.pool.QueryRow(ctx, `SELECT platform.purge_old_errors(make_interval(secs => $1::float8))`, retain.Seconds()).Scan(&n)
	return
}
