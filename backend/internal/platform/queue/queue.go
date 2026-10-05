// Package queue wraps Asynq (Redis) for background jobs. Task payloads always
// carry tenant_id explicitly: a worker has no HTTP request to take it from.
package queue

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
)

// Task types. Convention: <domain>:<action>.
const (
	TaskSyncAWSAccount     = "cloudaccounts:sync_aws_account"
	TaskSyncAll            = "cloudaccounts:sync_all"
	TaskCalculateCarbon    = "carbon:calculate"
	TaskEnsurePartitions   = "platform:ensure_partitions"
	TaskRefreshGrid        = "carbon:refresh_grid_intensity"
	TaskSyncAzure          = "cloudaccounts:sync_azure_subscription" // phase 2
	TaskSyncGCP            = "cloudaccounts:sync_gcp_billing"        // phase 2
	TaskSyncKubernetes     = "kubernetes:sync"                       // phase 2
	TaskGenerateReport     = "reports:generate"                      // phase 2
	TaskCalcRecommendation = "recommendations:calculate"             // phase 2
)

// TenantPayload is the minimal payload of tenant-scoped jobs.
type TenantPayload struct {
	TenantID  string `json:"tenant_id"`
	ProjectID string `json:"project_id,omitempty"`
	RefID     string `json:"ref_id,omitempty"`
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
	return err
}

// NewServer builds the worker server.
func NewServer(redisAddr string, concurrency int) *asynq.Server {
	return asynq.NewServer(asynq.RedisClientOpt{Addr: redisAddr}, asynq.Config{
		Concurrency: concurrency,
		Queues:      map[string]int{"default": 6, "critical": 3, "low": 1},
	})
}

// JobRegistrar is implemented by domain modules that process background jobs.
type JobRegistrar interface {
	RegisterJobs(mux *asynq.ServeMux)
}

// Decode parses a task payload and rejects missing tenant ids.
func Decode(t *asynq.Task) (TenantPayload, error) {
	var p TenantPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return p, err
	}
	if p.TenantID == "" {
		return p, asynq.SkipRetry // malformed job: never retry
	}
	return p, nil
}
