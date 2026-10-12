package application

import (
	"context"
	"math"
	"strconv"
	"strings"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/i18n"
	"github.com/quixgit/greenops-reliabilix/backend/internal/reports/domain"
)

// digestBody puts the headline figures of the report's period in front of the usual line, so a scheduled report's e-mail says
// something before it asks for a click. Without a reader, or on any failure, the body stays as it was.
func (s Service) digestBody(ctx context.Context, tenantID string, r domain.Report, body string) string {
	if s.Head == nil {
		return body
	}
	h, err := s.Head.Headline(ctx, tenantID, r.ProjectID, r.PeriodStart, r.PeriodEnd.AddDate(0, 0, 1))
	if err != nil || (h.Cost == 0 && h.CarbonKg == 0) {
		return body
	}
	return headlineText(i18n.Default, h) + "\n" + body
}

func headlineText(lang string, h domain.Headline) string {
	var b strings.Builder
	b.WriteString(i18n.T(lang, "digest.headline", "cost", money(h.Cost), "currency", h.Currency, "cost_change", change(lang, h.Cost, h.PrevCost),
		"carbon", strconv.FormatFloat(math.Round(h.CarbonKg*10)/10, 'f', -1, 64), "carbon_change", change(lang, h.CarbonKg, h.PrevCarbonKg)))
	if h.TopService != "" {
		b.WriteString("\n" + i18n.T(lang, "digest.top", "service", h.TopService, "cost", money(h.TopServiceCost), "currency", h.Currency))
	}
	return b.String()
}

func money(v float64) string { return strconv.FormatFloat(math.Round(v*100)/100, 'f', 2, 64) }

// change is the move from the previous period as a signed percentage, or a word when there is nothing to divide by.
func change(lang string, cur, prev float64) string {
	switch {
	case prev <= 0 && cur > 0:
		return i18n.T(lang, "digest.new")
	case prev <= 0:
		return i18n.T(lang, "digest.flat")
	}
	p := (cur - prev) / prev * 100
	if math.Abs(p) < 0.5 {
		return i18n.T(lang, "digest.flat")
	}
	sign := "+"
	if p < 0 {
		sign = ""
	}
	return sign + strconv.FormatFloat(math.Round(p), 'f', -1, 64) + "%"
}
