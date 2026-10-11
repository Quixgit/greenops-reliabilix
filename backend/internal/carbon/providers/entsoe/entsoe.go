// Package entsoe computes the carbon intensity of a European bidding zone or control area from the actual generation per
// production type published on the ENTSO-E Transparency Platform (https://transparency.entsoe.eu; free with a personal
// security token, requested from ENTSO-E by e-mail after registering).
//
// The figure is CALCULATED, not measured: intensity = sum(generation x emission factor) / sum(generation), with generic
// life-cycle factors per production type. It is labelled as computed from the generation mix and ranks behind a source
// that publishes a measured intensity. Cross-border flows are not accounted for, and pumped-storage generation is left out.
// The token is sent as a query parameter (ENTSO-E offers nothing else) and is never part of an error.
package entsoe

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Factors are gCO2e per kWh of generation by ENTSO-E production type code (B01...B20): generic life-cycle medians
// (IPCC AR5, WG III Annex III) for coal, gas, biomass, solar PV, geothermal, hydro, nuclear and wind; lignite, oil, oil
// shale, peat, derived gas, marine, waste and "other" are assumptions. They are approximations for an estimate.
var Factors = map[string]float64{
	"B01": 230,  // biomass
	"B02": 1000, // fossil brown coal / lignite (assumption: dirtier than the AR5 coal median)
	"B03": 500,  // fossil coal-derived gas (assumption)
	"B04": 490,  // fossil gas
	"B05": 820,  // fossil hard coal
	"B06": 650,  // fossil oil (assumption)
	"B07": 800,  // fossil oil shale (assumption)
	"B08": 1000, // fossil peat (assumption)
	"B09": 38,   // geothermal
	"B11": 24,   // hydro run-of-river and poundage
	"B12": 24,   // hydro water reservoir
	"B13": 17,   // marine (assumption)
	"B14": 12,   // nuclear
	"B15": 50,   // other renewable (assumption)
	"B16": 48,   // solar
	"B17": 450,  // waste (assumption: part biogenic, part fossil)
	"B18": 12,   // wind offshore
	"B19": 11,   // wind onshore
	"B20": 450,  // other (assumption)
}

// pumpedStorage (B10) is storage discharge, not primary generation: left out like the other connectors do.
const pumpedStorage = "B10"

// Domains are the ENTSO-E area codes (EIC) this connector asks for, keyed by a zone key of the platform's zone map (the
// country, or the sub-zone where the zone map has one). Great Britain is not published on the platform since 2021 (the
// UK Carbon Intensity source covers it). A code is data of ENTSO-E and is verified with the token on the console's test.
var Domains = map[string]string{
	"DE": "10Y1001A1001A82H", "FR": "10YFR-RTE------C", "ES": "10YES-REE------0", "IT": "10YIT-GRTN-----B", "IT-NO": "10Y1001A1001A73I",
	"PL": "10YPL-AREA-----S", "SE": "10YSE-1--------K", "FI": "10YFI-1--------U", "BE": "10YBE----------2", "NL": "10YNL----------L",
	"CH": "10YCH-SWISSGRIDZ", "AT": "10YAT-APG------L", "NO": "10YNO-0--------C", "NO-NO1": "10YNO-1--------2", "NO-NO2": "10YNO-2--------T",
	"DK": "10Y1001A1001A65H", "DK-DK1": "10YDK-1--------W", "DK-DK2": "10YDK-2--------M", "PT": "10YPT-REN------W", "CZ": "10YCZ-CEPS-----N",
	"HU": "10YHU-MAVIR----U", "GR": "10YGR-HTSO-----Y", "IE": "10Y1001A1001A59C", "RO": "10YRO-TEL------P", "BG": "10YCA-BULGARIA-R",
	"HR": "10YHR-HEP------M", "SK": "10YSK-SEPS-----K", "SI": "10YSI-ELES-----O", "RS": "10YCS-SERBIATSOV", "LT": "10YLT-1001A0008Q",
	"LV": "10YLV-1001A00074", "EE": "10Y1001A1001A39I",
}

// Reading is one hour of computed intensity.
type Reading struct {
	At time.Time // start of the hour, UTC
	G  float64
}

type Client struct {
	BaseURL string // default https://web-api.tp.entsoe.eu/api
	Token   string
	HTTP    *http.Client
	Allow   func(ctx context.Context) error
	Done    func(ctx context.Context, failed bool)
}

func New(token string) *Client {
	return &Client{BaseURL: "https://web-api.tp.entsoe.eu/api", Token: token,
		HTTP: &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

const (
	chunk    = 30 * 24 * time.Hour // a request may cover up to a year; a month keeps the document small
	minShare = 0.8                 // an hour is used only when this share of its generation has a known factor
)

var errNoToken = errors.New("entsoe: no security token is configured")

type document struct {
	XMLName xml.Name
	Reason  struct {
		Code string `xml:"code"`
	} `xml:"Reason"`
	TimeSeries []struct {
		In  string `xml:"inBiddingZone_Domain.mRID"`
		Out string `xml:"outBiddingZone_Domain.mRID"`
		PSR struct {
			Type string `xml:"psrType"`
		} `xml:"MktPSRType"`
		Period []struct {
			Interval struct {
				Start string `xml:"start"`
			} `xml:"timeInterval"`
			Resolution string `xml:"resolution"`
			Point      []struct {
				Position int      `xml:"position"`
				Quantity *float64 `xml:"quantity"`
			} `xml:"Point"`
		} `xml:"Period"`
	} `xml:"TimeSeries"`
}

// Hourly returns the computed intensity of an area for [from, to). Hours without generation, or where the types with a known
// factor are under 80% of it, are skipped; a period with no published data is an empty result, not an error.
func (c *Client) Hourly(ctx context.Context, area string, from, to time.Time) ([]Reading, error) {
	if c.Token == "" {
		return nil, errNoToken
	}
	var out []Reading
	for start := from.UTC().Truncate(time.Hour); start.Before(to); start = start.Add(chunk) {
		end := start.Add(chunk)
		if end.After(to) {
			end = to
		}
		q := url.Values{"securityToken": {c.Token}, "documentType": {"A75"}, "processType": {"A16"}, "in_Domain": {area},
			"periodStart": {start.Format("200601021504")}, "periodEnd": {end.UTC().Format("200601021504")}}
		var d document
		if err := c.get(ctx, "?"+q.Encode(), &d); err != nil {
			return nil, err
		}
		out = append(out, mix(d, from, to)...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out, nil
}

// resolutionOf reads an ISO 8601 duration of the forms PT15M, PT30M, PT60M and PT1H.
func resolutionOf(s string) time.Duration {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "PT15M":
		return 15 * time.Minute
	case "PT30M":
		return 30 * time.Minute
	case "PT60M", "PT1H":
		return time.Hour
	}
	return 0
}

// mix turns the series into hourly intensities, weighting every point by the time it covers. A point missing from a series
// repeats the previous one (the platform's "variable block" curve).
func mix(d document, from, to time.Time) []Reading {
	by := map[time.Time]*accumulator{}
	for _, ts := range d.TimeSeries {
		if ts.In == "" || ts.Out != "" || ts.PSR.Type == pumpedStorage {
			continue // consumption (storage charging) or storage discharge
		}
		f, known := Factors[ts.PSR.Type]
		if !known {
			f = Factors["B20"]
		}
		for _, p := range ts.Period {
			res := resolutionOf(p.Resolution)
			start, err := time.Parse("2006-01-02T15:04Z", p.Interval.Start)
			if res == 0 || err != nil {
				continue
			}
			var last *float64
			pos := 0
			for _, pt := range p.Point {
				for pos+1 < pt.Position && last != nil { // fill a gap with the previous value
					pos++
					add(by, start.Add(time.Duration(pos-1)*res), res, *last, f, known, from, to)
				}
				pos = pt.Position
				last = pt.Quantity
				if last != nil {
					add(by, start.Add(time.Duration(pos-1)*res), res, *last, f, known, from, to)
				}
			}
		}
	}
	out := make([]Reading, 0, len(by))
	for h, a := range by {
		if a.all <= 0 || a.known/a.all < minShare {
			continue
		}
		g := a.weighted / a.all
		if g < 1 || g > 1500 {
			continue
		}
		out = append(out, Reading{At: h, G: g})
	}
	return out
}

type accumulator struct{ known, all, weighted float64 }

func add(by map[time.Time]*accumulator, at time.Time, res time.Duration, mw, f float64, known bool, from, to time.Time) {
	if mw <= 0 || math.IsNaN(mw) {
		return
	}
	h := at.UTC().Truncate(time.Hour)
	if h.Before(from.UTC().Truncate(time.Hour)) || !h.Before(to) {
		return
	}
	a := by[h]
	if a == nil {
		a = &accumulator{}
		by[h] = a
	}
	w := mw * res.Hours()
	a.all += w
	a.weighted += w * f
	if known {
		a.known += w
	}
}

// Check proves the token works with the smallest request: Germany's last day. The token is never part of an error.
func (c *Client) Check(ctx context.Context) error {
	if c.Token == "" {
		return errNoToken
	}
	to := time.Now().UTC().Truncate(time.Hour)
	var d document
	q := url.Values{"securityToken": {c.Token}, "documentType": {"A75"}, "processType": {"A16"}, "in_Domain": {Domains["DE"]},
		"periodStart": {to.Add(-24 * time.Hour).Format("200601021504")}, "periodEnd": {to.Format("200601021504")}}
	if err := c.get(ctx, "?"+q.Encode(), &d); err != nil {
		return err
	}
	if len(d.TimeSeries) == 0 {
		return errors.New("entsoe: the token was accepted but returned no data")
	}
	return nil
}

func (c *Client) get(ctx context.Context, query string, dst *document) (err error) {
	if c.Allow != nil {
		if err := c.Allow(ctx); err != nil {
			return fmt.Errorf("entsoe: %w", err)
		}
	}
	defer func() {
		if c.Done != nil {
			c.Done(ctx, err != nil)
		}
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+query, nil)
	if err != nil {
		return errors.New("entsoe: bad address")
	}
	req.Header.Set("Accept", "application/xml")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("entsoe: request failed") // never the URL: it carries the token
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK, http.StatusBadRequest:
		raw, rerr := io.ReadAll(io.LimitReader(resp.Body, 48<<20))
		if rerr != nil {
			return errors.New("entsoe: the response could not be read")
		}
		if xerr := xml.Unmarshal(raw, dst); xerr != nil {
			return errors.New("entsoe: the response is not a valid document")
		}
		if dst.XMLName.Local == "Acknowledgement_MarketDocument" {
			if dst.Reason.Code == "999" { // "No matching data found"
				return nil
			}
			return fmt.Errorf("entsoe: the request was refused (reason %s)", dst.Reason.Code)
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("entsoe: status %d", resp.StatusCode)
		}
		return nil
	case http.StatusUnauthorized:
		return errors.New("entsoe: the token was not accepted")
	case http.StatusTooManyRequests:
		return errors.New("entsoe: rate limit reached, try later")
	}
	return fmt.Errorf("entsoe: status %d", resp.StatusCode)
}
