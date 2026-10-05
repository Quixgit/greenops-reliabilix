package domain

import "testing"

func TestSCI(t *testing.T) {
	got, err := SCI(10, 400, 1000, 100)
	if err != nil || got != 50 {
		t.Fatalf("SCI = %v, %v; want 50", got, err)
	}
	if _, err := SCI(1, 1, 1, 0); err == nil {
		t.Error("zero units accepted")
	}
}
