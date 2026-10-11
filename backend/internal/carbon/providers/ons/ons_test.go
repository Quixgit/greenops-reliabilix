package ons

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const file = "id_subsistema;nom_subsistema;din_instante;val_gerhidraulica;val_gertermica;val_gereolica;val_gersolar;val_carga;val_intercambio\n" +
	"SE ;SUDESTE/CENTRO-OESTE;2026-01-01 00:00:00;27000;5500;200;0;41000;-8000\n" +
	"SIN;SISTEMA INTERLIGADO NACIONAL;2026-01-01 00:00:00;53000;8000;11000;1;72000;0\n" +
	"S  ;SUL;2026-01-01 00:00:00;12000;150;400;1;11000;1400\n" +
	"SE ;SUDESTE/CENTRO-OESTE;2026-01-01 01:00:00;0;0;0;0;41000;0\n"

func TestHourlyComputesFromTheMixAndConvertsTheClock(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if !strings.HasSuffix(r.URL.Path, "BALANCO_ENERGIA_SUBSISTEMA_2026.csv") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(file))
	}))
	defer srv.Close()
	c := New()
	c.BaseURL = srv.URL
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rs, err := c.Hourly(context.Background(), Subsystems["BR-CS"], from, from.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 {
		t.Fatalf("rows: %+v", rs) // the all-zero hour is skipped, the SIN row and the South row are not this subsystem
	}
	// 00:00 Brasília time is 03:00 UTC
	if !rs[0].At.Equal(time.Date(2026, 1, 1, 3, 0, 0, 0, time.UTC)) {
		t.Fatalf("clock: %v", rs[0].At)
	}
	want := (27000*HydroFactor + 5500*ThermalFactor + 200*WindFactor) / (27000 + 5500 + 200)
	if math.Abs(rs[0].G-want) > 0.01 {
		t.Fatalf("g %v want %v", rs[0].G, want)
	}
	// a second region in the same refresh reads the cached file
	before := hits
	if _, err := c.Hourly(context.Background(), "S", from, from.Add(24*time.Hour)); err != nil || hits != before {
		t.Fatalf("hits %d (was %d) err %v", hits, before, err)
	}
}

func TestUnpublishedYearIsNoData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	defer srv.Close()
	c := New()
	c.BaseURL = srv.URL + "/none"
	rs, err := c.Hourly(context.Background(), "N", time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2031, 1, 2, 0, 0, 0, 0, time.UTC))
	if err != nil || len(rs) != 0 {
		t.Fatalf("%v %v", rs, err)
	}
}

func TestBrokenFileIsAnError(t *testing.T) {
	if _, err := parse(strings.NewReader("a;b\n1;2\n")); err == nil {
		t.Fatal("a file without the expected columns must fail")
	}
}
