package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

// SourceCostManagement is the focus.RawBatch.Source of the aggregate read through the Cost Management Query API.
const SourceCostManagement = "cost_management_api"

const (
	costAPIVersion = "2023-11-01"
	armBase        = "https://management.azure.com"
	maxCostPages   = 100
	maxCostRows    = 500_000
	maxBodyBytes   = 32 << 20
	maxRetries     = 5
	maxRetryWait   = 120 * time.Second
	queryPacing    = 2 * time.Second // between queries of one sync: Cost Management throttles bursts
	queryChunkDays = 92              // the API refuses very long daily windows: a sync reads at most a quarter at a time
)

// CostAPI reads costs of a subscription with the customer's own service principal.
type CostAPI struct {
	HTTP      *http.Client
	Authority string // token endpoint base; "" = Microsoft's
	ARM       string // management endpoint base; "" = https://management.azure.com
	// Pace is the wait before each query of a sync (0 = none, as in tests).
	Pace time.Duration
	// Sleep waits between throttled retries (replaced in tests).
	Sleep func(context.Context, time.Duration) error
}

func (a *CostAPI) arm() string {
	if a.ARM != "" {
		return strings.TrimRight(a.ARM, "/")
	}
	return armBase
}

func (a *CostAPI) sleep(ctx context.Context, d time.Duration) error {
	if a.Sleep != nil {
		return a.Sleep(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// costRow is one line of the answer: a day, the grouping values and the amount.
type costRow struct {
	Day      string
	Service  string
	Location string
	Currency string
	Amount   float64
}

type queryResponse struct {
	Properties struct {
		NextLink string `json:"nextLink"`
		Columns  []struct {
			Name string `json:"name"`
		} `json:"columns"`
		Rows [][]any `json:"rows"`
	} `json:"properties"`
}

// query runs one Cost Management query (ActualCost or AmortizedCost), grouped as asked, and follows nextLink.
func (a *CostAPI) query(ctx context.Context, token, subscription, costType string, from, to time.Time, groupBy []string) ([]costRow, error) {
	body := map[string]any{
		"type": costType, "timeframe": "Custom",
		"timePeriod": map[string]string{"from": from.UTC().Format(time.DateOnly) + "T00:00:00Z", "to": to.AddDate(0, 0, -1).UTC().Format(time.DateOnly) + "T23:59:59Z"},
		"dataset": map[string]any{
			"granularity": "Daily",
			"aggregation": map[string]any{"totalCost": map[string]string{"name": "Cost", "function": "Sum"}},
		},
	}
	if len(groupBy) > 0 {
		var g []map[string]string
		for _, d := range groupBy {
			g = append(g, map[string]string{"type": "Dimension", "name": d})
		}
		body["dataset"].(map[string]any)["grouping"] = g
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	if a.Pace > 0 {
		if err := a.sleep(ctx, a.Pace); err != nil {
			return nil, err
		}
	}
	base := a.arm()
	next := base + "/subscriptions/" + url.PathEscape(subscription) + "/providers/Microsoft.CostManagement/query?api-version=" + costAPIVersion
	var out []costRow
	for page := 0; next != ""; page++ {
		if page >= maxCostPages {
			return nil, errors.New("azure: cost query returned too many pages (limit exceeded)")
		}
		resp, err := a.post(ctx, token, next, raw)
		if err != nil {
			return nil, err
		}
		cols := make([]string, len(resp.Properties.Columns))
		for i, c := range resp.Properties.Columns {
			cols[i] = strings.ToLower(c.Name)
		}
		idx := func(names ...string) int {
			for _, n := range names {
				for i, c := range cols {
					if c == n {
						return i
					}
				}
			}
			return -1
		}
		ci, di, si, li, ri := idx("cost", "pretaxcost", "costusd"), idx("usagedate", "billingmonth"), idx("servicename"), idx("resourcelocation"), idx("currency")
		if ci < 0 || di < 0 {
			return nil, errors.New("azure: unexpected cost query answer (no cost or date column)")
		}
		for _, r := range resp.Properties.Rows {
			if len(r) != len(cols) {
				return nil, errors.New("azure: unexpected cost query row")
			}
			day, err := usageDate(r[di])
			if err != nil {
				return nil, err
			}
			row := costRow{Day: day, Amount: number(r[ci])}
			if si >= 0 {
				row.Service, _ = r[si].(string)
			}
			if li >= 0 {
				row.Location, _ = r[li].(string)
			}
			if ri >= 0 {
				row.Currency, _ = r[ri].(string)
			}
			out = append(out, row)
			if len(out) > maxCostRows {
				return nil, errors.New("azure: cost query returned too many rows (limit exceeded)")
			}
		}
		next = resp.Properties.NextLink
		if next != "" {
			// A next link is followed only on the management host: the bearer token must not go anywhere else.
			u, err := url.Parse(next)
			b, _ := url.Parse(base)
			if err != nil || b == nil || u.Scheme != b.Scheme || u.Host != b.Host {
				return nil, errors.New("azure: cost query next link points outside the management endpoint")
			}
		}
	}
	return out, nil
}

// post sends the query, waiting out throttling (429, with Retry-After) a bounded number of times.
func (a *CostAPI) post(ctx context.Context, token, target string, body []byte) (queryResponse, error) {
	var qr queryResponse
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
		if err != nil {
			return qr, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := a.HTTP.Do(req)
		if err != nil {
			return qr, fmt.Errorf("azure: cost query: %w", err)
		}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
		_ = resp.Body.Close()
		switch {
		case resp.StatusCode == http.StatusOK:
			if err := json.Unmarshal(data, &qr); err != nil {
				return qr, fmt.Errorf("azure: unreadable cost query answer: %w", err)
			}
			return qr, nil
		case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
			return qr, fmt.Errorf("%w: the service principal may not read this subscription's costs (role Cost Management Reader)", domain.ErrAccessDenied)
		case resp.StatusCode == http.StatusNotFound:
			return qr, fmt.Errorf("%w: the subscription was not found for this service principal", domain.ErrAccessDenied)
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable:
			wait := throttleWait(resp.Header)
			if attempt >= maxRetries || wait > maxRetryWait {
				return qr, domain.PublicError{Message: fmt.Sprintf("Azure Cost Management is throttling requests (HTTP %d, retry in %ds); the sync is retried automatically.", resp.StatusCode, int(wait.Seconds()))}
			}
			if err := a.sleep(ctx, wait); err != nil {
				return qr, err
			}
		case resp.StatusCode == http.StatusBadRequest:
			return qr, &badQueryError{code: azureErrorCode(data)}
		default:
			return qr, domain.PublicError{Message: fmt.Sprintf("Azure Cost Management answered HTTP %d%s; the sync is retried automatically.", resp.StatusCode, azureErrorSuffix(data))}
		}
	}
}

// badQueryError is HTTP 400: the API refused the query itself (for example an unsupported cost type for the offer).
type badQueryError struct{ code string }

func (e *badQueryError) Error() string {
	return "azure: cost query refused (HTTP 400" + map[bool]string{true: ", " + e.code, false: ""}[e.code != ""] + ")"
}

// azureErrorCode reads error.code of an Azure error body ("BadRequest", "InvalidAuthenticationToken" ...): a short
// machine word, safe to show.
func azureErrorCode(body []byte) string {
	var e struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &e) != nil || len(e.Error.Code) > 64 || strings.ContainsAny(e.Error.Code, " \n\t") {
		return ""
	}
	return e.Error.Code
}

func azureErrorSuffix(body []byte) string {
	if c := azureErrorCode(body); c != "" {
		return " (" + c + ")"
	}
	return ""
}

// throttleWait reads how long Azure asks to wait: the standard Retry-After, or one of the Cost Management specific
// x-ms-ratelimit-microsoft.costmanagement-*-retry-after headers (seconds). The longest one wins.
func throttleWait(h http.Header) time.Duration {
	var wait time.Duration
	found := false
	for name, vals := range h {
		l := strings.ToLower(name)
		if l != "retry-after" && !(strings.HasPrefix(l, "x-ms-ratelimit-microsoft.costmanagement-") && strings.HasSuffix(l, "-retry-after")) {
			continue
		}
		for _, v := range vals {
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 0 {
				found = true
				wait = max(wait, time.Duration(n)*time.Second)
			}
		}
	}
	if !found {
		return 15 * time.Second
	}
	return wait
}

func retryAfter(h string) time.Duration {
	if n, err := strconv.Atoi(strings.TrimSpace(h)); err == nil && n >= 0 {
		return time.Duration(n) * time.Second
	}
	return 10 * time.Second
}

// usageDate reads the numeric yyyymmdd the API uses ("20261001" arrives as a JSON number) or an ISO date.
func usageDate(v any) (string, error) {
	var s string
	switch t := v.(type) {
	case float64:
		s = strconv.FormatFloat(t, 'f', 0, 64)
	case string:
		s = t
	}
	if len(s) >= 10 && s[4] == '-' {
		s = s[:10]
		if _, err := time.Parse(time.DateOnly, s); err == nil {
			return s, nil
		}
	}
	if d, err := time.Parse("20060102", s); err == nil {
		return d.Format(time.DateOnly), nil
	}
	return "", fmt.Errorf("azure: unreadable usage date %v", v)
}

func number(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	}
	return 0
}

// windows splits [from, to) into chunks the API accepts.
func windows(from, to time.Time) [][2]time.Time {
	var out [][2]time.Time
	for s := from; s.Before(to); {
		e := s.AddDate(0, 0, queryChunkDays)
		if e.After(to) {
			e = to
		}
		out = append(out, [2]time.Time{s, e})
		s = e
	}
	return out
}

type apiCostKey struct{ day, service, location, currency string }

// usage reads billed (actual) and amortized cost per day, service, location and currency.
func (a *CostAPI) usage(ctx context.Context, token, subscription string, from, to time.Time) (focus.RawBatch, error) {
	type sums struct{ billed, effective float64 }
	agg := map[apiCostKey]*sums{}
	amortizedUnavailable := false
	for _, w := range windows(from, to) {
		for i, costType := range []string{"ActualCost", "AmortizedCost"} {
			rows, err := a.query(ctx, token, subscription, costType, w[0], w[1], []string{"ServiceName", "ResourceLocation"})
			var bad *badQueryError
			switch {
			case errors.As(err, &bad) && i == 1:
				// The amortized view is not offered for every kind of subscription. Without reservations it equals the
				// billed amount, so the billed amount stands in for it rather than failing the whole sync.
				rows, amortizedUnavailable = nil, true
			case errors.As(err, &bad):
				return focus.RawBatch{}, domain.PublicError{Message: "Azure Cost Management refused the cost query for this subscription (" + bad.code + "). Costs may not be available for this kind of subscription."}
			case err != nil:
				return focus.RawBatch{}, err
			}
			for _, r := range rows {
				k := apiCostKey{r.Day, r.Service, r.Location, strings.ToUpper(strings.TrimSpace(r.Currency))}
				s := agg[k]
				if s == nil {
					s = &sums{}
					agg[k] = s
				}
				if i == 0 {
					s.billed += r.Amount
				} else {
					s.effective += r.Amount
				}
			}
		}
	}
	type row struct {
		Day       string  `json:"day"`
		Service   string  `json:"service"`
		Location  string  `json:"location"`
		Currency  string  `json:"currency"`
		Billed    float64 `json:"billed"`
		Effective float64 `json:"effective"`
	}
	rows := make([]row, 0, len(agg))
	for k, v := range agg {
		if amortizedUnavailable {
			v.effective = v.billed
		}
		rows = append(rows, row{k.day, k.service, k.location, k.currency, v.billed, v.effective})
	}
	sort.Slice(rows, func(i, j int) bool {
		x, y := rows[i], rows[j]
		return x.Day+x.Service+x.Location+x.Currency < y.Day+y.Service+y.Location+y.Currency
	})
	payload, err := json.Marshal(map[string]any{"rows": rows})
	if err != nil {
		return focus.RawBatch{}, err
	}
	return focus.RawBatch{Provider: "azure", Source: SourceCostManagement, Payload: payload}, nil
}

// controlTotals reads the day totals of the billed cost with a query that has no grouping: an independent figure the
// detailed rows must add up to.
func (a *CostAPI) controlTotals(ctx context.Context, token, subscription string, from, to time.Time) ([]focus.ControlTotal, error) {
	sum := map[[2]string]float64{}
	for _, w := range windows(from, to) {
		rows, err := a.query(ctx, token, subscription, "ActualCost", w[0], w[1], nil)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			sum[[2]string{r.Day, strings.ToUpper(strings.TrimSpace(r.Currency))}] += r.Amount
		}
	}
	out := make([]focus.ControlTotal, 0, len(sum))
	for k, v := range sum {
		out = append(out, focus.ControlTotal{Day: k[0], Currency: k[1], Billed: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Day+out[i].Currency < out[j].Day+out[j].Currency })
	return out, nil
}
