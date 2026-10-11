// Package render turns a neutral reports Table into CSV, JSON or PDF bytes. It is pure: no I/O.
package render

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"time"

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
	// The first line is the header, so a spreadsheet or a data-frame reads the table as it is. The caveats that must travel
	// with the figures follow the table as footnotes, after an empty line.
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
	if len(t.Notes) > 0 {
		if err := w.Write(nil); err != nil {
			return nil, err
		}
		for _, n := range t.Notes {
			if err := w.Write([]string{"# " + n}); err != nil {
				return nil, err
			}
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
