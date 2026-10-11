package queue

import (
	"fmt"
	"sort"
	"time"

	"github.com/hibiken/asynq"
)

// Inspector reads queue telemetry for the operator console. It never returns a payload (payloads carry tenant ids)
// and it changes nothing.
type Inspector struct{ i *asynq.Inspector }

func NewInspector(redisAddr string) *Inspector {
	return &Inspector{i: asynq.NewInspector(asynq.RedisClientOpt{Addr: redisAddr})}
}

func (in *Inspector) Close() error { return in.i.Close() }

type QueueStat struct {
	Name      string `json:"name"`
	Size      int    `json:"size"`
	Pending   int    `json:"pending"`
	Active    int    `json:"active"`
	Scheduled int    `json:"scheduled"`
	Retry     int    `json:"retry"`
	Archived  int    `json:"archived"`
	Completed int    `json:"completed"`
	// Processed and Failed are today's counters (Asynq resets them daily).
	Processed int `json:"processed_today"`
	Failed    int `json:"failed_today"`
	// LatencySeconds is how long the oldest pending task has waited.
	LatencySeconds float64 `json:"latency_seconds"`
	Paused         bool    `json:"paused"`
}

type FailedTask struct {
	ID           string     `json:"id"`
	Queue        string     `json:"queue"`
	Type         string     `json:"type"`
	State        string     `json:"state"` // retry (will be tried again) or archived (gave up)
	LastError    string     `json:"last_error"`
	Retried      int        `json:"retried"`
	MaxRetry     int        `json:"max_retry"`
	LastFailedAt *time.Time `json:"last_failed_at"`
	NextAt       *time.Time `json:"next_attempt_at"`
}

type Schedule struct {
	Spec string     `json:"spec"`
	Type string     `json:"type"`
	Next time.Time  `json:"next"`
	Prev *time.Time `json:"prev"`
}

func (in *Inspector) Queues() ([]QueueStat, error) {
	names, err := in.i.Queues()
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	out := make([]QueueStat, 0, len(names))
	for _, n := range names {
		q, err := in.i.GetQueueInfo(n)
		if err != nil {
			return nil, fmt.Errorf("queue %s: %w", n, err)
		}
		out = append(out, QueueStat{Name: n, Size: q.Size, Pending: q.Pending, Active: q.Active, Scheduled: q.Scheduled, Retry: q.Retry,
			Archived: q.Archived, Completed: q.Completed, Processed: q.Processed, Failed: q.Failed, LatencySeconds: q.Latency.Seconds(), Paused: q.Paused})
	}
	return out, nil
}

// Failed lists tasks waiting for a retry and tasks that gave up, newest failure first, at most limit.
func (in *Inspector) Failed(limit int) ([]FailedTask, error) {
	names, err := in.i.Queues()
	if err != nil {
		return nil, err
	}
	out := []FailedTask{} // never nil: an empty list is `[]` on the wire
	for _, q := range names {
		retry, err := in.i.ListRetryTasks(q, asynq.PageSize(limit))
		if err != nil {
			return nil, err
		}
		arch, err := in.i.ListArchivedTasks(q, asynq.PageSize(limit))
		if err != nil {
			return nil, err
		}
		for _, t := range append(retry, arch...) {
			f := FailedTask{ID: t.ID, Queue: t.Queue, Type: t.Type, State: t.State.String(), LastError: t.LastErr, Retried: t.Retried, MaxRetry: t.MaxRetry}
			if !t.LastFailedAt.IsZero() {
				v := t.LastFailedAt
				f.LastFailedAt = &v
			}
			if !t.NextProcessAt.IsZero() {
				v := t.NextProcessAt
				f.NextAt = &v
			}
			out = append(out, f)
		}
	}
	sort.Slice(out, func(a, b int) bool {
		x, y := out[a].LastFailedAt, out[b].LastFailedAt
		switch {
		case x == nil:
			return false
		case y == nil:
			return true
		}
		return x.After(*y)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// Schedules lists the periodic jobs the scheduler process registered (read from its heartbeat in Redis).
func (in *Inspector) Schedules() ([]Schedule, error) {
	es, err := in.i.SchedulerEntries()
	if err != nil {
		return nil, err
	}
	out := make([]Schedule, 0, len(es))
	for _, e := range es {
		s := Schedule{Spec: e.Spec, Type: e.Task.Type(), Next: e.Next}
		if !e.Prev.IsZero() {
			v := e.Prev
			s.Prev = &v
		}
		out = append(out, s)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Next.Before(out[b].Next) })
	return out, nil
}

// Workers is how many worker processes are connected to the queues (read from their heartbeat in Redis).
func (in *Inspector) Workers() (int, error) {
	s, err := in.i.Servers()
	return len(s), err
}

// Retry puts a task that is waiting for a retry, or that gave up, back in the queue to run now.
func (in *Inspector) Retry(queueName, id string) error { return in.i.RunTask(queueName, id) }

// Delete removes a task that is waiting for a retry, or that gave up, so it will not run again.
func (in *Inspector) Delete(queueName, id string) error { return in.i.DeleteTask(queueName, id) }
