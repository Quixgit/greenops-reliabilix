package render

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"

	"github.com/quixgit/greenops-reliabilix/backend/internal/reports/domain"
)

// The report as a document someone can hand to a colleague or an auditor: a running header with the product's mark, a title
// block that says for whom, what and when, the headline figures, where they come from, a short account of what the figures are
// and how they were made, and then the data, in a table that repeats its header on every page. Landscape A4, because a report
// has many columns. Core fonts only (no files to ship); text is mapped to what they can draw.

var (
	cNavy   = [3]int{18, 35, 60}
	cLeaf   = [3]int{47, 143, 120}
	cLeafLt = [3]int{232, 246, 241}
	cInk    = [3]int{15, 27, 51}
	cMuted  = [3]int{107, 122, 148}
	cLine   = [3]int{221, 228, 240}
	cZebra  = [3]int{247, 249, 252}
)

const (
	pageW, pageH = 297.0, 210.0
	marginX      = 14.0
	contentW     = pageW - 2*marginX
	bottomLimit  = pageH - 17 // below this a new page starts
)

// pdfText maps what the core fonts cannot draw (subscripts, arrows, approximations) to plain forms, then to Latin-1.
func pdfText(tr func(string) string, s string) string {
	r := strings.NewReplacer("₂", "2", "→", "->", "≈", "~", "≥", ">=", "≤", "<=", "−", "-", "×", "x", "·", "-", "…", "...")
	return tr(r.Replace(s))
}

func color(pdf *fpdf.Fpdf, c [3]int, kind string) {
	switch kind {
	case "fill":
		pdf.SetFillColor(c[0], c[1], c[2])
	case "draw":
		pdf.SetDrawColor(c[0], c[1], c[2])
	default:
		pdf.SetTextColor(c[0], c[1], c[2])
	}
}

type column struct {
	head    string
	w       float64
	numeric bool
	decimal int
}

func parseNum(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v, err == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
}

// decimalsFor says how many decimals a column of figures is worth printing: money in cents, energy and carbon to the gram.
func decimalsFor(head string) int {
	h := strings.ToLower(head)
	switch {
	case strings.Contains(h, "cost"):
		return 2
	case strings.Contains(h, "intensity"):
		return 1
	case strings.Contains(h, "kwh"), strings.Contains(h, "carbon"), strings.Contains(h, "sci"):
		return 3
	default:
		return 3
	}
}

// prettyHeader turns "carbon_kg_co2e" into "Carbon kg CO2e".
func prettyHeader(h string) string {
	h = strings.ReplaceAll(h, "co2e", "CO2e")
	h = strings.ReplaceAll(h, "kwh", "kWh")
	h = strings.ReplaceAll(h, "_", " ")
	if h == "" {
		return h
	}
	return strings.ToUpper(h[:1]) + h[1:]
}

func fit(pdf *fpdf.Fpdf, s string, w float64) string {
	if pdf.GetStringWidth(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 1 && pdf.GetStringWidth(string(r)+"...") > w {
		r = r[:len(r)-1]
	}
	return string(r) + "..."
}

func humanKg(kg float64) string {
	if kg >= 1000 {
		return fmt.Sprintf("%.2f t CO2e", kg/1000)
	}
	return fmt.Sprintf("%.1f kg CO2e", kg)
}

func thousands(v float64, dec int) string {
	s := strconv.FormatFloat(v, 'f', dec, 64)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	whole, frac, _ := strings.Cut(s, ".")
	var b strings.Builder
	for i, c := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	out := b.String()
	if frac != "" {
		out += "." + frac
	}
	if neg {
		out = "-" + out
	}
	return out
}

type stat struct{ label, value, note string }

// kpis are the headline figures a reader looks for first, found in whatever columns the report has.
func kpis(t domain.Table) []stat {
	idx := map[string]int{}
	for i, h := range t.Headers {
		idx[h] = i
	}
	sum := func(h string) (float64, bool) {
		i, ok := idx[h]
		if !ok {
			return 0, false
		}
		var s float64
		var n int
		for _, r := range t.Rows {
			if i < len(r) {
				if v, ok := parseNum(r[i]); ok {
					s += v
					n++
				}
			}
		}
		return s, n > 0
	}
	var out []stat
	if v, ok := sum("billed_cost"); ok {
		cur := ""
		if i, has := idx["currency"]; has && len(t.Rows) > 0 && i < len(t.Rows[0]) {
			cur = t.Rows[0][i]
			for _, r := range t.Rows {
				if i < len(r) && r[i] != cur {
					cur = "" // several currencies: a single total would be a lie
					break
				}
			}
		}
		if cur != "" {
			out = append(out, stat{"Billed cost", thousands(v, 2) + " " + cur, "as invoiced, no currency conversion"})
		} else {
			out = append(out, stat{"Billed cost", "several currencies", "see the currency column"})
		}
	}
	if v, ok := sum("carbon_kg_co2e"); ok && idx["energy_kwh"] > 0 || ok {
		out = append(out, stat{"Operational emissions", humanKg(v), "embodied carbon not included"})
	}
	if v, ok := sum("energy_kwh"); ok {
		out = append(out, stat{"Energy", thousands(v, 1) + " kWh", "estimated, not metered"})
	}
	if i, ok := idx["day"]; ok {
		days := map[string]bool{}
		for _, r := range t.Rows {
			if i < len(r) {
				days[r[i]] = true
			}
		}
		out = append(out, stat{"Days covered", strconv.Itoa(len(days)), fmt.Sprintf("%s rows", thousands(float64(len(t.Rows)), 0))})
	}
	if len(out) > 4 {
		out = out[:4]
	}
	return out
}

type item struct {
	label string
	value float64
}

// breakdown sums a metric by one dimension and returns the largest, most to least.
func breakdown(t domain.Table, dim, metric string, n int) []item {
	di, mi := -1, -1
	for i, h := range t.Headers {
		if h == dim {
			di = i
		}
		if h == metric {
			mi = i
		}
	}
	if di < 0 || mi < 0 {
		return nil
	}
	m := map[string]float64{}
	for _, r := range t.Rows {
		if di < len(r) && mi < len(r) {
			if v, ok := parseNum(r[mi]); ok && v > 0 {
				m[r[di]] += v
			}
		}
	}
	out := make([]item, 0, len(m))
	for k, v := range m {
		out = append(out, item{k, v})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].value > out[j].value || (out[i].value == out[j].value && out[i].label < out[j].label)
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

func about(t domain.Table) []string {
	has := func(h string) bool {
		for _, x := range t.Headers {
			if x == h {
				return true
			}
		}
		return false
	}
	switch {
	case has("billed_cost"):
		return []string{
			"This report lists what the platform read from your connected cloud accounts: cost per day, service and region, in the billing currency of the account.",
			"Cost follows the FOCUS specification. Billed cost is what the provider invoiced; effective cost also reflects discounts and amortization. Amounts are never converted between currencies in this report.",
		}
	case has("functional_unit"):
		return []string{
			"This report relates the carbon of a project to the work it produced: the Software Carbon Intensity (SCI) per functional unit, day by day.",
			"A day without reported functional units has no SCI: it is left empty, not estimated.",
		}
	case has("energy_kwh"):
		return []string{
			"This report lists the energy your cloud usage is estimated to need and the carbon that follows from it, per day, service and region.",
			"Each row says how its energy was estimated (method) and where the grid carbon intensity came from (source and basis), so a figure is never more precise than its data.",
		}
	}
	return nil
}

// PDF renders a report as a formatted document.
func PDF(t domain.Table, generatedAt time.Time) ([]byte, error) {
	if len(t.Headers) == 0 {
		return nil, domain.ErrInvalidReport
	}
	pdf := fpdf.New("L", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	txt := func(s string) string { return pdfText(tr, s) }
	pdf.SetMargins(marginX, 24, marginX)
	pdf.SetAutoPageBreak(true, 17)
	pdf.AliasNbPages("{nb}")
	pdf.SetTitle(txt(t.Title), true)
	pdf.SetAuthor("Reliabilix GreenOps Platform", true)
	pdf.SetCreator("Reliabilix", true)
	pdf.SetSubject(txt(t.Period), true)
	stamp := generatedAt.UTC().Format("2006-01-02 15:04") + " UTC"

	pdf.SetHeaderFuncMode(func() {
		// the mark: a leaf-green tile with an R, then the name, and the report on the right
		color(pdf, cLeaf, "fill")
		pdf.RoundedRect(marginX, 8, 8, 8, 2, "1234", "F")
		pdf.SetFont("Helvetica", "B", 11)
		color(pdf, [3]int{255, 255, 255}, "text")
		pdf.SetXY(marginX, 8)
		pdf.CellFormat(8, 8, "R", "", 0, "C", false, 0, "")
		pdf.SetFont("Helvetica", "B", 12)
		color(pdf, cNavy, "text")
		pdf.SetXY(marginX+11, 7.6)
		pdf.CellFormat(50, 5, "Reliabilix", "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 7)
		color(pdf, cMuted, "text")
		pdf.SetXY(marginX+11, 12.2)
		pdf.CellFormat(50, 4, "GreenOps + FinOps", "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetXY(pageW-marginX-150, 9.5)
		pdf.CellFormat(150, 5, fit(pdf, txt(t.Title+"  |  "+t.Period), 150), "", 0, "R", false, 0, "")
		color(pdf, cLeaf, "draw")
		pdf.SetLineWidth(0.5)
		pdf.Line(marginX, 19, marginX+22, 19)
		color(pdf, cLine, "draw")
		pdf.SetLineWidth(0.2)
		pdf.Line(marginX+22, 19, pageW-marginX, 19)
	}, true)
	pdf.SetFooterFunc(func() {
		color(pdf, cLine, "draw")
		pdf.SetLineWidth(0.2)
		pdf.Line(marginX, pageH-13, pageW-marginX, pageH-13)
		pdf.SetFont("Helvetica", "", 7)
		color(pdf, cMuted, "text")
		left := "Generated " + stamp + "  |  Reliabilix GreenOps Platform"
		if t.Organization != "" {
			left += "  |  Prepared for " + t.Organization + ", confidential"
		}
		pdf.SetXY(marginX, pageH-11.5)
		pdf.CellFormat(contentW-30, 4, fit(pdf, txt(left), contentW-32), "", 0, "L", false, 0, "")
		pdf.SetXY(pageW-marginX-30, pageH-11.5)
		pdf.CellFormat(30, 4, fmt.Sprintf("Page %d of {nb}", pdf.PageNo()), "", 0, "R", false, 0, "")
	})
	pdf.AddPage()

	// ---- title block
	y := 25.0
	pdf.SetFont("Helvetica", "B", 7.5)
	color(pdf, cLeaf, "text")
	pdf.SetXY(marginX, y)
	pdf.CellFormat(60, 4, "REPORT", "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "B", 22)
	color(pdf, cNavy, "text")
	pdf.SetXY(marginX, y+4.5)
	pdf.CellFormat(contentW, 10, fit(pdf, txt(t.Title), contentW), "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	color(pdf, cMuted, "text")
	info := []string{}
	if t.Organization != "" {
		info = append(info, "Prepared for "+t.Organization)
	}
	if t.Scope != "" {
		info = append(info, t.Scope)
	}
	info = append(info, t.Period, "generated "+stamp)
	pdf.SetXY(marginX, y+15.5)
	pdf.CellFormat(contentW, 5, fit(pdf, txt(strings.Join(info, "   |   ")), contentW), "", 0, "L", false, 0, "")
	y += 25

	// ---- the sustainability summary is a list of findings, not a table of rows
	if len(t.Headers) == 4 && t.Headers[0] == "section" && t.Headers[2] == "value" {
		summarySections(pdf, txt, t, y)
		return finish(pdf)
	}

	// ---- headline figures
	if cards := kpis(t); len(cards) > 0 {
		gap := 5.0
		w := (contentW - gap*float64(len(cards)-1)) / float64(len(cards))
		for i, c := range cards {
			x := marginX + float64(i)*(w+gap)
			color(pdf, cLeafLt, "fill")
			pdf.RoundedRect(x, y, w, 24, 3, "1234", "F")
			color(pdf, cLeaf, "fill")
			pdf.Rect(x, y+4, 1.2, 16, "F")
			pdf.SetFont("Helvetica", "", 8)
			color(pdf, cMuted, "text")
			pdf.SetXY(x+5, y+3.5)
			pdf.CellFormat(w-7, 4, fit(pdf, txt(c.label), w-7), "", 0, "L", false, 0, "")
			pdf.SetFont("Helvetica", "B", 15)
			color(pdf, cNavy, "text")
			pdf.SetXY(x+5, y+8.5)
			pdf.CellFormat(w-7, 7, fit(pdf, txt(c.value), w-7), "", 0, "L", false, 0, "")
			pdf.SetFont("Helvetica", "", 7)
			color(pdf, cMuted, "text")
			pdf.SetXY(x+5, y+17)
			pdf.CellFormat(w-7, 4, fit(pdf, txt(c.note), w-7), "", 0, "L", false, 0, "")
		}
		y += 24 + 7
	}

	// ---- where it comes from: the two largest dimensions side by side
	metric := "billed_cost"
	unit := ""
	if has(t.Headers, "carbon_kg_co2e") {
		metric, unit = "carbon_kg_co2e", " kg"
	}
	left, right := breakdown(t, "service", metric, 6), breakdown(t, "region", metric, 6)
	if len(left) > 0 || len(right) > 0 {
		half := (contentW - 10) / 2
		hMax := 0.0
		if len(left) > 0 {
			hMax = math.Max(hMax, drawBars(pdf, txt, marginX, y, half, "Largest services", left, unit))
		}
		if len(right) > 0 {
			hMax = math.Max(hMax, drawBars(pdf, txt, marginX+half+10, y, half, "Largest regions", right, unit))
		}
		y += hMax + 6
	}

	// ---- what the figures are, in sentences
	pdf.SetXY(marginX, y)
	for _, p := range about(t) {
		pdf.SetFont("Helvetica", "", 8.5)
		color(pdf, cInk, "text")
		pdf.SetX(marginX)
		pdf.MultiCell(contentW, 4.3, txt(p), "", "L", false)
		pdf.Ln(1.2)
	}
	if len(t.Notes) > 0 {
		pdf.Ln(1)
		pdf.SetFont("Helvetica", "B", 9)
		color(pdf, cNavy, "text")
		pdf.SetX(marginX)
		pdf.CellFormat(contentW, 5, "Notes and method", "", 1, "L", false, 0, "")
		for _, n := range t.Notes {
			if pdf.GetY() > bottomLimit-12 {
				pdf.AddPage()
			}
			pdf.SetFont("Helvetica", "", 8)
			color(pdf, cMuted, "text")
			x0, y0 := marginX, pdf.GetY()
			pdf.SetXY(x0, y0)
			pdf.CellFormat(4, 4, "-", "", 0, "L", false, 0, "")
			pdf.SetXY(x0+4, y0)
			pdf.MultiCell(contentW-4, 4, txt(n), "", "L", false)
			pdf.Ln(0.6)
		}
	}

	// ---- the data
	pdf.Ln(3)
	if pdf.GetY() > bottomLimit-30 {
		pdf.AddPage()
	}
	pdf.SetFont("Helvetica", "B", 11)
	color(pdf, cNavy, "text")
	pdf.SetX(marginX)
	shown := len(t.Rows)
	if shown > MaxPDFRows {
		shown = MaxPDFRows
	}
	pdf.CellFormat(contentW/2, 6, "Details", "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 8)
	color(pdf, cMuted, "text")
	more := ""
	if len(t.Rows) > shown {
		more = fmt.Sprintf("first %s of %s rows  |  the CSV export has all of them", thousands(float64(shown), 0), thousands(float64(len(t.Rows)), 0))
	} else {
		more = fmt.Sprintf("%s rows", thousands(float64(len(t.Rows)), 0))
	}
	pdf.CellFormat(contentW/2, 6, txt(more), "", 1, "R", false, 0, "")
	pdf.Ln(1)
	table(pdf, txt, t, shown)
	return finish(pdf)
}

func has(headers []string, h string) bool {
	for _, x := range headers {
		if x == h {
			return true
		}
	}
	return false
}

func finish(pdf *fpdf.Fpdf) ([]byte, error) {
	var b bytes.Buffer
	if err := pdf.Output(&b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// drawBars is a small ranked list with a bar for each value; it returns the height it used.
func drawBars(pdf *fpdf.Fpdf, txt func(string) string, x, y, w float64, title string, items []item, unit string) float64 {
	pdf.SetFont("Helvetica", "B", 9)
	color(pdf, cNavy, "text")
	pdf.SetXY(x, y)
	pdf.CellFormat(w, 5, txt(title), "", 0, "L", false, 0, "")
	top := items[0].value
	var total float64
	for _, it := range items {
		total += it.value
	}
	rowH := 6.2
	labelW, valueW := w*0.42, 26.0
	barW := w - labelW - valueW - 4
	for i, it := range items {
		ry := y + 6.5 + float64(i)*rowH
		pdf.SetFont("Helvetica", "", 8)
		color(pdf, cInk, "text")
		pdf.SetXY(x, ry)
		pdf.CellFormat(labelW, rowH, fit(pdf, txt(it.label), labelW-2), "", 0, "L", false, 0, "")
		color(pdf, cLeafLt, "fill")
		pdf.RoundedRect(x+labelW, ry+1.6, barW, 3, 1.5, "1234", "F")
		color(pdf, cLeaf, "fill")
		pdf.RoundedRect(x+labelW, ry+1.6, math.Max(1.5, barW*it.value/top), 3, 1.5, "1234", "F")
		pdf.SetFont("Helvetica", "B", 8)
		color(pdf, cNavy, "text")
		pdf.SetXY(x+labelW+barW+2, ry)
		pdf.CellFormat(valueW, rowH, txt(thousands(it.value, decimalsFor(unit+"cost"))+strings.TrimSpace(unit)), "", 0, "R", false, 0, "")
	}
	return 6.5 + float64(len(items))*rowH
}

// table draws the rows: a header that repeats, zebra stripes, figures aligned right, text cut to its column.
func table(pdf *fpdf.Fpdf, txt func(string) string, t domain.Table, shown int) {
	cols := make([]column, len(t.Headers))
	const pad = 3.0
	pdf.SetCellMargin(2) // a text column never touches the figures beside it
	defer pdf.SetCellMargin(1)
	pdf.SetFont("Helvetica", "", 7.5)
	sample := t.Rows
	if len(sample) > 300 {
		sample = sample[:300]
	}
	var total float64
	for i, h := range t.Headers {
		c := column{head: prettyHeader(h), decimal: decimalsFor(h)}
		nums, filled := 0, 0
		maxW := 0.0
		pdf.SetFont("Helvetica", "B", 7.5)
		maxW = pdf.GetStringWidth(txt(c.head))
		pdf.SetFont("Helvetica", "", 7.5)
		for _, r := range sample {
			if i >= len(r) || r[i] == "" {
				continue
			}
			filled++
			if _, ok := parseNum(r[i]); ok && !strings.Contains(h, "version") && h != "day" {
				nums++
			}
			if w := pdf.GetStringWidth(txt(r[i])); w > maxW {
				maxW = w
			}
		}
		c.numeric = filled > 0 && float64(nums)/float64(filled) > 0.9
		c.w = math.Min(maxW+2*pad, 70)
		cols[i] = c
		total += c.w
	}
	if total > contentW { // shrink the widest columns first, never below a readable minimum
		for total > contentW+0.01 {
			wi, wmax := 0, 0.0
			for i, c := range cols {
				if c.w > wmax {
					wi, wmax = i, c.w
				}
			}
			cut := math.Min(total-contentW, math.Max(1, wmax*0.05))
			if wmax-cut < 16 {
				break
			}
			cols[wi].w -= cut
			total -= cut
		}
	}
	if total < contentW { // spread what is left over the text columns, so the table spans the page
		var flex float64
		for _, c := range cols {
			if !c.numeric {
				flex += c.w
			}
		}
		extra := contentW - total
		for i := range cols {
			if flex > 0 && !cols[i].numeric {
				cols[i].w += extra * cols[i].w / flex
			} else if flex == 0 {
				cols[i].w += extra / float64(len(cols))
			}
		}
	}
	head := func() {
		color(pdf, cNavy, "fill")
		pdf.SetFont("Helvetica", "B", 7.5)
		color(pdf, [3]int{255, 255, 255}, "text")
		pdf.SetX(marginX)
		for i, c := range cols {
			al := "L"
			if c.numeric {
				al = "R"
			}
			pdf.CellFormat(c.w, 7, fit(pdf, txt(c.head), c.w-2*pad+1), "", 0, al, true, 0, "")
			_ = i
		}
		pdf.Ln(-1)
	}
	head()
	const rowH = 5.8
	for ri, r := range t.Rows {
		if ri >= shown {
			break
		}
		if pdf.GetY()+rowH > bottomLimit {
			pdf.AddPage()
			head()
		}
		pdf.SetX(marginX)
		pdf.SetFont("Helvetica", "", 7.5)
		color(pdf, cInk, "text")
		zebra := ri%2 == 1
		if zebra {
			color(pdf, cZebra, "fill")
		}
		for i, c := range cols {
			s := ""
			if i < len(r) {
				s = r[i]
			}
			al := "L"
			if c.numeric {
				al = "R"
				if v, ok := parseNum(s); ok {
					s = thousands(v, c.decimal)
				}
			}
			pdf.CellFormat(c.w, rowH, fit(pdf, txt(s), c.w-2*pad+1), "", 0, al, zebra, 0, "")
		}
		pdf.Ln(-1)
	}
	color(pdf, cLine, "draw")
	pdf.SetLineWidth(0.2)
	pdf.Line(marginX, pdf.GetY(), pageW-marginX, pdf.GetY())
}

// summarySections draws a report that is findings rather than rows: each section a heading with its figures beneath it.
func summarySections(pdf *fpdf.Fpdf, txt func(string) string, t domain.Table, y float64) {
	type kv struct{ item, value, unit string }
	var order []string
	groups := map[string][]kv{}
	for _, r := range t.Rows {
		if len(r) < 4 {
			continue
		}
		if _, ok := groups[r[0]]; !ok {
			order = append(order, r[0])
		}
		groups[r[0]] = append(groups[r[0]], kv{r[1], r[2], r[3]})
	}
	pdf.SetY(y)
	for _, sec := range order {
		rows := groups[sec]
		need := 9 + float64(len(rows))*6.2
		if pdf.GetY()+need > bottomLimit {
			pdf.AddPage()
		}
		pdf.SetX(marginX)
		pdf.SetFont("Helvetica", "B", 11)
		color(pdf, cNavy, "text")
		pdf.CellFormat(contentW, 7, txt(sec), "", 1, "L", false, 0, "")
		color(pdf, cLeaf, "draw")
		pdf.SetLineWidth(0.5)
		pdf.Line(marginX, pdf.GetY(), marginX+14, pdf.GetY())
		pdf.Ln(1.5)
		for i, r := range rows {
			pdf.SetX(marginX)
			zebra := i%2 == 0
			if zebra {
				color(pdf, cZebra, "fill")
			}
			pdf.SetFont("Helvetica", "", 8.5)
			color(pdf, cInk, "text")
			pdf.CellFormat(contentW*0.62, 6.2, "  "+fit(pdf, txt(r.item), contentW*0.62-4), "", 0, "L", zebra, 0, "")
			pdf.SetFont("Helvetica", "B", 8.5)
			color(pdf, cNavy, "text")
			v := r.value
			if f, ok := parseNum(v); ok {
				v = thousands(f, map[bool]int{true: 0, false: 2}[f == math.Trunc(f)])
			}
			pdf.CellFormat(contentW*0.24, 6.2, fit(pdf, txt(v), contentW*0.24-2), "", 0, "R", zebra, 0, "")
			pdf.SetFont("Helvetica", "", 8)
			color(pdf, cMuted, "text")
			pdf.CellFormat(contentW*0.14, 6.2, "  "+fit(pdf, txt(r.unit), contentW*0.14-3), "", 1, "L", zebra, 0, "")
		}
		pdf.Ln(4)
	}
	if len(t.Notes) > 0 {
		if pdf.GetY()+12 > bottomLimit {
			pdf.AddPage()
		}
		pdf.SetX(marginX)
		pdf.SetFont("Helvetica", "B", 9)
		color(pdf, cNavy, "text")
		pdf.CellFormat(contentW, 5, "Notes and method", "", 1, "L", false, 0, "")
		for _, n := range t.Notes {
			if pdf.GetY() > bottomLimit-10 {
				pdf.AddPage()
			}
			pdf.SetFont("Helvetica", "", 8)
			color(pdf, cMuted, "text")
			x0, y0 := marginX, pdf.GetY()
			pdf.SetXY(x0, y0)
			pdf.CellFormat(4, 4, "-", "", 0, "L", false, 0, "")
			pdf.SetXY(x0+4, y0)
			pdf.MultiCell(contentW-4, 4, txt(n), "", "L", false)
			pdf.Ln(0.6)
		}
	}
}
