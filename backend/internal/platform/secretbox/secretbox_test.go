package secretbox

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func box(t *testing.T, fill byte) *Box {
	t.Helper()
	b, err := New(bytes.Repeat([]byte{fill}, 32))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSealOpenRoundTrip(t *testing.T) {
	b := box(t, 1)
	sealed, err := b.Seal("https://hooks.slack.com/services/T/B/secret", "tenant:dest")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sealed, "slack") || !strings.HasPrefix(sealed, "v1:") {
		t.Fatalf("sealed value leaks or lacks version: %q", sealed)
	}
	got, err := b.Open(sealed, "tenant:dest")
	if err != nil || got != "https://hooks.slack.com/services/T/B/secret" {
		t.Fatalf("Open = %q, %v", got, err)
	}
}

func TestSealIsRandomised(t *testing.T) {
	b := box(t, 1)
	a, _ := b.Seal("x", "aad")
	c, _ := b.Seal("x", "aad")
	if a == c {
		t.Fatal("same plaintext sealed twice must differ (random nonce)")
	}
}

func TestOpenRejectsWrongContextKeyAndTampering(t *testing.T) {
	b := box(t, 1)
	sealed, _ := b.Seal("secret", "tenant-a:dest-1")
	if _, err := b.Open(sealed, "tenant-b:dest-1"); !errors.Is(err, ErrInvalid) {
		t.Errorf("a ciphertext moved to another row must not open, got %v", err)
	}
	if _, err := box(t, 2).Open(sealed, "tenant-a:dest-1"); !errors.Is(err, ErrInvalid) {
		t.Errorf("wrong key must not open, got %v", err)
	}
	tampered := sealed[:len(sealed)-2] + "AA"
	if _, err := b.Open(tampered, "tenant-a:dest-1"); !errors.Is(err, ErrInvalid) {
		t.Errorf("tampered value must not open, got %v", err)
	}
	for _, bad := range []string{"", "plain", "v1:", "v1:%%%", "v2:AAAA"} {
		if _, err := b.Open(bad, "x"); !errors.Is(err, ErrInvalid) {
			t.Errorf("Open(%q) = %v, want ErrInvalid", bad, err)
		}
	}
}

func TestNewAndParseKey(t *testing.T) {
	if _, err := New(make([]byte, 16)); err == nil {
		t.Error("a 16-byte key must be rejected")
	}
	if _, err := ParseKey("short"); err == nil {
		t.Error("a short key must be rejected")
	}
	k, err := ParseKey("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=") // 32 zero bytes
	if err != nil || len(k) != 32 {
		t.Errorf("ParseKey = %v, %v", k, err)
	}
	var nilBox *Box
	if nilBox.Configured() || (&Box{}).Configured() {
		t.Error("nil and zero boxes are not configured")
	}
	if _, err := nilBox.Seal("x", "y"); err == nil {
		t.Error("an unconfigured box must refuse to seal")
	}
}

func TestDeriveIsStableAndSeparated(t *testing.T) {
	b := box(t, 1)
	first := b.Derive("webhook", "a")
	if first != b.Derive("webhook", "a") {
		t.Error("Derive must be deterministic")
	}
	if b.Derive("webhook", "a") == b.Derive("webhook", "b") || b.Derive("webhook", "a") == b.Derive("other", "a") {
		t.Error("Derive must differ per purpose and id")
	}
	if b.Derive("webhook", "a") == box(t, 2).Derive("webhook", "a") {
		t.Error("Derive must depend on the key")
	}
	if len(b.Derive("webhook", "a")) != 64 {
		t.Error("expected 32 bytes of hex")
	}
}
