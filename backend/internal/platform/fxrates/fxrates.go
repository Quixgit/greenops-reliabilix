// Package fxrates reads the euro foreign-exchange reference rates the European Central Bank publishes every working day
// (free, no key). A rate is "units of the currency per 1 EUR"; two currencies are converted through the euro.
//
// The rates are daily reference rates, not transaction rates: good for reporting cost in one currency, not for
// accounting or invoicing. The ECB publishes about 30 currencies; any other currency is not converted (and is said to be
// left out), never guessed.
package fxrates

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Rate is the value of one euro in a currency on a day.
type Rate struct {
	Day      time.Time
	Currency string
	PerEUR   float64
}

type Client struct {
	URL  string // default: the full history archive
	HTTP *http.Client
}

func New() *Client {
	return &Client{URL: "https://www.ecb.europa.eu/stats/eurofxref/eurofxref-hist.zip",
		HTTP: &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

// Fetch downloads the archive and returns every rate on or after `since` (zero = all).
func (c *Client) Fetch(ctx context.Context, since time.Time) ([]Rate, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, errors.New("fxrates: request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fxrates: status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	return Parse(raw, since)
}

// Parse reads the archive (a zip holding one CSV) or the bare CSV.
func Parse(raw []byte, since time.Time) ([]Rate, error) {
	body := raw
	if zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw))); err == nil {
		if len(zr.File) == 0 {
			return nil, errors.New("fxrates: empty archive")
		}
		f, err := zr.File[0].Open()
		if err != nil {
			return nil, err
		}
		defer func() { _ = f.Close() }()
		if body, err = io.ReadAll(io.LimitReader(f, 32<<20)); err != nil {
			return nil, err
		}
	}
	r := csv.NewReader(bytes.NewReader(body))
	r.FieldsPerRecord = -1
	head, err := r.Read()
	if err != nil || len(head) < 3 || !strings.EqualFold(strings.TrimSpace(head[0]), "date") {
		return nil, errors.New("fxrates: unexpected header")
	}
	cur := make([]string, len(head))
	for i, h := range head {
		cur[i] = strings.ToUpper(strings.TrimSpace(h))
	}
	var out []Rate
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		day, err := time.Parse("2006-01-02", strings.TrimSpace(rec[0]))
		if err != nil || day.Before(since) {
			continue
		}
		for i := 1; i < len(rec) && i < len(cur); i++ {
			v := strings.TrimSpace(rec[i])
			if v == "" || v == "N/A" || len(cur[i]) != 3 {
				continue
			}
			f, err := strconv.ParseFloat(v, 64)
			if err != nil || f <= 0 {
				continue
			}
			out = append(out, Rate{Day: day, Currency: cur[i], PerEUR: f})
		}
	}
	if len(out) == 0 {
		return nil, errors.New("fxrates: no rates in the file")
	}
	return out, nil
}
