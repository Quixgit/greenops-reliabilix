// Package electricitymaps implements domain.CarbonDataProvider on the
// Electricity Maps HTTP API.
package electricitymaps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
)

// zones maps cloud regions to Electricity Maps zones (extend as regions are onboarded).
var zones = map[string]string{
	"us-east-1": "US-MIDA-PJM", "us-west-2": "US-NW-PACW",
	"eu-west-1": "IE", "eu-central-1": "DE", "eu-north-1": "SE",
}

type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func New(apiKey string) *Client {
	return &Client{BaseURL: "https://api.electricitymaps.com/v3", APIKey: apiKey, HTTP: &http.Client{Timeout: 10 * time.Second}}
}

// Regions lists the cloud regions this client can resolve to a zone.
func (*Client) Regions() []string { return sortedRegions() }

func sortedRegions() []string {
	out := make([]string, 0, len(zones))
	for r := range zones {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

func (c *Client) GetIntensity(ctx context.Context, region string) (domain.Intensity, error) {
	zone, ok := zones[region]
	if !ok {
		return domain.Intensity{}, fmt.Errorf("electricitymaps: no zone for region %q", region)
	}
	var body struct {
		CarbonIntensity float64   `json:"carbonIntensity"`
		Datetime        time.Time `json:"datetime"`
	}
	if err := c.get(ctx, "/carbon-intensity/latest?zone="+url.QueryEscape(zone), &body); err != nil {
		return domain.Intensity{}, err
	}
	return domain.Intensity{Region: region, GPerKWh: body.CarbonIntensity, At: body.Datetime}, nil
}

func (c *Client) GetForecast(ctx context.Context, region string) (domain.Forecast, error) {
	zone, ok := zones[region]
	if !ok {
		return domain.Forecast{}, fmt.Errorf("electricitymaps: no zone for region %q", region)
	}
	var body struct {
		Forecast []struct {
			CarbonIntensity float64   `json:"carbonIntensity"`
			Datetime        time.Time `json:"datetime"`
		} `json:"forecast"`
	}
	if err := c.get(ctx, "/carbon-intensity/forecast?zone="+url.QueryEscape(zone), &body); err != nil {
		return domain.Forecast{}, err
	}
	f := domain.Forecast{Region: region}
	for _, p := range body.Forecast {
		f.Points = append(f.Points, domain.Intensity{Region: region, GPerKWh: p.CarbonIntensity, At: p.Datetime, IsForecast: true})
	}
	return f, nil
}

func (c *Client) get(ctx context.Context, path string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("auth-token", c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("electricitymaps: status %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(dst)
}

// Regions lists the same regions as the real client (development only).
func (Static) Regions() []string { return sortedRegions() }

// Static is a fixed-intensity provider for local development only.
type Static struct{ GPerKWh float64 }

func (s Static) GetIntensity(_ context.Context, region string) (domain.Intensity, error) {
	return domain.Intensity{Region: region, GPerKWh: s.GPerKWh, At: time.Now()}, nil
}
func (s Static) GetForecast(context.Context, string) (domain.Forecast, error) {
	return domain.Forecast{}, nil
}
