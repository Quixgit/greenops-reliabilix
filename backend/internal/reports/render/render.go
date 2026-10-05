// Package render turns a neutral reports Table into CSV, JSON or PDF bytes. It is pure: no I/O.
package render

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"

	"github.com/quixgit/greenops-reliabilix/backend/internal/reports/domain"
)

// MaxPDFRows keeps PDFs readable; larger reports should use CSV.
const MaxPDFRows = 2000

// Render produces the file for the requested format.
func Render(t domain.Table, f domain.Format, generatedAt time.Time) ([]byte, error) {
	switch f {
	case domain.CSV:
		return CSV(t)
	case domain.JSON:
		return JSON(t, generatedAt)
	case domain.PDF:
		return PDF(t, generatedAt)
	}
	return nil, domain.ErrInvalidReport
}

// safeCell neutralizes spreadsheet formula injection (CSV injection): cells starting with = + - @ are prefixed
// with an apostrophe. Numeric cells (including negative numbers) are left alone.
func safeCell(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '@', '\t', '\r':
		return "'" + s
	case '-':
		var f float64
		if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
			return s
		}
		return "'" + s
	}
	return s
}

func CSV(t domain.Table) ([]byte, error) {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	for _, n := range t.Notes {
		if err := w.Write([]string{"# " + n}); err != nil {
			return nil, err
		}
	}
	if err := w.Write(t.Headers); err != nil {
		return nil, err
	}
	for _, r := range t.Rows {
		row := make([]string, len(r))
		for i, c := range r {
			row[i] = safeCell(c)
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return b.Bytes(), w.Error()
}

func JSON(t domain.Table, generatedAt time.Time) ([]byte, error) {
	rows := make([]map[string]string, 0, len(t.Rows))
	for _, r := range t.Rows {
		m := make(map[string]string, len(t.Headers))
		for i, h := range t.Headers {
			if i < len(r) {
				m[h] = r[i]
			}
		}
		rows = append(rows, m)
	}
	return json.MarshalIndent(map[string]any{"title": t.Title, "period": t.Period, "generated_at": generatedAt.UTC().Format(time.RFC3339), "notes": t.Notes, "rows": rows}, "", "  ")
}

func PDF(t domain.Table, generatedAt time.Time) ([]byte, error) {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 12)
	tr := pdf.UnicodeTranslatorFromDescriptor("") // core fonts are Latin-1: map the rest safely
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 9, tr(t.Title), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.CellFormat(0, 5, tr(fmt.Sprintf("%s  |  generated %s UTC  |  Reliabilix GreenOps", t.Period, generatedAt.UTC().Format("2006-01-02 15:04"))), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "I", 8)
	for _, n := range t.Notes {
		pdf.MultiCell(0, 4, tr(n), "", "L", false)
	}
	pdf.Ln(2)

	cols := len(t.Headers)
	if cols == 0 {
		return nil, domain.ErrInvalidReport
	}
	w := 277.0 / float64(cols)
	header := func() {
		pdf.SetFont("Helvetica", "B", 8)
		pdf.SetFillColor(230, 236, 250)
		for _, h := range t.Headers {
			pdf.CellFormat(w, 6, tr(clip(h, w)), "1", 0, "L", true, 0, "")
		}
		pdf.Ln(-1)
		pdf.SetFont("Helvetica", "", 8)
	}
	header()
	for i, r := range t.Rows {
		if i >= MaxPDFRows {
			pdf.SetFont("Helvetica", "I", 8)
			pdf.CellFormat(0, 6, tr(fmt.Sprintf("... %d more rows omitted. Use the CSV export for the full data.", len(t.Rows)-MaxPDFRows)), "", 1, "L", false, 0, "")
			break
		}
		if pdf.GetY() > 190 {
			pdf.AddPage()
			header()
		}
		for _, c := range r {
			pdf.CellFormat(w, 5.5, tr(clip(c, w)), "1", 0, "L", false, 0, "")
		}
		pdf.Ln(-1)
	}
	var b bytes.Buffer
	if err := pdf.Output(&b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// clip shortens text to roughly fit a column (about 2 characters per mm at 8pt).
func clip(s string, widthMM float64) string {
	max := int(widthMM * 2.1)
	if max < 4 {
		max = 4
	}
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max-1]) + "…"
}
