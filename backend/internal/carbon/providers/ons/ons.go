// Package ons computes the carbon intensity of a Brazilian subsystem of the national grid (SIN) from the hourly energy
// balance that the Operador Nacional do Sistema Elétrico publishes as open data (https://dados.ons.org.br, CC BY, no key):
// generation by hydro, thermal, wind and solar for the subsystems North, Northeast, South and Southeast/Centre-West.
//
// The figure is CALCULATED, not measured: intensity = sum(generation x emission factor) / sum(generation). Hydro, wind and
// solar use generic life-cycle medians (IPCC AR5); the dataset does not split thermal generation by fuel, so one factor
// stands for Brazil's thermal mix (gas, biomass, coal, nuclear, oil: an assumption, see ThermalFactor). Distributed
// generation and imports are not accounted for. It is labelled as computed and ranks behind a measured source.
package ons

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Generic life-cycle factors, gCO2e per kWh. ThermalFactor is an assumption for Brazil's thermal fleet (about 45% gas, 25%
// biomass, 10% nuclear, 10% oil, 8% coal by generation, each at its generic median), not a published figure.
const (
	HydroFactor   = 24.0
	WindFactor    = 11.0
	SolarFactor   = 48.0
	ThermalFactor = 420.0
)

// Subsystems maps the platform's grid zone keys (Electricity Maps naming) to the ONS subsystem ids.
var Subsystems = map[string]string{"BR-CS": "SE", "BR-S": "S", "BR-NE": "NE", "BR-N": "N"}

// Reading is one hour of computed intensity.
type Reading struct {
	At time.Time // start of the hour, UTC
	G  float64
}

type Client struct {
	// BaseURL is the dataset folder; the yearly files BALANCO_ENERGIA_SUBSISTEMA_<year>.csv are under it.
	BaseURL string
	HTTP    *http.Client
	Allow   func(ctx context.Context) error
	Done    func(ctx context.Context, failed bool)
}

const defaultBase = "https://ons-aws-prod-opendata.s3.amazonaws.com/dataset/balanco_energia_subsistema_ho"

func New() *Client {
	return &Client{BaseURL: defaultBase,
		HTTP: &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

// ONS stamps hours in Brasília time (UTC-3, no daylight saving since 2019). Earlier years had it, which this connector does
// not correct for: readings before 2019 are not requested by the platform (the automatic loading goes back 365 days at most).
var brt = time.FixedZone("BRT", -3*3600)

type row struct {
	at                          time.Time
	sub                         string
	hydro, thermal, wind, solar float64
}

type cached struct {
	at   time.Time
	rows []row
}

var (
	cacheMu sync.Mutex
	cache   = map[string]cached{}
)

// cacheTTL: a year file is a few MB and several Brazilian regions ask for it in the same refresh.
const cacheTTL = 20 * time.Minute

// Hourly returns the computed intensity of a subsystem for [from, to).
func (c *Client) Hourly(ctx context.Context, subsystem string, from, to time.Time) ([]Reading, error) {
	var out []Reading
	for y := from.In(brt).Year(); y <= to.Add(-time.Second).In(brt).Year(); y++ {
		rows, err := c.year(ctx, y)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if r.sub != subsystem || r.at.Before(from) || !r.at.Before(to) {
				continue
			}
			total := r.hydro + r.thermal + r.wind + r.solar
			if total <= 0 {
				continue
			}
			g := (r.hydro*HydroFactor + r.thermal*ThermalFactor + r.wind*WindFactor + r.solar*SolarFactor) / total
			if g < 1 || g > 1500 || math.IsNaN(g) {
				continue
			}
			out = append(out, Reading{At: r.at, G: g})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out, nil
}

func (c *Client) year(ctx context.Context, y int) ([]row, error) {
	u := fmt.Sprintf("%s/BALANCO_ENERGIA_SUBSISTEMA_%d.csv", strings.TrimRight(c.BaseURL, "/"), y)
	cacheMu.Lock()
	if e, ok := cache[u]; ok && time.Since(e.at) < cacheTTL {
		cacheMu.Unlock()
		return e.rows, nil
	}
	cacheMu.Unlock()
	rows, err := c.fetch(ctx, u)
	if err != nil {
		return nil, err
	}
	cacheMu.Lock()
	cache[u] = cached{at: time.Now(), rows: rows}
	cacheMu.Unlock()
	return rows, nil
}

func (c *Client) fetch(ctx context.Context, u string) (rows []row, err error) {
	if c.Allow != nil {
		if err := c.Allow(ctx); err != nil {
			return nil, fmt.Errorf("ons: %w", err)
		}
	}
	defer func() {
		if c.Done != nil {
			c.Done(ctx, err != nil)
		}
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("ons: request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound, http.StatusForbidden: // a year that is not published (yet): no data, not a failure
		return nil, nil
	default:
		return nil, fmt.Errorf("ons: status %d", resp.StatusCode)
	}
	return parse(io.LimitReader(resp.Body, 64<<20))
}

// parse reads the semicolon-separated file: id_subsistema;nom_subsistema;din_instante;val_gerhidraulica;val_gertermica;
// val_gereolica;val_gersolar;val_carga;val_intercambio. The SIN total row is skipped (it is not a subsystem).
func parse(r io.Reader) ([]row, error) {
	cr := csv.NewReader(r)
	cr.Comma = ';'
	cr.FieldsPerRecord = -1
	head, err := cr.Read()
	if err != nil {
		return nil, errors.New("ons: the file is empty")
	}
	idx := map[string]int{}
	for i, h := range head {
		idx[strings.TrimPrefix(strings.ToLower(strings.TrimSpace(h)), "\xef\xbb\xbf")] = i
	}
	need := []string{"id_subsistema", "din_instante", "val_gerhidraulica", "val_gertermica", "val_gereolica", "val_gersolar"}
	for _, n := range need {
		if _, ok := idx[n]; !ok {
			return nil, errors.New("ons: the file does not have the expected columns")
		}
	}
	var out []row
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil || len(rec) <= idx["val_gersolar"] {
			continue
		}
		sub := strings.ToUpper(strings.TrimSpace(rec[idx["id_subsistema"]]))
		if sub == "SIN" {
			continue
		}
		at, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(rec[idx["din_instante"]]), brt)
		if err != nil {
			continue
		}
		f := func(name string) float64 {
			v, err := strconv.ParseFloat(strings.TrimSpace(rec[idx[name]]), 64)
			if err != nil || v < 0 {
				return 0
			}
			return v
		}
		out = append(out, row{at: at.UTC(), sub: sub, hydro: f("val_gerhidraulica"), thermal: f("val_gertermica"), wind: f("val_gereolica"), solar: f("val_gersolar")})
	}
	return out, nil
}

// Check proves the dataset can be reached: the current year's file, or the previous one in January.
func (c *Client) Check(ctx context.Context) error {
	now := time.Now().In(brt)
	for _, y := range []int{now.Year(), now.Year() - 1} {
		rows, err := c.year(ctx, y)
		if err != nil {
			return err
		}
		if len(rows) > 0 {
			return nil
		}
	}
	return errors.New("ons: no data was returned")
}
