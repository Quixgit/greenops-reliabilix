// Command loadtest measures the read endpoints of a running API under a fixed number of concurrent clients and prints a
// Markdown table: requests per second, latency percentiles and failures (docs/operations/performance.md).
//
//	go run scripts/loadtest/main.go -base http://127.0.0.1:38460 -token dev:load:<tenant>:owner -duration 15s -conc 1,10,50
//
// It is a closed-loop test: each client sends the next request when the previous answer arrived, so the numbers are what
// the service sustains at that concurrency, not what it would do under an open arrival rate. A single endpoint is tried
// at a time; then a mix of all of them, and with -page the opening of the Overview (its widgets in parallel). It only reads.
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type endpoint struct{ name, path string }

func main() {
	base := flag.String("base", "http://127.0.0.1:8080", "API address")
	token := flag.String("token", "", "bearer token (an API key or a development token)")
	dur := flag.Duration("duration", 15*time.Second, "time per endpoint and level")
	concs := flag.String("conc", "1,10,50", "concurrency levels")
	from := flag.String("from", "2026-07-01", "window start")
	to := flag.String("to", "2026-09-30", "window end")
	page := flag.Bool("page", false, "also measure opening the Overview: its widgets asked in parallel, the time is the slowest of them")
	flag.Parse()
	if *token == "" {
		fmt.Fprintln(os.Stderr, "-token is required")
		os.Exit(2)
	}
	w := "?from=" + *from + "&to=" + *to
	eps := []endpoint{
		{"projects", "/api/v1/projects"},
		{"finops/summary", "/api/v1/finops/summary" + w},
		{"carbon/trend", "/api/v1/carbon/trend" + w},
		{"carbon/summary", "/api/v1/carbon/summary" + w},
		{"finops/forecast", "/api/v1/finops/forecast"},
		{"dashboard/services", "/api/v1/dashboard/services" + w},
		{"recommendations", "/api/v1/recommendations"},
	}
	client := &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{MaxIdleConnsPerHost: 256, MaxConnsPerHost: 256}}
	fmt.Println("| endpoint | clients | req/s | p50 ms | p95 ms | p99 ms | max ms | non-200 |")
	fmt.Println("|---|---:|---:|---:|---:|---:|---:|---:|")
	for _, c := range strings.Split(*concs, ",") {
		n, _ := strconv.Atoi(strings.TrimSpace(c))
		if n < 1 {
			continue
		}
		for _, e := range eps {
			row(e.name, n, run(client, *base, *token, n, *dur, func(int) string { return e.path }))
		}
		row("mix of all", n, run(client, *base, *token, n, *dur, func(i int) string { return eps[i%len(eps)].path }))
		if *page {
			row("Overview page (7 widgets in parallel)", n, runPage(client, *base, *token, n, *dur, []string{
				"/api/v1/finops/summary" + w, "/api/v1/dashboard/trend" + w, "/api/v1/dashboard/services" + w, "/api/v1/dashboard/regions" + w,
				"/api/v1/carbon/summary" + w, "/api/v1/finops/forecast", "/api/v1/recommendations"}))
		}
	}
}

type result struct {
	lat  []time.Duration
	bad  int
	took time.Duration
}

func run(c *http.Client, base, token string, n int, dur time.Duration, pick func(int) string) result {
	var mu sync.Mutex
	var all []time.Duration
	bad := 0
	stop := time.Now().Add(dur)
	start := time.Now()
	var wg sync.WaitGroup
	for g := 0; g < n; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			var lat []time.Duration
			b := 0
			for i := g; time.Now().Before(stop); i++ {
				req, _ := http.NewRequest(http.MethodGet, base+pick(i), nil)
				req.Header.Set("Authorization", "Bearer "+token)
				t := time.Now()
				res, err := c.Do(req)
				if err != nil {
					b++
					continue
				}
				_, _ = io.Copy(io.Discard, res.Body)
				_ = res.Body.Close()
				lat = append(lat, time.Since(t))
				if res.StatusCode != http.StatusOK {
					b++
				}
			}
			mu.Lock()
			all, bad = append(all, lat...), bad+b
			mu.Unlock()
		}(g)
	}
	wg.Wait()
	return result{lat: all, bad: bad, took: time.Since(start)}
}

func row(name string, n int, r result) {
	if len(r.lat) == 0 {
		fmt.Printf("| %s | %d | 0 | - | - | - | - | %d |\n", name, n, r.bad)
		return
	}
	sort.Slice(r.lat, func(i, j int) bool { return r.lat[i] < r.lat[j] })
	q := func(p float64) float64 { return float64(r.lat[int(float64(len(r.lat)-1)*p)].Microseconds()) / 1000 }
	fmt.Printf("| %s | %d | %.0f | %.1f | %.1f | %.1f | %.1f | %d |\n", name, n, float64(len(r.lat))/r.took.Seconds(), q(.5), q(.95), q(.99), q(1), r.bad)
}

// runPage is a person opening a page: every widget is asked at once and the page is ready when the slowest has answered.
func runPage(c *http.Client, base, token string, n int, dur time.Duration, paths []string) result {
	var mu sync.Mutex
	var all []time.Duration
	bad := 0
	stop := time.Now().Add(dur)
	start := time.Now()
	var wg sync.WaitGroup
	for g := 0; g < n; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var lat []time.Duration
			b := 0
			for time.Now().Before(stop) {
				t := time.Now()
				var inner sync.WaitGroup
				var im sync.Mutex
				failed := false
				for _, p := range paths {
					inner.Add(1)
					go func(p string) {
						defer inner.Done()
						req, _ := http.NewRequest(http.MethodGet, base+p, nil)
						req.Header.Set("Authorization", "Bearer "+token)
						res, err := c.Do(req)
						if err != nil {
							im.Lock()
							failed = true
							im.Unlock()
							return
						}
						_, _ = io.Copy(io.Discard, res.Body)
						_ = res.Body.Close()
						if res.StatusCode != http.StatusOK {
							im.Lock()
							failed = true
							im.Unlock()
						}
					}(p)
				}
				inner.Wait()
				lat = append(lat, time.Since(t))
				if failed {
					b++
				}
			}
			mu.Lock()
			all, bad = append(all, lat...), bad+b
			mu.Unlock()
		}()
	}
	wg.Wait()
	return result{lat: all, bad: bad, took: time.Since(start)}
}
