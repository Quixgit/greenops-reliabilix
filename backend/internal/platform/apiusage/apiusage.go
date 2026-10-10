// Package apiusage meters calls to paid external APIs and enforces a daily cap, so a retry loop or a mistake cannot run
// up a bill (cost controls). It counts requests and failures per provider and day, in memory for speed and in the
// database so the count survives restarts and is shared by the api and the worker.
package apiusage

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrBudget is returned instead of making a request once the day's cap is reached.
var ErrBudget = errors.New("the daily request budget of this provider is used up")

// Meter counts and caps requests of one process. The cap is read on every check (it is a console setting).
type Meter struct {
	Pool *pgxpool.Pool
	// Cap returns the maximum requests per day for a provider (0 = unlimited).
	Cap func(ctx context.Context, provider string) int

	mu    sync.Mutex
	day   string
	count map[string]int // requests made today by this and (once loaded) other processes
	load  map[string]bool
}

func today() string { return time.Now().UTC().Format(time.DateOnly) }

func (m *Meter) roll() {
	if d := today(); d != m.day {
		m.day, m.count, m.load = d, map[string]int{}, map[string]bool{}
	}
}

// Allow reports whether one more request may be made now.
func (m *Meter) Allow(ctx context.Context, provider string) error {
	limit := 0
	if m.Cap != nil {
		limit = m.Cap(ctx, provider)
	}
	if limit <= 0 {
		return nil
	}
	m.mu.Lock()
	m.roll()
	if !m.load[provider] && m.Pool != nil {
		m.load[provider] = true
		var n int
		if err := m.Pool.QueryRow(ctx, `SELECT coalesce(max(requests), 0) FROM platform.api_usage WHERE provider = $1 AND day = (now() AT TIME ZONE 'utc')::date`, provider).Scan(&n); err == nil && n > m.count[provider] {
			m.count[provider] = n
		}
	}
	used := m.count[provider]
	m.mu.Unlock()
	if used >= limit {
		return ErrBudget
	}
	return nil
}

// Record counts one finished request (failed or not).
func (m *Meter) Record(ctx context.Context, provider string, failed bool) {
	m.mu.Lock()
	m.roll()
	m.count[provider]++
	m.mu.Unlock()
	if m.Pool == nil {
		return
	}
	errs := 0
	if failed {
		errs = 1
	}
	c, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	_, _ = m.Pool.Exec(c, `SELECT platform.record_api_usage($1, 1, $2)`, provider, errs)
}
