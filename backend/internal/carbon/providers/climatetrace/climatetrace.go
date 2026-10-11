// Package climatetrace reads a country's annual power-sector emissions from the Climate TRACE public API
// (https://api.climatetrace.org, v6; free, no key). It is an emissions INVENTORY (tonnes of CO2e by sector and country), not a grid
// intensity: it has no generation. Divided by a country's annual generation (Ember) it gives an independent, direct-emission
// intensity to hold hourly sources against. It is never used for a calculation (ADR-0046).
package climatetrace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	BaseURL string // default https://api.climatetrace.org
	HTTP    *http.Client
	Allow   func(ctx context.Context) error
	Done    func(ctx context.Context, failed bool)
}

func New() *Client {
	return &Client{BaseURL: "https://api.climatetrace.org",
		HTTP: &http.Client{Timeout: 40 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

// PowerTonnes returns, per ISO 3166 alpha-3 country code, the power sector's emissions of the year in tonnes of CO2e (100-year
// warming potentials). A country without a figure is absent from the result.
func (c *Client) PowerTonnes(ctx context.Context, countries []string, year int) (map[string]float64, error) {
	out := map[string]float64{}
	if len(countries) == 0 {
		return out, nil
	}
	var body []struct {
		Country   string `json:"country"`
		Emissions struct {
			CO2e100 float64 `json:"co2e_100yr"`
		} `json:"emissions"`
	}
	q := url.Values{"since": {fmt.Sprint(year)}, "to": {fmt.Sprint(year)}, "countries": {strings.Join(countries, ",")}, "sectors": {"power"}}
	if err := c.get(ctx, "/v6/country/emissions?"+q.Encode(), &body); err != nil {
		return nil, err
	}
	for _, r := range body {
		if r.Emissions.CO2e100 > 0 {
			out[strings.ToUpper(r.Country)] = r.Emissions.CO2e100
		}
	}
	return out, nil
}

// Check proves the API can be reached with the smallest request.
func (c *Client) Check(ctx context.Context) error {
	got, err := c.PowerTonnes(ctx, []string{"DEU"}, time.Now().UTC().Year()-2)
	if err != nil {
		return err
	}
	if len(got) == 0 {
		return errors.New("climate trace: no data was returned")
	}
	return nil
}

func (c *Client) get(ctx context.Context, path string, dst any) (err error) {
	if c.Allow != nil {
		if err := c.Allow(ctx); err != nil {
			return fmt.Errorf("climate trace: %w", err)
		}
	}
	defer func() {
		if c.Done != nil {
			c.Done(ctx, err != nil)
		}
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("climate trace: request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("climate trace: status %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(dst)
}
