package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/reports/domain"
)

var tbl = domain.Table{
	Title: "Carbon report", Period: "2026-09-01 to 2026-09-30", Notes: []string{"Methodology CCF-2026.1 (provisional)"},
	Headers: []string{"day", "service", "carbon_kg_co2e"},
	Rows:    [][]string{{"2026-09-01", "Amazon EC2", "12.5"}, {"2026-09-02", "=HYPERLINK(\"http://evil\")", "-3.2"}, {"2026-09-03", "-cmd|' /C calc'!A0", "1"}},
}

var at = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

func TestCSVNeutralizesFormulaInjectionButKeepsNegativeNumbers(t *testing.T) {
	b, err := CSV(tbl)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "# Methodology CCF-2026.1 (provisional)") || !strings.Contains(s, "day,service,carbon_kg_co2e") {
		t.Errorf("header/notes missing:\n%s", s)
	}
	if !strings.Contains(s, `"'=HYPERLINK(""http://evil"")"`) {
		t.Errorf("formula not neutralized:\n%s", s)
	}
	if !strings.Contains(s, ",-3.2") {
		t.Errorf("negative number must stay numeric:\n%s", s)
	}
	if !strings.Contains(s, "'-cmd") {
		t.Errorf("minus-prefixed text not neutralized:\n%s", s)
	}
}

func TestJSON(t *testing.T) {
	b, err := JSON(tbl, at)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Title string
		Rows  []map[string]string
	}
	if err := json.Unmarshal(b, &out); err != nil || out.Title != "Carbon report" || len(out.Rows) != 3 || out.Rows[0]["service"] != "Amazon EC2" {
		t.Fatalf("json: %v %s", err, b)
	}
}

func TestPDF(t *testing.T) {
	b, err := PDF(tbl, at)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(b, []byte("%PDF-")) || len(b) < 500 {
		t.Errorf("not a PDF (%d bytes)", len(b))
	}
	big := domain.Table{Title: "big", Headers: []string{"a"}}
	for i := 0; i < MaxPDFRows+50; i++ {
		big.Rows = append(big.Rows, []string{"x"})
	}
	if _, err := PDF(big, at); err != nil {
		t.Errorf("large report: %v", err)
	}
	if _, err := PDF(domain.Table{Title: "no cols"}, at); err == nil {
		t.Error("table without columns accepted")
	}
}

func TestRenderDispatch(t *testing.T) {
	for _, f := range []domain.Format{domain.CSV, domain.JSON, domain.PDF} {
		if b, err := Render(tbl, f, at); err != nil || len(b) == 0 {
			t.Errorf("%s: %v", f, err)
		}
	}
	if _, err := Render(tbl, "xlsx", at); err == nil {
		t.Error("unknown format accepted")
	}
}
