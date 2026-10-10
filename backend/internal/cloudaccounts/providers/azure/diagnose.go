package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Diagnose runs the same three queries as a sync against one subscription and prints what Azure answers: the status,
// the columns and the first rows, or Azure's own error. It is for the person who runs `admin azure-check` on their
// own machine with their own secret, so it prints raw answers (the platform never calls it on its own).
// It returns true when every step worked.
func Diagnose(ctx context.Context, w io.Writer, tenant, client, secret, subscription string) bool {
	h := NoRedirects(&http.Client{Timeout: 2 * time.Minute})
	ok := true
	say := func(format string, a ...any) { fmt.Fprintf(w, format+"\n", a...) }

	say("1. sign in as the service principal (tenant %s, application %s)", tenant, client)
	cc := &ClientCredentials{ClientID: client, Secret: secret, HTTP: h, Scope: "https://management.azure.com/.default"}
	tok, err := cc.Token(ctx, tenant)
	if err != nil {
		say("   FAILED: %v", err)
		say("   Check the tenant id, the application id and that the secret is the VALUE (not the secret id) and has not expired.")
		return false
	}
	say("   ok")

	now := time.Now().UTC().Truncate(24 * time.Hour)
	from, to := now.AddDate(0, 0, -7), now
	type q struct {
		name, costType string
		group          []string
	}
	for i, x := range []q{
		{"2. billed cost, one total per day (what Verify runs)", "ActualCost", nil},
		{"3. billed cost by service and location (what the sync reads)", "ActualCost", []string{"ServiceName", "ResourceLocation"}},
		{"4. amortized cost by service and location (the effective cost)", "AmortizedCost", []string{"ServiceName", "ResourceLocation"}},
	} {
		say("%s", x.name)
		body := map[string]any{"type": x.costType, "timeframe": "Custom",
			"timePeriod": map[string]string{"from": from.Format(time.DateOnly) + "T00:00:00Z", "to": to.AddDate(0, 0, -1).Format(time.DateOnly) + "T23:59:59Z"},
			"dataset":    map[string]any{"granularity": "Daily", "aggregation": map[string]any{"totalCost": map[string]string{"name": "Cost", "function": "Sum"}}}}
		if len(x.group) > 0 {
			var g []map[string]string
			for _, d := range x.group {
				g = append(g, map[string]string{"type": "Dimension", "name": d})
			}
			body["dataset"].(map[string]any)["grouping"] = g
		}
		raw, _ := json.Marshal(body)
		target := armBase + "/subscriptions/" + url.PathEscape(subscription) + "/providers/Microsoft.CostManagement/query?api-version=" + costAPIVersion
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(raw))
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Content-Type", "application/json")
		resp, err := h.Do(req)
		if err != nil {
			say("   FAILED: %v", err)
			ok = false
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			ok = ok && i == 2 // the amortized view is optional
			msg := strings.TrimSpace(string(data))
			if len(msg) > 600 {
				msg = msg[:600] + "…"
			}
			say("   HTTP %d%s: %s", resp.StatusCode, retryHint(resp), msg)
			continue
		}
		var qr queryResponse
		if err := json.Unmarshal(data, &qr); err != nil {
			say("   HTTP 200 but unreadable: %v", err)
			ok = false
			continue
		}
		var cols []string
		for _, c := range qr.Properties.Columns {
			cols = append(cols, c.Name)
		}
		say("   HTTP 200: %d rows, columns [%s]", len(qr.Properties.Rows), strings.Join(cols, ", "))
		for n, r := range qr.Properties.Rows {
			if n == 3 {
				break
			}
			say("   %v", r)
		}
		if qr.Properties.NextLink != "" {
			say("   (more pages follow)")
		}
	}
	return ok
}

func retryHint(r *http.Response) string {
	if v := r.Header.Get("Retry-After"); v != "" {
		return " (Retry-After " + v + "s)"
	}
	return ""
}
