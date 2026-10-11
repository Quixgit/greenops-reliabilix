package awsprice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/pricing"
)

// The managed services are read from AWS's official bulk price list (https://pricing.us-east-1.amazonaws.com, public, no key): one
// file per service and region. The EC2 index uses the lighter pricing-page maps; the bulk files are the only public source for RDS
// and ElastiCache. A file is large (RDS about 27 MB per region), so it is streamed and only the basket is kept.

// Service is one billed service the bulk index describes.
type Service struct {
	ID     string // the provider column of the stored index: "aws:rds", "aws:elasticache"
	Offer  string // the offer code in the bulk list
	Basket []string
	// Match decides whether a product row is the plain on-demand node the basket names, and returns its instance type.
	Match func(family string, attrs map[string]string) (instanceType string, ok bool)
	// MinOverlap is how many basket entries a region must share with the baseline.
	MinOverlap int
}

var (
	nodeUsage     = regexp.MustCompile(`^([A-Z0-9]+-)?NodeUsage:(cache\.[a-z0-9]+\.[a-z0-9]+)$`)
	instanceUsage = regexp.MustCompile(`^([A-Z0-9]+-)?InstanceUsage:(db\.[a-z0-9]+\.[a-z0-9]+)$`)
)

// RDS is Amazon RDS for PostgreSQL, single-AZ, license included in the price (no license model), on-demand. Other engines follow the
// same regional price ratio closely, so this basket stands for the service.
var RDS = Service{ID: "aws:rds", Offer: "AmazonRDS", MinOverlap: 4,
	Basket: []string{"db.t3.medium", "db.t3.large", "db.m5.large", "db.m5.xlarge", "db.r5.large", "db.r5.xlarge", "db.m6g.large"},
	Match: func(family string, a map[string]string) (string, bool) {
		if family != "Database Instance" || a["databaseEngine"] != "PostgreSQL" || a["deploymentOption"] != "Single-AZ" || a["licenseModel"] != "No license required" {
			return "", false
		}
		m := instanceUsage.FindStringSubmatch(a["usagetype"])
		if m == nil || (a["locationType"] != "" && a["locationType"] != "AWS Region") {
			return "", false
		}
		return m[2], true
	}}

// ElastiCache is Amazon ElastiCache for Redis (OSS engine), on-demand nodes in a region (not Outposts, not extended support).
var ElastiCache = Service{ID: "aws:elasticache", Offer: "AmazonElastiCache", MinOverlap: 4,
	Basket: []string{"cache.t3.small", "cache.t3.medium", "cache.m5.large", "cache.m5.xlarge", "cache.r5.large", "cache.r5.xlarge", "cache.m6g.large"},
	Match: func(family string, a map[string]string) (string, bool) {
		if family != "Cache Instance" || a["cacheEngine"] != "Redis" || (a["locationType"] != "" && a["locationType"] != "AWS Region") {
			return "", false
		}
		m := nodeUsage.FindStringSubmatch(a["usagetype"])
		if m == nil {
			return "", false
		}
		return m[2], true
	}}

// Services are every managed service an index is built for.
var Services = []Service{RDS, ElastiCache}

// BulkBase is the host of the bulk price list.
const BulkBase = "https://pricing.us-east-1.amazonaws.com"

// Bulk reads the bulk price list.
type Bulk struct {
	BaseURL string
	HTTP    *http.Client
	Allow   func(ctx context.Context) error
	Done    func(ctx context.Context, failed bool)
	// Parallel is how many regions are read at once (default 3): a region of RDS is a large file.
	Parallel int
}

func NewBulk() *Bulk {
	return &Bulk{BaseURL: BulkBase, Parallel: 3,
		HTTP: &http.Client{Timeout: 5 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

// Prices returns USD per hour by instance type for one region of a service.
func (b *Bulk) Prices(ctx context.Context, svc Service, region string) (map[string]float64, error) {
	if !validRegion(region) {
		return nil, fmt.Errorf("awsprice: bad region %q", region)
	}
	if b.Allow != nil {
		if err := b.Allow(ctx); err != nil {
			return nil, fmt.Errorf("awsprice: %w", err)
		}
	}
	var failed bool
	defer func() {
		if b.Done != nil {
			b.Done(ctx, failed)
		}
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/offers/v1.0/aws/%s/current/%s/index.json", b.BaseURL, svc.Offer, region), nil)
	if err != nil {
		failed = true
		return nil, err
	}
	resp, err := b.HTTP.Do(req)
	if err != nil {
		failed = true
		return nil, errors.New("awsprice: request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("awsprice: no %s price list for %s: %w", svc.Offer, region, pricing.ErrNotOffered)
	}
	if resp.StatusCode != http.StatusOK {
		failed = true
		return nil, fmt.Errorf("awsprice: status %d for %s %s", resp.StatusCode, svc.Offer, region)
	}
	out, err := streamPrices(resp.Body, svc)
	if err != nil {
		failed = true
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("awsprice: no %s prices for %s: %w", svc.Offer, region, pricing.ErrNotOffered)
	}
	return out, nil
}

func validRegion(r string) bool {
	if len(r) < 6 || len(r) > 20 {
		return false
	}
	for _, c := range r {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

// streamPrices walks the file once: "products" first (keep the SKUs of the basket), then "terms.OnDemand" (their hourly USD price).
// Memory stays at the size of the basket, not of the file.
func streamPrices(r io.Reader, svc Service) (map[string]float64, error) {
	dec := json.NewDecoder(io.LimitReader(r, 512<<20))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, errors.New("awsprice: the price list could not be read")
	}
	inBasket := map[string]bool{}
	for _, it := range svc.Basket {
		inBasket[it] = true
	}
	skuType := map[string]string{}
	out := map[string]float64{}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, errors.New("awsprice: the price list could not be read")
		}
		switch keyTok {
		case "products":
			if err := walkObject(dec, func(sku string) error {
				var p struct {
					Family string            `json:"productFamily"`
					Attrs  map[string]string `json:"attributes"`
				}
				if err := dec.Decode(&p); err != nil {
					return err
				}
				if it, ok := svc.Match(p.Family, p.Attrs); ok && inBasket[it] {
					skuType[sku] = it
				}
				return nil
			}); err != nil {
				return nil, errors.New("awsprice: the products could not be read")
			}
		case "terms":
			if err := walkObject(dec, func(kind string) error {
				if kind != "OnDemand" {
					return skipValue(dec)
				}
				return walkObject(dec, func(sku string) error {
					it, want := skuType[sku]
					if !want {
						return skipValue(dec)
					}
					var offers map[string]struct {
						Dims map[string]struct {
							Unit  string            `json:"unit"`
							Price map[string]string `json:"pricePerUnit"`
						} `json:"priceDimensions"`
					}
					if err := dec.Decode(&offers); err != nil {
						return err
					}
					for _, o := range offers {
						for _, d := range o.Dims {
							if d.Unit != "Hrs" {
								continue
							}
							p, err := strconv.ParseFloat(strings.TrimSpace(d.Price["USD"]), 64)
							if err != nil || p <= 0 || math.IsNaN(p) {
								continue
							}
							if old, ok := out[it]; !ok || p < old {
								out[it] = p
							}
						}
					}
					return nil
				})
			}); err != nil {
				return nil, errors.New("awsprice: the terms could not be read")
			}
		default:
			if err := skipValue(dec); err != nil {
				return nil, errors.New("awsprice: the price list could not be read")
			}
		}
	}
	return out, nil
}

// walkObject reads the next value as an object and calls fn for each key (fn must consume the value).
func walkObject(dec *json.Decoder, fn func(key string) error) error {
	t, err := dec.Token()
	if err != nil {
		return err
	}
	if t != json.Delim('{') {
		return errors.New("not an object")
	}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return err
		}
		k, _ := kt.(string)
		if err := fn(k); err != nil {
			return err
		}
	}
	_, err = dec.Token() // the closing brace
	return err
}

func skipValue(dec *json.Decoder) error {
	var skip json.RawMessage
	return dec.Decode(&skip)
}

// ServiceRatio is the geometric mean of price(region)/price(baseline) over the basket both have.
func ServiceRatio(svc Service, region, base map[string]float64) (ratio float64, n int, ok bool) {
	sum := 0.0
	for _, it := range svc.Basket {
		a, b := region[it], base[it]
		if a > 0 && b > 0 {
			sum += math.Log(a / b)
			n++
		}
	}
	if n < max(1, svc.MinOverlap) {
		return 0, n, false
	}
	return math.Exp(sum / float64(n)), n, true
}

// BuildServiceIndex fetches the baseline and every wanted region of one managed service, a few at a time. A region that fails or that
// does not offer the service is reported in errs and left out; it has no index and no estimate.
func (b *Bulk) BuildServiceIndex(ctx context.Context, svc Service, regions []string) (idx []Index, errs map[string]error, err error) {
	base, err := b.Prices(ctx, svc, Baseline)
	if err != nil {
		return nil, nil, fmt.Errorf("baseline %s: %w", Baseline, err)
	}
	errs = map[string]error{}
	idx = []Index{{Region: Baseline, Idx: 1, N: len(base)}}
	par := b.Parallel
	if par < 1 {
		par = 1
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, par)
	for _, r := range regions {
		if r == Baseline {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			p, e := b.Prices(ctx, svc, r)
			var ix *Index
			if e == nil {
				if ratio, n, ok := ServiceRatio(svc, p, base); ok {
					ix = &Index{Region: r, Idx: math.Round(ratio*10000) / 10000, N: n}
				} else {
					e = fmt.Errorf("awsprice: only %d basket entries overlap for %s: %w", n, r, pricing.ErrNotOffered)
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if ix != nil {
				idx = append(idx, *ix)
			} else {
				errs[r] = e
			}
		}()
	}
	wg.Wait()
	sort.Slice(idx, func(i, j int) bool { return idx[i].Region < idx[j].Region })
	return idx, errs, nil
}
