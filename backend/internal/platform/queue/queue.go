// Package queue wraps Asynq (Redis) for background jobs. Task payloads always
// carry tenant_id explicitly: a worker has no HTTP request to take it from.
package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"
)

// Task types. Convention: <domain>:<action>.
const (
	TaskSyncAWSAccount     = "cloudaccounts:sync_aws_account"
	TaskRecommendAll       = "recommendations:calculate_all"
	TaskSyncAll            = "cloudaccounts:sync_all"
	TaskCalculateCarbon    = "carbon:calculate"
	TaskEnsurePartitions   = "platform:ensure_partitions"
	TaskRefreshGrid        = "carbon:refresh_grid_intensity"
	TaskRecalculateAll     = "carbon:recalculate_all"
	TaskSyncAzure          = "cloudaccounts:sync_azure_subscription" // phase 2
	TaskSyncGCP            = "cloudaccounts:sync_gcp_billing"        // phase 2
	TaskSyncKubernetes     = "kubernetes:sync"                       // phase 2
	TaskGenerateReport     = "reports:generate"                      // phase 2
	TaskCalcRecommendation = "recommendations:calculate"             // phase 2
)

// Enqueuer is what domains depend on to chain jobs (sync -> calculate -> recommend). Implemented by
// *Client; tests use a recorder. There is deliberately no event bus: inside the monolith a completed
// job simply enqueues the next one (ADR-0003).
type Enqueuer interface {
	Enqueue(ctx context.Context, taskType string, payload any, opts ...asynq.Option) error
}

// TenantPayload is the minimal payload of tenant-scoped jobs (workers have no request to take it from).
type TenantPayload struct {
	TenantID  string `json:"tenant_id"`
	ProjectID string `json:"project_id,omitempty"`
	RefID     string `json:"ref_id,omitempty"` // connection_id / report_id
}

// WindowPayload is a tenant job over a date window (inclusive from, exclusive to; YYYY-MM-DD).
type WindowPayload struct {
	TenantID  string `json:"tenant_id"`
	ProjectID string `json:"project_id"`
	From      string `json:"from"`
	To        string `json:"to"`
}

// Client enqueues jobs.
type Client struct{ c *asynq.Client }

func NewClient(redisAddr string) *Client {
	return &Client{c: asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})}
}

func (c *Client) Close() error { return c.c.Close() }

// Enqueue schedules a task. Retries and timeouts are bounded by default.
func (c *Client) Enqueue(ctx context.Context, taskType string, p any, opts ...asynq.Option) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	opts = append([]asynq.Option{asynq.MaxRetry(5), asynq.Timeout(10 * 60 * 1e9)}, opts...)
	_, err = c.c.EnqueueContext(ctx, asynq.NewTask(taskType, b), opts...)
	if errors.Is(err, asynq.ErrDuplicateTask) {
		return nil // an identical unique job is already queued: that is the goal, not a failure
	}
	return err
}

// NewServer builds the worker server. onError receives every failed attempt (log + Sentry in the binary).
func NewServer(redisAddr string, concurrency int, onError func(ctx context.Context, taskType string, err error)) *asynq.Server {
	return asynq.NewServer(asynq.RedisClientOpt{Addr: redisAddr}, asynq.Config{
		Concurrency: concurrency,
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, t *asynq.Task, err error) {
			if onError != nil {
				onError(ctx, t.Type(), err)
			}
		}),
		Queues: map[string]int{"default": 6, "critical": 3, "low": 1},
	})
}

// JobRegistrar is implemented by domain modules that process background jobs.
type JobRegistrar interface {
	RegisterJobs(mux *asynq.ServeMux)
}

// DecodeAs parses a task payload. Malformed payloads are not retried.
func DecodeAs[T any](t *asynq.Task) (T, error) {
	var p T
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return p, fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}
	return p, nil
}

// Decode parses a TenantPayload and rejects a missing tenant id.
func Decode(t *asynq.Task) (TenantPayload, error) {
	p, err := DecodeAs[TenantPayload](t)
	if err == nil && p.TenantID == "" {
		return p, fmt.Errorf("%w: missing tenant_id", asynq.SkipRetry)
	}
	return p, err
}

// Inline runs jobs synchronously in-process instead of queuing them. It backs the admin CLI (and chained
// jobs, e.g. carbon -> recommendations, run to completion). Set Mux after the handlers are registered.
type Inline struct{ Mux *asynq.ServeMux }

func (i *Inline) Enqueue(ctx context.Context, taskType string, p any, _ ...asynq.Option) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return i.Mux.ProcessTask(ctx, asynq.NewTask(taskType, b))
}
