// Package azureprice reads Microsoft's public Azure Retail Prices API (https://prices.azure.com, free, no key) and turns the
// on-demand Linux virtual machine prices into a relative index per region: how much more or less the same basket of VM sizes
// costs there than in a baseline region. The platform uses it to estimate the cost effect of moving a VM workload.
//
// What the index is, and is not: a ratio of list prices (Linux, pay-as-you-go, USD, no Spot or low-priority) for a fixed
// basket. It is NOT a quote: reservations, savings plans, hybrid benefit, data transfer and the migration itself are outside it.
// It applies to virtual machine compute only.
package azureprice

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

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/pricing"
)

// Baseline is the region whose basket price is 1.0.
const Baseline = "eastus"

// Basket is the fixed set of sizes the index is built from: general purpose, compute and memory sizes that exist in nearly every
// region. A size missing in a region is left out for that pair, never guessed.
var Basket = []string{"Standard_D2s_v5", "Standard_D4s_v5", "Standard_E2s_v5", "Standard_E4s_v5", "Standard_F2s_v2", "Standard_F4s_v2", "Standard_D2as_v5", "Standard_B2s"}

// Regions are the Azure regions the index is built for (the ids the platform knows).
var Regions = []string{
	"eastus", "eastus2", "centralus", "northcentralus", "southcentralus", "westus", "westus2", "westus3", "westcentralus",
	"canadacentral", "canadaeast", "brazilsouth", "northeurope", "westeurope", "uksouth", "ukwest", "francecentral", "germanywestcentral",
	"switzerlandnorth", "norwayeast", "swedencentral", "polandcentral", "italynorth", "spaincentral", "japaneast", "japanwest",
	"koreacentral", "southeastasia", "eastasia", "australiaeast", "australiasoutheast", "centralindia", "southindia", "uaenorth",
	"southafricanorth", "qatarcentral", "israelcentral", "newzealandnorth",
}

const minOverlap = 4

// item is one retail price row.
type item struct {
	Price   float64 `json:"retailPrice"`
	Region  string  `json:"armRegionName"`
	SKU     string  `json:"armSkuName"`
	SKUName string  `json:"skuName"`
	Product string  `json:"productName"`
	Meter   string  `json:"meterName"`
	Type    string  `json:"type"`
}

// Family is one kind of billed service the index describes: which retail rows make up its basket and how each becomes one number.
// The id doubles as the provider column of the stored index ("azure" is virtual machines, for compatibility).
type Family struct {
	ID         string
	Service    string // serviceName in the retail catalog
	Basket     []string
	MinOverlap int
	// Key turns a row into its basket key and a comparable hourly price, or ok=false to leave it out.
	Key func(it item) (key string, price float64, ok bool)
	// Filter narrows the request beyond the service name (an OData clause, optional).
	Filter string
	// Generic families have no curated basket: every consumption meter the service has in both regions is compared (the geometric mean of
	// the price ratios), which is a rougher figure than a basket of sizes and is labelled so (ADR-0059).
	Generic bool
}

// GenericServices are the Azure services an index is built for from every meter they have in both regions. Services whose price does not
// depend on the region (Azure DevOps has no regional price at all) are not listed and stay "not estimated"; storage, with a meter for every
// redundancy and tier, is left out because an unweighted mean over them says little.
var GenericServices = []string{"Azure App Service", "Azure Container Apps", "Functions", "Azure Cognitive Search", "Azure Cosmos DB", "Foundry Models", "Azure Kubernetes Service", "Azure Monitor"}

// GenericPrefix starts the id of a generic family ("azure:svc:azure_app_service").
const GenericPrefix = "azure:svc:"

// Slug turns a service name into the part of a family id: lower case letters and digits, anything else one underscore.
func Slug(service string) string {
	var b strings.Builder
	us := false
	for _, r := range strings.ToLower(strings.TrimSpace(service)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			us = false
		} else if !us && b.Len() > 0 {
			b.WriteByte('_')
			us = true
		}
	}
	return strings.TrimRight(b.String(), "_")
}

// GenericFamily is the family of one service compared meter by meter.
func GenericFamily(service string) Family {
	return Family{ID: GenericPrefix + Slug(service), Service: service, MinOverlap: 5, Generic: true,
		Key: func(it item) (string, float64, bool) {
			if strings.Contains(it.SKUName, "Spot") || strings.Contains(it.SKUName, "Low Priority") || strings.Contains(it.Meter, "Spot") {
				return "", 0, false
			}
			return it.Product + "|" + it.SKUName + "|" + it.Meter, it.Price, true
		}}
}

// GenericFamilies is one family per generic service.
func GenericFamilies() []Family {
	out := make([]Family, 0, len(GenericServices))
	for _, s := range GenericServices {
		out = append(out, GenericFamily(s))
	}
	return out
}

// GenericIDs are the family ids of GenericFamilies, for the code that reads the stored indexes.
func GenericIDs() []string {
	out := make([]string, 0, len(GenericServices))
	for _, s := range GenericServices {
		out = append(out, GenericPrefix+Slug(s))
	}
	return out
}

// VM is Linux pay-as-you-go virtual machines: the original index.
var VM = Family{ID: "azure", Service: "Virtual Machines", Basket: Basket, MinOverlap: minOverlap,
	Filter: "(" + orSkus(Basket) + ")",
	Key: func(it item) (string, float64, bool) {
		if strings.Contains(it.Product, "Windows") || strings.Contains(it.SKUName, "Spot") || strings.Contains(it.SKUName, "Low Priority") || strings.Contains(it.Meter, "Spot") {
			return "", 0, false
		}
		return it.SKU, it.Price, true
	}}

// PostgreSQL is Azure Database for PostgreSQL Flexible Server compute, per vCore hour, over a few general-purpose and
// memory-optimized series. Storage, backup and IOPS are not part of it.
var PostgreSQL = Family{ID: "azure:postgresql", Service: "Azure Database for PostgreSQL", MinOverlap: 2,
	Basket: []string{
		"Azure Database for PostgreSQL Flexible Server General Purpose Ddsv4 Series Compute",
		"Azure Database for PostgreSQL Flexible Server General Purpose Ddsv5 Series Compute",
		"Azure Database for PostgreSQL Flexible Server Memory Optimized Edsv4 Series Compute",
		"Azure Database for PostgreSQL Flexible Server Memory Optimized Edsv5 Series Compute",
	},
	Key: func(it item) (string, float64, bool) {
		if !strings.Contains(it.Product, "Flexible Server") || !strings.HasSuffix(it.Product, "Compute") || !strings.Contains(it.SKUName, "vCore") {
			return "", 0, false
		}
		n := 1.0 // "vCore" and "1 vCore" are one core; "32 vCore" is the price of 32
		if f := strings.Fields(it.SKUName); len(f) == 2 {
			if v, err := strconv.ParseFloat(f[0], 64); err == nil && v > 0 {
				n = v
			}
		}
		return it.Product, it.Price / n, true
	}}

// Redis is Azure Cache for Redis (Basic, Standard and Premium tiers), per cache hour, over the commonly used sizes.
var Redis = Family{ID: "azure:redis", Service: "Redis Cache", MinOverlap: 3,
	Basket: []string{"Azure Redis Cache Basic/C1", "Azure Redis Cache Standard/C1", "Azure Redis Cache Standard/C2", "Azure Redis Cache Standard/C3", "Azure Redis Cache Premium/P1"},
	Key: func(it item) (string, float64, bool) {
		if !strings.HasPrefix(it.Product, "Azure Redis Cache ") || strings.Contains(it.Product, "Enterprise") {
			return "", 0, false
		}
		return it.Product + "/" + it.SKUName, it.Price, true
	}}

// Families are every service an index is built for.
var Families = []Family{VM, PostgreSQL, Redis}

func orSkus(sizes []string) string {
	parts := make([]string, len(sizes))
	for i, s := range sizes {
		parts[i] = "armSkuName eq '" + s + "'"
	}
	return strings.Join(parts, " or ")
}

type Client struct {
	BaseURL string // default https://prices.azure.com
	HTTP    *http.Client
	Allow   func(ctx context.Context) error
	Done    func(ctx context.Context, failed bool)
}

func New() *Client {
	return &Client{BaseURL: "https://prices.azure.com",
		HTTP: &http.Client{Timeout: 45 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

// Prices returns USD per hour by size for a region (Linux, pay-as-you-go).
func (c *Client) Prices(ctx context.Context, region string) (map[string]float64, error) {
	return c.FamilyPrices(ctx, VM, region)
}

// FamilyPrices returns the comparable hourly price per basket key of one service family in a region.
func (c *Client) FamilyPrices(ctx context.Context, fam Family, region string) (map[string]float64, error) {
	if strings.ContainsAny(region, "'\"") {
		return nil, fmt.Errorf("azureprice: bad region %q", region)
	}
	if c.Allow != nil {
		if err := c.Allow(ctx); err != nil {
			return nil, fmt.Errorf("azureprice: %w", err)
		}
	}
	var failed bool
	defer func() {
		if c.Done != nil {
			c.Done(ctx, failed)
		}
	}()
	filter := "serviceName eq '" + fam.Service + "' and armRegionName eq '" + region + "' and priceType eq 'Consumption'"
	if fam.Filter != "" {
		filter += " and " + fam.Filter
	}
	inBasket := map[string]bool{}
	for _, k := range fam.Basket {
		inBasket[k] = true
	}
	next := c.BaseURL + "/api/retail/prices?$filter=" + url.QueryEscape(filter)
	out := map[string]float64{}
	for page := 0; next != "" && page < 8; page++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, next, nil)
		if err != nil {
			failed = true
			return nil, err
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			failed = true
			return nil, errors.New("azureprice: request failed")
		}
		var doc struct {
			Items []item `json:"Items"`
			Next  string `json:"NextPageLink"`
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			failed = true
			return nil, fmt.Errorf("azureprice: status %d for %s", resp.StatusCode, region)
		}
		err = json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&doc)
		_ = resp.Body.Close()
		if err != nil {
			failed = true
			return nil, errors.New("azureprice: the price list could not be read")
		}
		for _, it := range doc.Items {
			if it.Price <= 0 || math.IsNaN(it.Price) || it.Type != "Consumption" || !strings.EqualFold(it.Region, region) {
				continue
			}
			key, price, ok := fam.Key(it)
			if !ok || price <= 0 || (!fam.Generic && !inBasket[key]) {
				continue
			}
			if old, have := out[key]; !have || price < old { // several rows are variants: the plain one is the lowest
				out[key] = price
			}
		}
		next = doc.Next
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("azureprice: no prices for %s: %w", region, pricing.ErrNotOffered)
	}
	return out, nil
}

// Ratio is the geometric mean of price(region)/price(baseline) over the basket sizes both have, and how many that was.
func Ratio(region, base map[string]float64) (ratio float64, n int, ok bool) {
	return familyRatio(VM, region, base)
}

func familyRatio(fam Family, region, base map[string]float64) (ratio float64, n int, ok bool) {
	sum := 0.0
	keys := fam.Basket
	if fam.Generic {
		keys = keys[:0:0]
		for k := range base {
			keys = append(keys, k)
		}
	}
	for _, it := range keys {
		a, b := region[it], base[it]
		if a > 0 && b > 0 {
			sum += math.Log(a / b)
			n++
		}
	}
	if n < max(1, fam.MinOverlap) {
		return 0, n, false
	}
	return math.Exp(sum / float64(n)), n, true
}

// Index is a computed price index entry.
type Index struct {
	Region string
	Idx    float64
	N      int
}

// BuildIndex is the virtual machine index.
func (c *Client) BuildIndex(ctx context.Context, regions []string) (idx []Index, errs map[string]error, err error) {
	return c.BuildFamilyIndex(ctx, VM, regions)
}

// BuildFamilyIndex fetches the baseline and every wanted region of one service family. A region that fails, or has too few
// basket entries in common with the baseline, is skipped (and reported in errs): a service that is not offered there has no index.
func (c *Client) BuildFamilyIndex(ctx context.Context, fam Family, regions []string) (idx []Index, errs map[string]error, err error) {
	base, err := c.FamilyPrices(ctx, fam, Baseline)
	if err != nil {
		return nil, nil, fmt.Errorf("baseline %s: %w", Baseline, err)
	}
	errs = map[string]error{}
	idx = append(idx, Index{Region: Baseline, Idx: 1, N: len(base)})
	for _, r := range regions {
		if r == Baseline {
			continue
		}
		p, e := c.FamilyPrices(ctx, fam, r)
		if e != nil {
			errs[r] = e
			continue
		}
		ratio, n, ok := familyRatio(fam, p, base)
		if !ok {
			errs[r] = fmt.Errorf("azureprice: only %d basket entries overlap for %s: %w", n, r, pricing.ErrNotOffered)
			continue
		}
		idx = append(idx, Index{Region: r, Idx: math.Round(ratio*10000) / 10000, N: n})
	}
	sort.Slice(idx, func(i, j int) bool { return idx[i].Region < idx[j].Region })
	return idx, errs, nil
}
