// Package events defines the cross-service event contract and the Bus
// abstraction. Production implementation: NATS JetStream (ADR-0011).
package events

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// Subjects follow: greenops.<domain>.<event>.v<major>
const (
	SubjectCloudSyncCompleted    = "greenops.cloud.sync_completed.v1"
	SubjectUsageNormalized       = "greenops.usage.normalized.v1"
	SubjectCarbonCalculated      = "greenops.carbon.calculated.v1"
	SubjectRecommendationCreated = "greenops.recommendation.created.v1"
)

// Envelope wraps every event. Consumers MUST be idempotent on ID.
type Envelope struct {
	ID         string          `json:"id"`
	Subject    string          `json:"subject"`
	TenantID   string          `json:"tenant_id"`
	OccurredAt time.Time       `json:"occurred_at"`
	TraceID    string          `json:"trace_id,omitempty"`
	Data       json.RawMessage `json:"data"`
}

// Handler processes an event; returning an error triggers redelivery.
type Handler func(ctx context.Context, e Envelope) error

// Bus publishes and subscribes to events.
type Bus interface {
	Publish(ctx context.Context, e Envelope) error
	Subscribe(subject string, h Handler) error
}

// MemoryBus is a synchronous in-process Bus for tests and local development.
type MemoryBus struct {
	mu   sync.RWMutex
	subs map[string][]Handler
}

func NewMemoryBus() *MemoryBus { return &MemoryBus{subs: map[string][]Handler{}} }

func (b *MemoryBus) Subscribe(subject string, h Handler) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[subject] = append(b.subs[subject], h)
	return nil
}

func (b *MemoryBus) Publish(ctx context.Context, e Envelope) error {
	b.mu.RLock()
	hs := append([]Handler(nil), b.subs[e.Subject]...)
	b.mu.RUnlock()
	for _, h := range hs {
		if err := h(ctx, e); err != nil {
			return err
		}
	}
	return nil
}
