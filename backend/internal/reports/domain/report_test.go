package domain

import "testing"

func TestNormalizeClouds(t *testing.T) {
	got, ok := NormalizeClouds([]string{" AWS", "azure", "aws", ""})
	if !ok || len(got) != 2 || got[0] != "aws" || got[1] != "azure" {
		t.Errorf("NormalizeClouds = %v %v, want [aws azure]", got, ok)
	}
	if _, ok := NormalizeClouds([]string{"aws; drop table"}); ok {
		t.Error("a malformed cloud name must be refused, not narrowed silently")
	}
	nine := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}
	if _, ok := NormalizeClouds(nine); ok {
		t.Error("more than eight clouds must be refused")
	}
}
