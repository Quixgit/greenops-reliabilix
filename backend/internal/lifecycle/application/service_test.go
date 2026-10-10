package application

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/lifecycle/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/storage"
)

type fakeRepo struct {
	name      string
	scheduled *time.Time
	due       []string
	erased    []string
	eraseErr  map[string]error
	cutoff    time.Time
}

func (f *fakeRepo) OrganizationName(context.Context, string) (string, error) { return f.name, nil }
func (f *fakeRepo) Deletion(context.Context, string) (domain.Deletion, error) {
	return domain.Deletion{Scheduled: f.scheduled != nil}, nil
}
func (f *fakeRepo) Schedule(_ context.Context, _, _ string, at time.Time) error {
	if f.scheduled != nil {
		return domain.ErrAlreadyScheduled
	}
	f.scheduled = &at
	return nil
}
func (f *fakeRepo) Cancel(context.Context, string) error { f.scheduled = nil; return nil }
func (f *fakeRepo) Export(_ context.Context, _ string, emit func(string, json.RawMessage) error) error {
	return emit("projects", json.RawMessage(`{"name":"p"}`))
}
func (f *fakeRepo) DueDeletions(context.Context) ([]string, error) { return f.due, nil }
func (f *fakeRepo) Erase(_ context.Context, id string) (map[string]int64, error) {
	if err := f.eraseErr[id]; err != nil {
		return nil, err
	}
	f.erased = append(f.erased, id)
	return map[string]int64{"projects.projects": 2}, nil
}
func (f *fakeRepo) DropOldPartitions(_ context.Context, c time.Time) (int, error) {
	f.cutoff = c
	return 3, nil
}

func svc(r *fakeRepo, st storage.Store) Service {
	return Service{Repo: r, Store: st, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now: func() time.Time { return time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC) }}
}

func TestScheduleNeedsTheExactNameAndUsesTheGracePeriod(t *testing.T) {
	r := &fakeRepo{name: "Acme Corp"}
	s := svc(r, nil)
	if _, err := s.ScheduleDeletion(context.Background(), "t", "me", "acme"); !errors.Is(err, domain.ErrNameMismatch) || r.scheduled != nil {
		t.Fatalf("a wrong name must not schedule anything: %v", err)
	}
	if _, err := s.ScheduleDeletion(context.Background(), "t", "me", "Acme Corp"); err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC); !r.scheduled.Equal(want) {
		t.Fatalf("execute_after %v, want the 14-day default %v", r.scheduled, want)
	}
	if _, err := s.ScheduleDeletion(context.Background(), "t", "me", "Acme Corp"); !errors.Is(err, domain.ErrAlreadyScheduled) {
		t.Fatalf("second request: %v", err)
	}
}

func TestEraseDueRemovesFilesBeforeRowsAndContinuesPastFailures(t *testing.T) {
	st := storage.NewMemory()
	good, bad := "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"
	for _, id := range []string{good, bad} {
		k, _ := storage.Key(id, "reports", "r.pdf")
		_ = st.Put(context.Background(), k, strings.NewReader("x"))
	}
	r := &fakeRepo{due: []string{bad, good}, eraseErr: map[string]error{bad: errors.New("boom")}}
	n, err := svc(r, st).EraseDue(context.Background())
	if n != 1 || err == nil {
		t.Fatalf("erased %d, err %v: one failed, one succeeded", n, err)
	}
	if len(r.erased) != 1 || r.erased[0] != good {
		t.Fatalf("erased %v", r.erased)
	}
	// Both organizations' files were removed before the database step (idempotent, so a retry is safe).
	if len(st.M) != 0 {
		t.Fatalf("files left: %v", st.M)
	}
}

func TestDeletePrefixRefusesBroadPrefixes(t *testing.T) {
	st := storage.NewMemory()
	for _, p := range []string{"", "/", "tenants/", "tenants", "tenants/../x/"} {
		if err := st.DeletePrefix(context.Background(), p); err == nil {
			t.Errorf("prefix %q must be refused", p)
		}
	}
}

func TestEnforceRetention(t *testing.T) {
	r := &fakeRepo{}
	n, err := svc(r, nil).EnforceRetention(context.Background())
	if err != nil || n != 0 || !r.cutoff.IsZero() {
		t.Fatalf("retention is off by default and must touch nothing: n=%d err=%v cutoff=%v", n, err, r.cutoff)
	}
}

func TestExportStartsWithADescriptionAndEndsLinesWithNewlines(t *testing.T) {
	var lines []string
	err := svc(&fakeRepo{}, nil).Export(context.Background(), "t", func(b []byte) error {
		lines = append(lines, string(b))
		return nil
	})
	if err != nil || len(lines) != 2 || !strings.Contains(lines[0], `"export"`) || !strings.HasSuffix(lines[1], "\n") {
		t.Fatalf("%v %q", err, lines)
	}
}
