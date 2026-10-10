package fxrates

import (
	"archive/zip"
	"bytes"
	"testing"
	"time"
)

const csvBody = "Date,USD,JPY,CYP,GBP,\n2026-10-09,1.1206,177.34,N/A,0.84763,\n2026-10-08,1.1186,177.05,N/A,0.84698,\n2026-09-01,1.1,170,N/A,0.85,\n"

func TestParseReadsRatesSkipsNotAvailableAndHonoursSince(t *testing.T) {
	got, err := Parse([]byte(csvBody), time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 6 { // two days x three currencies; CYP is N/A; September is before `since`
		t.Fatalf("got %d rates: %+v", len(got), got)
	}
	for _, r := range got {
		if r.Currency == "CYP" || r.Day.Month() != 10 || r.PerEUR <= 0 {
			t.Fatalf("unexpected %+v", r)
		}
	}
}

func TestParseReadsTheZippedArchive(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("eurofxref-hist.csv")
	_, _ = w.Write([]byte(csvBody))
	_ = zw.Close()
	got, err := Parse(buf.Bytes(), time.Time{})
	if err != nil || len(got) != 9 {
		t.Fatalf("%d %v", len(got), err)
	}
}

func TestParseRefusesSomethingThatIsNotTheFile(t *testing.T) {
	if _, err := Parse([]byte("<html>404</html>"), time.Time{}); err == nil {
		t.Fatal("an error page must not be read as rates")
	}
}
