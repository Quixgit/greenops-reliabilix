package domain

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
)

func TestDestinationMatches(t *testing.T) {
	ev := func(typ string, s notify.Severity) notify.Event { return notify.Event{Type: typ, Severity: s} }
	cases := []struct {
		name string
		d    Destination
		e    notify.Event
		want bool
	}{
		{"exact type", Destination{Enabled: true, MinSeverity: notify.Info, EventTypes: []string{notify.TypeReportReady}}, ev(notify.TypeReportReady, notify.Info), true},
		{"other type", Destination{Enabled: true, MinSeverity: notify.Info, EventTypes: []string{notify.TypeReportReady}}, ev(notify.TypeReportFailed, notify.Warning), false},
		{"wildcard", Destination{Enabled: true, MinSeverity: notify.Info, EventTypes: []string{notify.Wildcard}}, ev(notify.TypeConnectionError, notify.Warning), true},
		{"below min severity", Destination{Enabled: true, MinSeverity: notify.Warning, EventTypes: []string{notify.Wildcard}}, ev(notify.TypeReportReady, notify.Info), false},
		{"at min severity", Destination{Enabled: true, MinSeverity: notify.Warning, EventTypes: []string{notify.Wildcard}}, ev(notify.TypeReportFailed, notify.Warning), true},
		{"disabled", Destination{Enabled: false, MinSeverity: notify.Info, EventTypes: []string{notify.Wildcard}}, ev(notify.TypeReportReady, notify.Critical), false},
	}
	for _, c := range cases {
		if got := c.d.Matches(c.e); got != c.want {
			t.Errorf("%s: Matches = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestDestinationValidate(t *testing.T) {
	ok := Destination{Name: "ops", MinSeverity: notify.Info, EventTypes: []string{notify.TypeReportReady}}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, mut := range map[string]func(*Destination){
		"empty name":       func(d *Destination) { d.Name = "  " },
		"long name":        func(d *Destination) { d.Name = string(make([]byte, 101)) },
		"bad severity":     func(d *Destination) { d.MinSeverity = "loud" },
		"no types":         func(d *Destination) { d.EventTypes = nil },
		"unknown type":     func(d *Destination) { d.EventTypes = []string{"made.up"} },
		"test is internal": func(d *Destination) { d.EventTypes = []string{notify.TypeNotificationTest} },
	} {
		d := ok
		mut(&d)
		if err := d.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: Validate = %v, want ErrInvalid", name, err)
		}
	}
}

func TestPermanentErrors(t *testing.T) {
	base := errors.New("rejected")
	if IsPermanent(base) || !IsPermanent(Permanent(base)) || !IsPermanent(fmt.Errorf("wrapped: %w", Permanent(base))) {
		t.Error("IsPermanent must see through wrapping and only match Permanent errors")
	}
	if Permanent(nil) != nil {
		t.Error("Permanent(nil) must stay nil")
	}
	if !errors.Is(Permanent(base), base) {
		t.Error("Permanent must keep the cause")
	}
}

func TestBackoffGrowsAndCaps(t *testing.T) {
	want := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 16 * time.Minute, 32 * time.Minute, time.Hour}
	for i, w := range want {
		if got := Backoff(i + 1); got != w {
			t.Errorf("Backoff(%d) = %v, want %v", i+1, got, w)
		}
	}
	if Backoff(0) != 30*time.Second || Backoff(100) != time.Hour {
		t.Error("Backoff must clamp its input and cap at one hour")
	}
}

type stubChannel struct{ k Kind }

func (s stubChannel) Kind() Kind                                { return s.k }
func (stubChannel) Prepare(a string) (string, string, error)    { return a, "h", nil }
func (stubChannel) Send(context.Context, Target, Message) error { return nil }

func TestRegistry(t *testing.T) {
	r := NewRegistry(stubChannel{KindWebhook}, stubChannel{KindEmail})
	if c, err := r.Get(KindEmail); err != nil || c.Kind() != KindEmail {
		t.Errorf("Get(email) = %v, %v", c, err)
	}
	if _, err := r.Get(KindSlack); !errors.Is(err, ErrUnknownKind) {
		t.Errorf("Get(slack) = %v, want ErrUnknownKind", err)
	}
	if k := r.Kinds(); len(k) != 2 || k[0] != KindEmail || k[1] != KindWebhook {
		t.Errorf("Kinds = %v, want sorted [email webhook]", k)
	}
	var nilReg *Registry
	if _, err := nilReg.Get(KindEmail); !errors.Is(err, ErrUnknownKind) || len(nilReg.Kinds()) != 0 {
		t.Error("a nil registry has no channels")
	}
}
