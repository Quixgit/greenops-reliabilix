package render

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/reports/domain"
)

func sampleFinops(n int) domain.Table {
	t := domain.Table{Title: "FinOps cost report", Period: "2026-09-12 to 2026-10-10", Organization: "Dev Organization", Scope: "Project production",
		Notes:   []string{"Costs follow FOCUS: billed_cost = BilledCost, effective_cost = EffectiveCost, in the billing currency (no FX conversion)."},
		Headers: []string{"day", "provider", "service", "category", "region", "billed_cost", "effective_cost", "currency"}}
	svc := []string{"Azure Cognitive Search", "Azure Database for PostgreSQL", "Azure App Service", "Virtual Network", "Redis Cache"}
	reg := []string{"canadacentral", "canadaeast", "eastus2"}
	for i := 0; i < n; i++ {
		d := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i/6)
		t.Rows = append(t.Rows, []string{d.Format("2006-01-02"), "azure", svc[i%5], "compute", reg[i%3], fmt.Sprintf("%.6f", 3+float64(i%17)*1.37), fmt.Sprintf("%.6f", 3+float64(i%17)*1.37), "CAD"})
	}
	return t
}

func TestPDFIsAFormattedDocument(t *testing.T) {
	b, err := PDF(sampleFinops(240), at)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(b, []byte("%PDF-")) || len(b) < 4000 {
		t.Fatalf("not a PDF document (%d bytes)", len(b))
	}
	if n := bytes.Count(b, []byte("/Type /Page\n")); n < 2 {
		t.Errorf("240 rows should run over several pages, found %d", n)
	}
	carbon := domain.Table{Title: "Carbon footprint report", Period: "2026-09-12 to 2026-10-10", Organization: "Dev Organization", Scope: "All projects",
		Notes:   []string{"Methodology RLX-CCF-1 (provisional): energy of usage-based rows follows published coefficients; rows marked cost_based are rough estimates from spend."},
		Headers: []string{"day", "provider", "service", "region", "method", "energy_kwh", "carbon_kg_co2e", "methodology_version", "intensity_g_per_kwh", "intensity_source", "intensity_basis"}}
	for i := 0; i < 30; i++ {
		carbon.Rows = append(carbon.Rows, []string{"2026-10-09", "azure", "Azure App Service", "canadacentral", "cost_based", "4.101084", "0.455220", "RLX-CCF-1", "111.0", "electricitymaps", "latest_overall"})
	}
	sustain := domain.Table{Title: "Sustainability summary", Period: "2026-09-12 to 2026-10-10", Organization: "Dev Organization", Scope: "All projects",
		Notes: []string{"Emissions are operational only."}, Headers: []string{"section", "item", "value", "unit"},
		Rows: [][]string{{"Footprint", "Operational emissions", "52.858", "kg CO2e"}, {"Footprint", "Energy", "511.18", "kWh"}, {"Data quality", "Share measured from usage", "0", "%"}, {"Recommendations", "Open", "10", ""}}}
	for name, tb := range map[string]domain.Table{"carbon": carbon, "sustainability": sustain} {
		if out, err := PDF(tb, at); err != nil || !bytes.HasPrefix(out, []byte("%PDF-")) {
			t.Errorf("%s: %v", name, err)
		} else if dir := os.Getenv("PDF_OUT"); dir != "" {
			_ = os.WriteFile(dir+"/"+name+".pdf", out, 0o600)
		}
	}
	if dir := os.Getenv("PDF_OUT"); dir != "" {
		_ = os.WriteFile(dir+"/finops.pdf", b, 0o600)
	}
}

func TestPDFRefusesATableWithoutColumns(t *testing.T) {
	if _, err := PDF(domain.Table{Title: "x"}, at); err == nil {
		t.Error("a report without columns is not a report")
	}
}

func TestPDFTextIsMappedToWhatTheCoreFontsDraw(t *testing.T) {
	got := pdfText(func(s string) string { return s }, "CO₂e → ≈ 5 · done")
	if strings.ContainsAny(got, "₂→≈·") {
		t.Errorf("unmapped characters left: %q", got)
	}
}
