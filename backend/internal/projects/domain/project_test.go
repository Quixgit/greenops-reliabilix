package domain

import (
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	if _, err := New("t", "  api  ", "request", time.Now()); err != nil {
		t.Fatal(err)
	}
	for _, c := range [][2]string{{"", "u"}, {strings.Repeat("x", 121), "u"}, {"n", ""}} {
		if _, err := New("t", c[0], c[1], time.Now()); err == nil {
			t.Errorf("%q/%q accepted", c[0], c[1])
		}
	}
}
