// Package awsprice reads the on-demand EC2 price list that AWS publishes for its public pricing pages and turns it into
// a relative price index per region: how much more or less the same basket of instances costs there than in a baseline
// region. The platform uses the index to estimate the cost effect of moving a compute workload to another region.
//
// What the index is, and is not: it is a ratio of list prices (Linux, shared tenancy, on-demand, USD) for a fixed basket
// of general-purpose, compute and memory instances. It is NOT a quote: discounts, savings plans and reservations, data
// transfer and the cost of the migration itself are outside it. It applies to EC2 compute only.
package awsprice

import (
	"compress/gzip"
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

// Locations maps a region code to the location name used in the price list.
var Locations = map[string]string{
	"us-east-1": "US East (N. Virginia)", "us-east-2": "US East (Ohio)", "us-west-1": "US West (N. California)", "us-west-2": "US West (Oregon)",
	"ca-central-1": "Canada (Central)", "ca-west-1": "Canada West (Calgary)",
	"eu-west-1": "EU (Ireland)", "eu-west-2": "EU (London)", "eu-west-3": "EU (Paris)", "eu-central-1": "EU (Frankfurt)",
	"eu-central-2": "EU (Zurich)", "eu-north-1": "EU (Stockholm)", "eu-south-1": "EU (Milan)", "eu-south-2": "EU (Spain)",
	"ap-northeast-1": "Asia Pacific (Tokyo)", "ap-northeast-2": "Asia Pacific (Seoul)", "ap-northeast-3": "Asia Pacific (Osaka)",
	"ap-southeast-1": "Asia Pacific (Singapore)", "ap-southeast-2": "Asia Pacific (Sydney)", "ap-southeast-4": "Asia Pacific (Melbourne)",
	"ap-south-1": "Asia Pacific (Mumbai)", "ap-east-1": "Asia Pacific (Hong Kong)",
	"sa-east-1": "South America (Sao Paulo)", "il-central-1": "Israel (Tel Aviv)", "af-south-1": "Africa (Cape Town)", "me-south-1": "Middle East (Bahrain)",
}

// Baseline is the region whose basket price is 1.0.
const Baseline = "us-east-1"

// Basket is the fixed set of instance types the index is built from: current general-purpose, compute and memory sizes
// that exist in nearly every region. An instance missing in a region is left out for that pair, never guessed.
var Basket = []string{"m5.large", "m5.xlarge", "c5.large", "c5.xlarge", "r5.large", "r5.xlarge", "m6i.large", "c6i.large", "t3.medium"}

// minOverlap is how many basket instances a region must share with the baseline for its index to be trusted.
const minOverlap = 4

type Client struct {
	BaseURL string // default https://b0.p.awsstatic.com
	HTTP    *http.Client
	Allow   func(ctx context.Context) error
	Done    func(ctx context.Context, failed bool)
}

func New() *Client {
	return &Client{BaseURL: "https://b0.p.awsstatic.com",
		HTTP: &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

// Prices returns USD per hour by instance type for a region (Linux, shared tenancy, on-demand).
func (c *Client) Prices(ctx context.Context, region string) (map[string]float64, error) {
	loc, ok := Locations[region]
	if !ok {
		return nil, fmt.Errorf("awsprice: unknown region %q", region)
	}
	if c.Allow != nil {
		if err := c.Allow(ctx); err != nil {
			return nil, fmt.Errorf("awsprice: %w", err)
		}
	}
	var failed bool
	defer func() {
		if c.Done != nil {
			c.Done(ctx, failed)
		}
	}()
	u := c.BaseURL + "/pricing/2.0/meteredUnitMaps/ec2/USD/current/ec2-ondemand-without-sec-sel/" + url.PathEscape(loc) + "/Linux/index.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		failed = true
		return nil, err
	}
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		failed = true
		return nil, errors.New("awsprice: request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		failed = true
		return nil, fmt.Errorf("awsprice: status %d for %s", resp.StatusCode, region)
	}
	var body io.Reader = io.LimitReader(resp.Body, 64<<20)
	if resp.Header.Get("Content-Encoding") == "gzip" {
		zr, err := gzip.NewReader(body)
		if err != nil {
			failed = true
			return nil, errors.New("awsprice: bad gzip body")
		}
		defer func() { _ = zr.Close() }()
		body = zr
	}
	var doc struct {
		Regions map[string]map[string]struct {
			Price        string `json:"price"`
			InstanceType string `json:"Instance Type"`
			OS           string `json:"Operating System"`
			Software     string `json:"Pre Installed S/W"`
		} `json:"regions"`
	}
	if err := json.NewDecoder(body).Decode(&doc); err != nil {
		failed = true
		return nil, errors.New("awsprice: the price list could not be read")
	}
	out := map[string]float64{}
	for _, rows := range doc.Regions {
		for _, r := range rows {
			if r.InstanceType == "" || (r.OS != "" && r.OS != "Linux") || (r.Software != "" && r.Software != "NA") {
				continue
			}
			p, err := strconv.ParseFloat(strings.TrimSpace(r.Price), 64)
			if err != nil || p <= 0 || math.IsNaN(p) {
				continue
			}
			if old, ok := out[r.InstanceType]; !ok || p < old { // several rows per type are variants: the plain one is the lowest
				out[r.InstanceType] = p
			}
		}
	}
	if len(out) == 0 {
		failed = true
		return nil, fmt.Errorf("awsprice: no prices for %s", region)
	}
	return out, nil
}

// Ratio is the geometric mean of price(region)/price(baseline) over the basket instances both have, and how many that
// was. ok is false when too few instances overlap to trust the figure.
func Ratio(region, base map[string]float64) (ratio float64, n int, ok bool) {
	sum := 0.0
	for _, it := range Basket {
		a, b := region[it], base[it]
		if a > 0 && b > 0 {
			sum += math.Log(a / b)
			n++
		}
	}
	if n < minOverlap {
		return 0, n, false
	}
	return math.Exp(sum / float64(n)), n, true
}

// Index is a computed price index entry.
type Index struct {
	Region string
	Idx    float64
	N      int // basket instances the ratio rests on
}

// BuildIndex fetches the baseline and every wanted region and returns an index per region that could be computed. A region
// that fails is skipped (and reported in errs) so one bad page does not drop the others.
func (c *Client) BuildIndex(ctx context.Context, regions []string) (idx []Index, errs map[string]error, err error) {
	base, err := c.Prices(ctx, Baseline)
	if err != nil {
		return nil, nil, fmt.Errorf("baseline %s: %w", Baseline, err)
	}
	errs = map[string]error{}
	seen := map[string]bool{Baseline: true}
	idx = append(idx, Index{Region: Baseline, Idx: 1, N: len(Basket)})
	for _, r := range regions {
		if seen[r] {
			continue
		}
		seen[r] = true
		p, e := c.Prices(ctx, r)
		if e != nil {
			errs[r] = e
			continue
		}
		ratio, n, ok := Ratio(p, base)
		if !ok {
			errs[r] = fmt.Errorf("awsprice: only %d basket instances in common with %s", n, Baseline)
			continue
		}
		idx = append(idx, Index{Region: r, Idx: math.Round(ratio*10000) / 10000, N: n})
	}
	sort.Slice(idx, func(i, j int) bool { return idx[i].Region < idx[j].Region })
	return idx, errs, nil
}
