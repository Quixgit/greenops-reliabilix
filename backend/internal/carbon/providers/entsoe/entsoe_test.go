package entsoe

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const doc = `<?xml version="1.0" encoding="UTF-8"?>
<GL_MarketDocument xmlns="urn:iec62325.351:tc57wg16:451-6:generationloaddocument:3:0">
 <TimeSeries><inBiddingZone_Domain.mRID codingScheme="A01">10Y1001A1001A82H</inBiddingZone_Domain.mRID>
  <MktPSRType><psrType>B14</psrType></MktPSRType>
  <Period><timeInterval><start>2026-10-10T00:00Z</start><end>2026-10-10T02:00Z</end></timeInterval><resolution>PT60M</resolution>
   <Point><position>1</position><quantity>100</quantity></Point><Point><position>2</position><quantity>100</quantity></Point></Period></TimeSeries>
 <TimeSeries><inBiddingZone_Domain.mRID codingScheme="A01">10Y1001A1001A82H</inBiddingZone_Domain.mRID>
  <MktPSRType><psrType>B05</psrType></MktPSRType>
  <Period><timeInterval><start>2026-10-10T00:00Z</start><end>2026-10-10T02:00Z</end></timeInterval><resolution>PT30M</resolution>
   <Point><position>1</position><quantity>100</quantity></Point><Point><position>2</position><quantity>100</quantity></Point>
   <Point><position>4</position><quantity>300</quantity></Point></Period></TimeSeries>
 <TimeSeries><outBiddingZone_Domain.mRID codingScheme="A01">10Y1001A1001A82H</outBiddingZone_Domain.mRID>
  <MktPSRType><psrType>B10</psrType></MktPSRType>
  <Period><timeInterval><start>2026-10-10T00:00Z</start><end>2026-10-10T01:00Z</end></timeInterval><resolution>PT60M</resolution>
   <Point><position>1</position><quantity>5000</quantity></Point></Period></TimeSeries>
</GL_MarketDocument>`

func TestHourlyWeightsByTimeAndSkipsStorage(t *testing.T) {
	var gotToken, gotDomain string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken, gotDomain = r.URL.Query().Get("securityToken"), r.URL.Query().Get("in_Domain")
		_, _ = w.Write([]byte(doc))
	}))
	defer srv.Close()
	c := New("tok")
	c.BaseURL = srv.URL
	from := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	rs, err := c.Hourly(context.Background(), Domains["DE"], from, from.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if gotToken != "tok" || gotDomain != "10Y1001A1001A82H" {
		t.Fatalf("query: %q %q", gotToken, gotDomain)
	}
	if len(rs) != 2 {
		t.Fatalf("hours: %+v", rs)
	}
	// hour 0: nuclear 100 MW x 1 h (12) and coal 100 MW for both half hours (820): (100*12 + 100*820) / 200 = 416
	if math.Abs(rs[0].G-416) > 0.01 {
		t.Fatalf("hour 0: %v", rs[0].G)
	}
	// hour 1: coal position 3 is missing and repeats position 2 (100), position 4 is 300: weights 100+300 half hours
	want := (100*12.0 + (100*0.5*820 + 300*0.5*820)) / (100 + 100*0.5 + 300*0.5)
	if math.Abs(rs[1].G-want) > 0.01 {
		t.Fatalf("hour 1: %v want %v", rs[1].G, want)
	}
}

func TestNoDataIsEmptyNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<Acknowledgement_MarketDocument><Reason><code>999</code><text>No matching data found</text></Reason></Acknowledgement_MarketDocument>`))
	}))
	defer srv.Close()
	c := New("tok")
	c.BaseURL = srv.URL
	rs, err := c.Hourly(context.Background(), Domains["FR"], time.Now().Add(-2*time.Hour), time.Now())
	if err != nil || len(rs) != 0 {
		t.Fatalf("%v %v", rs, err)
	}
}

func TestTokenIsNeverInAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer srv.Close()
	c := New("secret-token-123")
	c.BaseURL = srv.URL
	err := c.Check(context.Background())
	if err == nil || strings.Contains(err.Error(), "secret-token-123") {
		t.Fatalf("error: %v", err)
	}
	c.BaseURL = "http://127.0.0.1:1"
	if err := c.Check(context.Background()); err == nil || strings.Contains(err.Error(), "secret-token-123") {
		t.Fatalf("transport error: %v", err)
	}
	if _, err := New("").Hourly(context.Background(), "x", time.Now(), time.Now()); err == nil {
		t.Fatal("no token must fail")
	}
}
