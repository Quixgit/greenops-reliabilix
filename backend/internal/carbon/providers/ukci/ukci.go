// Package ukci reads Great Britain's grid carbon intensity from the UK National Grid ESO "Carbon Intensity API": free,
// no key, half-hourly, actual and forecast values in gCO2/kWh. Only measured ("actual") values are used; a forecast is
// never stored as a reading. It covers Great Britain only.
package ukci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

// Reading is one half-hour of measured intensity.
type Reading struct {
	At time.Time // start of the half hour, UTC
	G  float64
}

type Client struct {
	BaseURL string // default https://api.carbonintensity.org.uk
	HTTP    *http.Client
	// Allow and Done meter and cap requests; both optional.
	Allow func(ctx context.Context) error
	Done  func(ctx context.Context, failed bool)
}

func New() *Client {
	return &Client{BaseURL: "https://api.carbonintensity.org.uk",
		HTTP: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

// maxRange is what one request may cover (the API allows 14 days; stay under it).
const maxRange = 13 * 24 * time.Hour

const (
	minIntensity = 1.0
	maxIntensity = 2000.0
)

// Range returns the measured half-hours in [from, to), requested in chunks. Half-hours without an actual value (the
// most recent ones) are skipped, and so are implausible values.
func (c *Client) Range(ctx context.Context, from, to time.Time) ([]Reading, error) {
	var out []Reading
	for start := from.UTC(); start.Before(to); start = start.Add(maxRange) {
		end := start.Add(maxRange)
		if end.After(to) {
			end = to
		}
		var body struct {
			Data []struct {
				From      string `json:"from"`
				Intensity struct {
					Actual *float64 `json:"actual"`
				} `json:"intensity"`
			} `json:"data"`
		}
		path := fmt.Sprintf("/intensity/%s/%s", start.Format("2006-01-02T15:04Z"), end.Format("2006-01-02T15:04Z"))
		if err := c.get(ctx, path, &body); err != nil {
			return nil, err
		}
		for _, d := range body.Data {
			t, err := time.Parse("2006-01-02T15:04Z", d.From)
			if err != nil || d.Intensity.Actual == nil {
				continue
			}
			g := *d.Intensity.Actual
			if math.IsNaN(g) || g < minIntensity || g > maxIntensity || t.Before(from) || !t.Before(to) {
				continue
			}
			out = append(out, Reading{At: t.UTC(), G: g})
		}
	}
	return out, nil
}

func (c *Client) get(ctx context.Context, path string, dst any) (err error) {
	if c.Allow != nil {
		if err := c.Allow(ctx); err != nil {
			return fmt.Errorf("uk carbon intensity: %w", err)
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
		return errors.New("uk carbon intensity: request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("uk carbon intensity: status %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(dst)
}
