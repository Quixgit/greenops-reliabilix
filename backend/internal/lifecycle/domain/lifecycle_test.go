package domain

import (
	"strings"
	"testing"
	"time"
)

func TestConfirmName(t *testing.T) {
	for _, tc := range []struct {
		org, typed string
		want       bool
	}{
		{"Acme Corp", "Acme Corp", true},
		{"Acme Corp", "  Acme Corp ", true},
		{"Acme Corp", "acme corp", false},
		{"Acme Corp", "", false},
		{"", "", false},
	} {
		if got := ConfirmName(tc.org, tc.typed); got != tc.want {
			t.Errorf("ConfirmName(%q,%q) = %v", tc.org, tc.typed, got)
		}
	}
}

func TestRetentionCutoff(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	if _, ok := RetentionCutoff(now, 0); ok {
		t.Fatal("0 months means keep everything")
	}
	c, ok := RetentionCutoff(now, 24)
	if !ok || !c.Equal(time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("24 months: %v %v", c, ok)
	}
	// Anything under 13 months is raised to 13: period comparisons need a full year of history.
	c, _ = RetentionCutoff(now, 3)
	if !c.Equal(time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("3 months must act as 13: %v", c)
	}
}

func TestExportNeverIncludesSecrets(t *testing.T) {
	banned := []string{"key_hash", "token_hash", "target_sealed", "external_id", "auth_sub", "password"}
	seen := map[string]bool{}
	for _, tb := range ExportTables {
		if seen[tb.Name] {
			t.Errorf("duplicate export name %s", tb.Name)
		}
		seen[tb.Name] = true
		for _, b := range banned {
			if strings.Contains(tb.SQL, b) {
				t.Errorf("%s exports %s", tb.Name, b)
			}
		}
		if strings.Contains(tb.SQL, "SELECT t.*") || strings.Contains(tb.SQL, "SELECT *") {
			t.Errorf("%s must list its columns", tb.Name)
		}
	}
}
