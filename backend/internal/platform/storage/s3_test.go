package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeS3 is a minimal path-style S3: PUT and GET of whole objects.
func fakeS3(t *testing.T) (*httptest.Server, *sync.Map) {
	t.Helper()
	var objects sync.Map
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			objects.Store(r.URL.Path, b)
			w.Header().Set("ETag", `"x"`)
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			v, ok := objects.Load(r.URL.Path)
			if !ok {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`<Error><Code>NoSuchKey</Code><Message>no</Message></Error>`))
				return
			}
			_, _ = w.Write(v.([]byte)) //nolint:gosec // test fake: echoes bytes the test itself stored
		case http.MethodDelete:
			objects.Delete(r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &objects
}

func TestS3StoreRoundTrip(t *testing.T) {
	srv, objects := fakeS3(t)
	s, err := NewS3(context.Background(), S3Config{Endpoint: srv.URL, Bucket: "greenops", Region: "us-east-1", PathStyle: true, AccessKey: "k", SecretKey: "s"})
	if err != nil {
		t.Fatal(err)
	}
	key, err := Key("tenant-1", "reports", "r1.csv")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Put(context.Background(), key, strings.NewReader("day,kg\n2026-09-01,1\n")); err != nil {
		t.Fatalf("put: %v", err)
	}
	if _, ok := objects.Load("/greenops/" + key); !ok {
		t.Fatal("object not stored under bucket/tenants/{tenant}/reports/")
	}
	body, err := s.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer func() { _ = body.Close() }()
	got, _ := io.ReadAll(body)
	if string(got) != "day,kg\n2026-09-01,1\n" {
		t.Errorf("round trip = %q", got)
	}
	if err := s.Delete(context.Background(), key); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := objects.Load("/greenops/" + key); ok {
		t.Fatal("object still stored after delete")
	}
	if _, err := s.Get(context.Background(), "tenants/tenant-1/reports/missing"); err == nil {
		t.Error("missing object returned no error")
	}
}

func TestKeyRejectsTraversal(t *testing.T) {
	for _, bad := range [][3]string{{"", "reports", "a"}, {"t", "reports", "../x"}, {"t/../u", "reports", "a"}, {"t", "reports", "/abs"}, {"t", `re\ports`, "a"}} {
		if _, err := Key(bad[0], bad[1], bad[2]); err == nil {
			t.Errorf("Key%v accepted", bad)
		}
	}
	if k, err := Key("t1", "raw", "aws/c1/x.json"); err != nil || k != "tenants/t1/raw/aws/c1/x.json" {
		t.Errorf("key = %q %v", k, err)
	}
}

func TestFSStoreStaysInsideRoot(t *testing.T) {
	s := FSStore{Root: t.TempDir()}
	if err := s.Put(context.Background(), "tenants/t/reports/a.csv", strings.NewReader("x")); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(context.Background(), "../escape", strings.NewReader("x")); err == nil {
		t.Error("write outside the root accepted")
	}
	if _, err := s.Get(context.Background(), "../../etc/passwd"); err == nil {
		t.Error("read outside the root accepted")
	}
}
