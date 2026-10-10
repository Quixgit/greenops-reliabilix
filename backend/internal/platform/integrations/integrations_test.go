package integrations

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCatalogFieldsAreValidated(t *testing.T) {
	_, f, ok := Lookup("smtp", "port")
	if !ok {
		t.Fatal("smtp.port missing")
	}
	for v, wantErr := range map[string]bool{"587": false, "465": false, "0": true, "70000": true, "abc": true, "": true} {
		if err := f.Validate(v); (err != nil) != wantErr {
			t.Errorf("port %q: %v", v, err)
		}
	}
	_, from, _ := Lookup("smtp", "from")
	if from.Validate("Reliabilix <notifications@example.com>") != nil || from.Validate("not an address") == nil || from.Validate("a@b.c\r\nBcc: x@y.z") == nil {
		t.Error("sender address validation")
	}
	_, zo, _ := Lookup("electricitymaps", "zone_overrides")
	if zo.Validate("eu-central-1=DE, us-east-1=US-MIDA-PJM") != nil || zo.Validate("eu-central-1:DE") == nil {
		t.Error("zone override validation")
	}
	_, key, _ := Lookup("electricitymaps", "api_key")
	if key.Validate("short") == nil || key.Validate("has a space in it") == nil || key.Validate("abcdefgh12345678") != nil {
		t.Error("api key validation")
	}
}

func TestSecretsAreMarkedAndEveryFieldHasAnEnvironmentFallback(t *testing.T) {
	secrets := 0
	for _, in := range Catalog() {
		for _, f := range in.Fields {
			if f.Env == "" {
				t.Errorf("%s.%s has no environment variable", in.ID, f.Key)
			}
			if f.Secret {
				secrets++
			}
		}
	}
	if secrets < 2 {
		t.Error("the API key and the SMTP password must be secrets")
	}
}

func TestValueResolutionAndStatusNeverExposeSecrets(t *testing.T) {
	env := map[string]string{"electricitymaps/api_key": "env-secret-key-0000", "smtp/host": "smtp.example.com"}
	s := &Store{Env: func(i, k string) string { return env[i+"/"+k] }, Now: time.Now}
	// No pool: only the environment is consulted.
	v, src := s.Resolve(context.Background(), "electricitymaps", "api_key")
	if v != "env-secret-key-0000" || src != FromEnvironment {
		t.Fatalf("%q %s", v, src)
	}
	if v, src := s.Resolve(context.Background(), "smtp", "from"); v != "" || src != NotSet {
		t.Fatalf("unset must be empty: %q %s", v, src)
	}
	if got := s.ValueOrDefault(context.Background(), "smtp", "port"); got != "587" {
		t.Fatalf("default port: %q", got)
	}
	sts, err := s.Statuses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range sts {
		for _, f := range st.Fields {
			if f.Secret && f.Value != "" {
				t.Errorf("%s.%s: a secret's value must never be reported", st.ID, f.Key)
			}
			if f.Secret && f.Configured && !strings.HasPrefix(f.Hint, "••••") {
				t.Errorf("%s.%s hint %q", st.ID, f.Key, f.Hint)
			}
		}
	}
	for _, st := range sts {
		if st.ID == "smtp" && st.Ready {
			t.Error("smtp lacks a sender address: not ready")
		}
	}
}

func TestHintRevealsAtMostTheLastFourOfALongSecret(t *testing.T) {
	if hint("abc") != "••••" || hint("abcdefghijkl") != "••••ijkl" {
		t.Fatalf("%q %q", hint("abc"), hint("abcdefghijkl"))
	}
}
