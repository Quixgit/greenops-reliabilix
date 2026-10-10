package ukci

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRangeKeepsOnlyMeasuredPlausibleHalfHoursAndChunks(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[
		 {"from":"2026-10-01T00:00Z","to":"2026-10-01T00:30Z","intensity":{"forecast":150,"actual":179,"index":"high"}},
		 {"from":"2026-10-01T00:30Z","to":"2026-10-01T01:00Z","intensity":{"forecast":150,"actual":null,"index":"moderate"}},
		 {"from":"2026-10-01T01:00Z","to":"2026-10-01T01:30Z","intensity":{"forecast":150,"actual":0,"index":"low"}}]}`))
	}))
	defer srv.Close()
	c := New()
	c.BaseURL = srv.URL
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	got, err := c.Range(context.Background(), from, from.AddDate(0, 0, 20))
	if err != nil {
		t.Fatal(err)
	}
	// 20 days = 13 + 7: two requests; each answer yields one measured, plausible half hour (the same stamp: it is repeated)
	if len(paths) != 2 || len(got) != 2 || got[0].G != 179 {
		t.Errorf("paths=%v got=%v", paths, got)
	}
	if !strings.HasPrefix(paths[0], "/intensity/2026-10-01T00:00Z/") {
		t.Errorf("path = %s", paths[0])
	}
	c.Allow = func(context.Context) error { return http.ErrAbortHandler }
	if _, err := c.Range(context.Background(), from, from.Add(time.Hour)); err == nil {
		t.Error("a request over the cap must not be made")
	}
}
