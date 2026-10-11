// Package gcpprice reads Google's Cloud Billing Catalog API (needs a free API key: there is no keyless public price list) and
// turns the on-demand Compute Engine N2 core and RAM prices into a relative index per region, the same shape as the AWS and Azure
// indexes. N2 standard-4 (4 vCPU, 16 GB) stands for the basket: price = 4 x core + 16 x RAM per hour.
//
// It is a ratio of list prices (on-demand, no committed use, no Spot, USD). NOT a quote. Compute only.
package gcpprice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Baseline is the region whose price is 1.0.
const Baseline = "us-central1"

// computeService is the Cloud Billing id of the Compute Engine service.
const computeService = "6F81-5844-456A"

type Client struct {
	BaseURL string // default https://cloudbilling.googleapis.com
	Key     string
	HTTP    *http.Client
	Allow   func(ctx context.Context) error
	Done    func(ctx context.Context, failed bool)
}

func New(key string) *Client {
	return &Client{BaseURL: "https://cloudbilling.googleapis.com", Key: key,
		HTTP: &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

var errNoKey = errors.New("gcpprice: no API key is configured")

type sku struct {
	Description string `json:"description"`
	Category    struct {
		Family string `json:"resourceFamily"`
		Usage  string `json:"usageType"`
	} `json:"category"`
	Regions []string `json:"serviceRegions"`
	Pricing []struct {
		Expr struct {
			Tiers []struct {
				Unit struct {
					Units string `json:"units"`
					Nanos int64  `json:"nanos"`
				} `json:"unitPrice"`
			} `json:"tieredRates"`
		} `json:"pricingExpression"`
	} `json:"pricingInfo"`
}

func (s sku) price() float64 {
	if len(s.Pricing) == 0 || len(s.Pricing[0].Expr.Tiers) == 0 {
		return 0
	}
	u := s.Pricing[0].Expr.Tiers[len(s.Pricing[0].Expr.Tiers)-1].Unit
	units, _ := strconv.ParseFloat(u.Units, 64)
	return units + float64(u.Nanos)/1e9
}

// Prices returns the N2 standard-4 hourly price per region.
func (c *Client) Prices(ctx context.Context) (map[string]float64, error) {
	if c.Key == "" {
		return nil, errNoKey
	}
	core, ram := map[string]float64{}, map[string]float64{}
	token := ""
	for page := 0; page < 60; page++ {
		skus, next, err := c.page(ctx, token)
		if err != nil {
			return nil, err
		}
		for _, s := range skus {
			if s.Category.Family != "Compute" || s.Category.Usage != "OnDemand" {
				continue
			}
			d := s.Description
			if !strings.HasPrefix(d, "N2 Instance ") || strings.Contains(d, "Custom") || strings.Contains(d, "Sole Tenancy") || strings.Contains(d, "Spot") || strings.Contains(d, "Preemptible") {
				continue
			}
			p := s.price()
			if p <= 0 || math.IsNaN(p) {
				continue
			}
			target := core
			if strings.HasPrefix(d, "N2 Instance Ram") {
				target = ram
			} else if !strings.HasPrefix(d, "N2 Instance Core") {
				continue
			}
			for _, r := range s.Regions {
				if old, ok := target[r]; !ok || p < old {
					target[r] = p
				}
			}
		}
		if next == "" {
			break
		}
		token = next
	}
	out := map[string]float64{}
	for r, cp := range core {
		if rp, ok := ram[r]; ok {
			out[r] = 4*cp + 16*rp
		}
	}
	if len(out) == 0 {
		return nil, errors.New("gcpprice: no N2 prices were returned")
	}
	return out, nil
}

func (c *Client) page(ctx context.Context, token string) (skus []sku, next string, err error) {
	if c.Allow != nil {
		if err := c.Allow(ctx); err != nil {
			return nil, "", fmt.Errorf("gcpprice: %w", err)
		}
	}
	defer func() {
		if c.Done != nil {
			c.Done(ctx, err != nil)
		}
	}()
	q := url.Values{"key": {c.Key}, "pageSize": {"5000"}, "currencyCode": {"USD"}}
	if token != "" {
		q.Set("pageToken", token)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/v1/services/"+computeService+"/skus?"+q.Encode(), nil)
	if err != nil {
		return nil, "", errors.New("gcpprice: bad address")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, "", errors.New("gcpprice: request failed") // never the URL: it carries the key
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusForbidden, http.StatusBadRequest, http.StatusUnauthorized:
		return nil, "", errors.New("gcpprice: the key was not accepted")
	default:
		return nil, "", fmt.Errorf("gcpprice: status %d", resp.StatusCode)
	}
	var doc struct {
		SKUs []sku  `json:"skus"`
		Next string `json:"nextPageToken"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&doc); err != nil {
		return nil, "", errors.New("gcpprice: the price list could not be read")
	}
	return doc.SKUs, doc.Next, nil
}

// Check proves the key works with the smallest request.
func (c *Client) Check(ctx context.Context) error {
	if c.Key == "" {
		return errNoKey
	}
	q := url.Values{"key": {c.Key}, "pageSize": {"1"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/v1/services/"+computeService+"/skus?"+q.Encode(), nil)
	if err != nil {
		return errors.New("gcpprice: bad address")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return errors.New("gcpprice: request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return errors.New("gcpprice: the key was not accepted")
	}
	return nil
}

// Index is a computed price index entry.
type Index struct {
	Region string
	Idx    float64
	N      int
}

// BuildIndex returns the index of the wanted regions against the baseline (one catalog read serves every region).
func (c *Client) BuildIndex(ctx context.Context, regions []string) (idx []Index, errs map[string]error, err error) {
	prices, err := c.Prices(ctx)
	if err != nil {
		return nil, nil, err
	}
	base, ok := prices[Baseline]
	if !ok || base <= 0 {
		return nil, nil, fmt.Errorf("gcpprice: no baseline price for %s", Baseline)
	}
	errs = map[string]error{}
	for _, r := range regions {
		p, ok := prices[r]
		if !ok {
			errs[r] = fmt.Errorf("gcpprice: no price for %s", r)
			continue
		}
		idx = append(idx, Index{Region: r, Idx: math.Round(p/base*10000) / 10000, N: 2})
	}
	sort.Slice(idx, func(i, j int) bool { return idx[i].Region < idx[j].Region })
	return idx, errs, nil
}
