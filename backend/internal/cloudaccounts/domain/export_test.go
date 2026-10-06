package domain

import (
	"errors"
	"testing"
)

func TestExportConfigValidate(t *testing.T) {
	ok := ExportConfig{Bucket: "acme-billing", Prefix: "exports/focus", Name: "reliabilix-focus", Region: "eu-central-1"}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	if ok.Root() != "exports/focus/reliabilix-focus/" || (ExportConfig{Name: "n"}).Root() != "n/" {
		t.Errorf("root wrong: %q", ok.Root())
	}
	bad := map[string]ExportConfig{
		"upper-case bucket":  {Bucket: "Acme", Name: "n", Region: "eu-central-1"},
		"dotdot bucket":      {Bucket: "a..b", Name: "n", Region: "eu-central-1"},
		"short bucket":       {Bucket: "ab", Name: "n", Region: "eu-central-1"},
		"traversal prefix":   {Bucket: "acme-billing", Prefix: "a/../b", Name: "n", Region: "eu-central-1"},
		"leading slash":      {Bucket: "acme-billing", Prefix: "/a", Name: "n", Region: "eu-central-1"},
		"trailing slash":     {Bucket: "acme-billing", Prefix: "a/", Name: "n", Region: "eu-central-1"},
		"empty segment":      {Bucket: "acme-billing", Prefix: "a//b", Name: "n", Region: "eu-central-1"},
		"weird characters":   {Bucket: "acme-billing", Prefix: "a b", Name: "n", Region: "eu-central-1"},
		"name with slash":    {Bucket: "acme-billing", Name: "a/b", Region: "eu-central-1"},
		"name is dotdot":     {Bucket: "acme-billing", Name: "..", Region: "eu-central-1"},
		"empty name":         {Bucket: "acme-billing", Region: "eu-central-1"},
		"bad region":         {Bucket: "acme-billing", Name: "n", Region: "Frankfurt"},
		"endpoint as region": {Bucket: "acme-billing", Name: "n", Region: "evil.example.com"},
	}
	for name, c := range bad {
		if err := c.Validate(); !errors.Is(err, ErrInvalidConnection) {
			t.Errorf("%s: want ErrInvalidConnection, got %v", name, err)
		}
	}
}
