package errlog

import (
	"strings"
	"testing"
)

func TestSanitizeRemovesSecrets(t *testing.T) {
	cases := map[string][]string{ // input -> fragments that must not survive
		`dial postgres://greenops_api:S3cr3t@db.internal:5432/greenops: refused`: {"S3cr3t", "greenops_api"},
		`Get "https://hooks.example.com/in/abc?token=TOPSECRET123": timeout`:     {"TOPSECRET123", "token="},
		`request failed: Authorization: Bearer eyJhbGciOiJSUzI1NiJ9.payload.sig`: {"eyJhbGci", "payload.sig"},
		`denied for AKIAIOSFODNN7EXAMPLE in account`:                             {"AKIAIOSFODNN7EXAMPLE"},
		`mail to alice.smith@example.com bounced`:                                {"alice.smith", "example.com"},
		`unexpected key v1:AbCdEfGhIjKlMnOpQrStUvWxYz0123456789abcdef`:           {"AbCdEfGhIjKlMnOpQrStUvWxYz0123456789abcdef"},
		`password=hunter2 and secret: s3kr1t`:                                    {"hunter2", "s3kr1t"},
	}
	for in, gone := range cases {
		out := Sanitize(in)
		for _, g := range gone {
			if strings.Contains(out, g) {
				t.Errorf("Sanitize(%q) = %q still contains %q", in, out, g)
			}
		}
	}
	if got := Sanitize("line1\r\nline2\x00\ttab"); strings.ContainsAny(got, "\r\n\x00\t") {
		t.Errorf("control characters survived: %q", got)
	}
	if got := Sanitize(strings.Repeat("é", 400)); len(got) > 500 || !strings.HasPrefix(got, "é") {
		t.Errorf("length not bounded or not valid UTF-8: %d", len(got))
	}
	if got := Sanitize("connection refused"); got != "connection refused" {
		t.Errorf("plain text must stay readable, got %q", got)
	}
}

func TestFingerprintGroupsEqualProblems(t *testing.T) {
	a := Fingerprint("worker", "cloudaccounts:sync", "account 123456789012 denied after 3 attempts in 4s")
	b := Fingerprint("worker", "cloudaccounts:sync", "account 999999999999 denied after 7 attempts in 12s")
	if a != b || len(a) != 64 {
		t.Errorf("numbers must not split groups: %s vs %s", a, b)
	}
	if a == Fingerprint("worker", "cloudaccounts:sync", "account 1 timed out") {
		t.Error("a different message is a different problem")
	}
	if a == Fingerprint("api", "cloudaccounts:sync", "account 123456789012 denied after 3 attempts in 4s") {
		t.Error("the same message in another service is a different problem")
	}
	u1 := Fingerprint("api", "GET /x", "report 3f2a9c1e-1111-4222-8333-444455556666 missing")
	u2 := Fingerprint("api", "GET /x", "report 00000000-1111-4222-8333-444455556666 missing")
	if u1 != u2 {
		t.Error("UUIDs must not split groups")
	}
}

func TestRecordAggregatesInMemoryAndIsBounded(t *testing.T) {
	p := &PG{service: "worker", buf: map[string]*pending{}, maxKeys: 3}
	for range 5 {
		p.Record(Event{Component: "job", Message: "boom 1", Severity: Warning})
	}
	p.Record(Event{Component: "job", Message: "boom 2", Severity: Critical, TenantID: "t1", RequestID: "r1"}) // same group, worse severity
	if len(p.buf) != 1 {
		t.Fatalf("groups = %d, want 1 (numbers masked)", len(p.buf))
	}
	for _, x := range p.buf {
		if x.count != 6 || x.e.Severity != Critical || x.e.TenantID != "t1" || x.e.RequestID != "r1" || x.e.Service != "worker" {
			t.Errorf("group = %+v", x)
		}
	}
	for i := range 10 {
		p.Record(Event{Component: "job", Message: strings.Repeat("x", i+1) + " different"})
	}
	if len(p.buf) != 3 {
		t.Errorf("buffer must stay bounded at maxKeys, got %d", len(p.buf))
	}
	p.Record(Event{Message: "no component or severity"})
	for _, x := range p.buf {
		if x.e.Component == "" || x.e.Severity == "" {
			t.Errorf("defaults not applied: %+v", x.e)
		}
	}
}

func TestRecordSanitizesBeforeBuffering(t *testing.T) {
	p := &PG{service: "api", buf: map[string]*pending{}, maxKeys: 10}
	p.Record(Event{Component: "POST /x", Message: "failed: https://u:pw@host/path?token=abc"})
	for _, x := range p.buf {
		if strings.Contains(x.e.Message, "pw") || strings.Contains(x.e.Message, "token=abc") {
			t.Errorf("secret buffered: %q", x.e.Message)
		}
	}
}
