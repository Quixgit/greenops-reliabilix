package electricitymaps

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetIntensity(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("auth-token") != "k" || r.URL.Query().Get("zone") != "DE" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"carbonIntensity":312.5,"datetime":"2026-10-05T12:00:00Z"}`))
	}))
	defer s.Close()
	c := &Client{BaseURL: s.URL, APIKey: "k", HTTP: s.Client()}
	got, err := c.GetIntensity(context.Background(), "eu-central-1")
	if err != nil || got.GPerKWh != 312.5 {
		t.Fatalf("got %+v, %v", got, err)
	}
	if _, err := c.GetIntensity(context.Background(), "mars-1"); err == nil {
		t.Error("unknown region accepted")
	}
}
