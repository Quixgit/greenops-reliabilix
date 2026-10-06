package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/quixgit/greenops-reliabilix/backend/internal/app"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/providers/electricitymaps"
	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/providers/gcp"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/config"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/storage"
)

// doctor verifies every external dependency and says what to fix. It never prints secrets (no DSNs, keys
// or tokens), only names, hosts without credentials, counts and fix hints. Exit status 1 when anything failed.

type level int

const (
	lvlOK level = iota
	lvlInfo
	lvlWarn
	lvlFail
)

type finding struct {
	lvl       level
	name, msg string
	hint      string
}

func ok(name, msg string) finding         { return finding{lvlOK, name, msg, ""} }
func info(name, msg string) finding       { return finding{lvlInfo, name, msg, ""} }
func warn(name, msg, hint string) finding { return finding{lvlWarn, name, msg, hint} }
func fail(name, msg, hint string) finding { return finding{lvlFail, name, msg, hint} }

func runDoctor(ctx context.Context, cfg config.Config, cfgErr error, log *slog.Logger) int {
	var all []finding
	all = append(all, checkConfig(cfg, cfgErr)...)
	all = append(all, checkDatabase(ctx, cfg)...)
	all = append(all, checkRedis(cfg)...)
	all = append(all, checkStorage(ctx, cfg, log)...)
	all = append(all, checkAuth0(ctx, cfg)...)
	all = append(all, checkElectricityMaps(ctx, cfg)...)
	all = append(all, checkAWS(ctx, cfg)...)
	all = append(all, checkGCP(ctx, cfg)...)
	all = append(all, checkObservability(cfg)...)

	labels := map[level]string{lvlOK: "OK  ", lvlInfo: "INFO", lvlWarn: "WARN", lvlFail: "FAIL"}
	counts := map[level]int{}
	for _, f := range all {
		counts[f.lvl]++
		fmt.Printf("[%s] %-18s %s\n", labels[f.lvl], f.name, f.msg)
		if f.hint != "" && f.lvl >= lvlWarn {
			fmt.Printf("       -> %s\n", f.hint)
		}
	}
	fmt.Printf("\n%d ok, %d info, %d warnings, %d failures (ENV=%s)\n", counts[lvlOK], counts[lvlInfo], counts[lvlWarn], counts[lvlFail], cfg.Env)
	if counts[lvlFail] > 0 {
		return 1
	}
	return 0
}

func checkConfig(cfg config.Config, err error) []finding {
	var out []finding
	if err != nil {
		out = append(out, fail("config", err.Error(), "set the missing variable (see docs/setup/README.md and .env.example)"))
	} else {
		out = append(out, ok("config", "required variables for ENV="+cfg.Env+" are set"))
	}
	if cfg.IsDev() {
		if strings.HasPrefix(cfg.HTTPAddr, ":") || strings.HasPrefix(cfg.HTTPAddr, "0.0.0.0") {
			out = append(out, warn("dev-exposure", "ENV=dev accepts unauthenticated dev tokens and HTTP_ADDR listens on all interfaces",
				"bind to 127.0.0.1 (HTTP_ADDR=127.0.0.1:8080) or set ENV=prod; never expose a dev instance"))
		}
	} else {
		for _, o := range cfg.CORSOrigins {
			if !strings.HasPrefix(o, "https://") {
				out = append(out, warn("cors", "a CORS origin is not https://", "use https origins in production"))
				break
			}
		}
	}
	if _, perr := electricitymaps.ParseOverrides(cfg.ElectricityMapsZoneOverrides); perr != nil {
		out = append(out, fail("zone-overrides", perr.Error(), "format: ELECTRICITYMAPS_ZONE_OVERRIDES=eu-central-1=DE,ap-southeast-2=AU-NSW"))
	}
	return out
}

var devSecret = regexp.MustCompile(`(?i)dev-only|changeme|password|secret`)

func checkDatabase(ctx context.Context, cfg config.Config) []finding {
	var out []finding
	if cfg.DatabaseURL == "" {
		return []finding{fail("database", "DATABASE_URL is not set", "export the DSN of the greenops_api (or greenops_worker) role")}
	}
	if u, err := url.Parse(cfg.DatabaseURL); err != nil {
		return []finding{fail("database", "DATABASE_URL is not a valid URL", "use postgres://user:pass@host:5432/greenops?sslmode=require")}
	} else if !cfg.IsDev() {
		if pw, _ := u.User.Password(); pw == "" || devSecret.MatchString(pw) {
			out = append(out, fail("db-password", "the database password is empty or a development placeholder", "set a strong password from your secret manager"))
		}
		switch u.Query().Get("sslmode") {
		case "require", "verify-ca", "verify-full":
		default:
			out = append(out, fail("db-tls", "database connection is not forced to use TLS", "append ?sslmode=require (or verify-full with a CA)"))
		}
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pool, err := database.Connect(cctx, cfg.DatabaseURL)
	if err != nil {
		return append(out, fail("database", "cannot connect: "+sanitize(err), "check host, port, firewall and credentials; roles are created by migration 0001, passwords are yours to set (ALTER ROLE ... PASSWORD)"))
	}
	defer pool.Close()
	st, err := database.CheckSchema(cctx, pool, time.Now())
	if err != nil {
		return append(out, fail("database", "schema inspection failed: "+sanitize(err), "were the migrations applied? make migrate"))
	}
	out = append(out, ok("database", "connected as role "+st.Role))
	if st.Superuser || st.BypassRLS {
		out = append(out, fail("db-role", "the application role is superuser or BYPASSRLS: row level security would NOT protect tenants",
			"connect as greenops_api / greenops_worker (created by migration 0001), never as the owner or postgres"))
	} else if st.Role != "greenops_api" && st.Role != "greenops_worker" {
		out = append(out, warn("db-role", "unexpected role "+st.Role, "the api uses greenops_api and the worker greenops_worker"))
	} else {
		out = append(out, ok("db-role", "no superuser, no BYPASSRLS"))
	}
	switch {
	case st.MigrationVersion == 0:
		out = append(out, warn("migrations", "goose version table not found, cannot verify the schema level", "apply migrations with goose: make migrate"))
	case st.MigrationVersion < database.ExpectedMigration:
		out = append(out, fail("migrations", fmt.Sprintf("schema is at migration %d but this build needs %d", st.MigrationVersion, database.ExpectedMigration), "run: make migrate (goose up) before starting the new version"))
	default:
		out = append(out, ok("migrations", fmt.Sprintf("schema at migration %d", st.MigrationVersion)))
	}
	if len(st.UnprotectedTables) > 0 {
		sort.Strings(st.UnprotectedTables)
		out = append(out, fail("rls", "tables without ENABLE + FORCE row level security: "+strings.Join(st.UnprotectedTables, ", "), "enable RLS with platform.enable_tenant_rls(...) in a migration"))
	} else {
		out = append(out, ok("rls", fmt.Sprintf("row level security enforced on %d tenant tables", st.TenantTables)))
	}
	if st.MonthPartition {
		out = append(out, ok("partitions", "current month usage partition exists"))
	} else {
		out = append(out, warn("partitions", "current month usage partition is missing", "the scheduler creates it daily; create it now with: admin partitions (worker role)"))
	}
	return out
}

func checkRedis(cfg config.Config) []finding {
	q := queue.NewClient(cfg.RedisAddr)
	defer func() { _ = q.Close() }()
	if err := q.Ping(context.Background()); err != nil {
		return []finding{fail("redis", "cannot reach Redis at "+cfg.RedisAddr+": "+sanitize(err), "start Redis or fix REDIS_ADDR; the queue (sync, carbon, reports) needs it")}
	}
	return []finding{ok("redis", "reachable at "+cfg.RedisAddr)}
}

func checkStorage(ctx context.Context, cfg config.Config, log *slog.Logger) []finding {
	store, err := app.NewStore(ctx, cfg, log)
	if err != nil {
		return []finding{fail("storage", sanitize(err), "set S3_BUCKET (and S3_ENDPOINT/S3_PATH_STYLE for MinIO or R2)")}
	}
	key, kerr := storage.Key("_doctor", "probe", fmt.Sprintf("%d.txt", time.Now().UnixNano()))
	if kerr != nil {
		return []finding{fail("storage", kerr.Error(), "")}
	}
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	probe := []byte("greenops doctor probe")
	if err := store.Put(cctx, key, bytes.NewReader(probe)); err != nil {
		return []finding{fail("storage", "write failed: "+sanitize(err), "check the bucket exists, credentials and IAM (s3:PutObject/GetObject/DeleteObject on the bucket)")}
	}
	defer func() { _ = store.Delete(context.Background(), key) }()
	rc, err := store.Get(cctx, key)
	if err != nil {
		return []finding{fail("storage", "read-back failed: "+sanitize(err), "the identity needs s3:GetObject")}
	}
	got, _ := io.ReadAll(io.LimitReader(rc, 1<<10))
	_ = rc.Close()
	if !bytes.Equal(got, probe) {
		return []finding{fail("storage", "read-back returned different content", "")}
	}
	kind := "S3-compatible bucket " + cfg.S3Bucket
	if cfg.S3Bucket == "" {
		kind = "local directory (development only)"
	}
	return []finding{ok("storage", "write, read and delete work on "+kind)}
}

var auth0DomainRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$`)

func checkAuth0(ctx context.Context, cfg config.Config) []finding {
	if cfg.IsDev() {
		return []finding{info("auth0", "skipped in ENV=dev (dev tokens are used)")}
	}
	if !auth0DomainRe.MatchString(cfg.Auth0Domain) || cfg.Auth0Audience == "" {
		return []finding{fail("auth0", "AUTH0_DOMAIN must be a bare host name (e.g. tenant.eu.auth0.com) and AUTH0_AUDIENCE must be set", "see docs/setup/README.md, Auth0")}
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+cfg.Auth0Domain+"/.well-known/jwks.json", nil)
	cl := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := cl.Do(req) //nolint:gosec // host validated against a strict pattern above
	if err != nil {
		return []finding{fail("auth0", "cannot fetch the JWKS: "+sanitize(err), "check AUTH0_DOMAIN and outbound HTTPS")}
	}
	defer func() { _ = resp.Body.Close() }()
	var set struct{ Keys []json.RawMessage }
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&set) != nil || len(set.Keys) == 0 {
		return []finding{fail("auth0", fmt.Sprintf("JWKS endpoint answered %d without keys", resp.StatusCode), "AUTH0_DOMAIN is probably wrong")}
	}
	return []finding{ok("auth0", fmt.Sprintf("JWKS reachable, %d signing key(s); audience configured", len(set.Keys)))}
}

func checkElectricityMaps(ctx context.Context, cfg config.Config) []finding {
	if cfg.ElectricityMapsKey == "" {
		if cfg.IsDev() {
			return []finding{info("electricitymaps", "no key: development uses a static 400 g/kWh")}
		}
		return []finding{warn("electricitymaps", "ELECTRICITYMAPS_API_KEY is not set: only stored grid readings are used, no live data", "create an API key at electricitymaps.com and set it")}
	}
	overrides, _ := electricitymaps.ParseOverrides(cfg.ElectricityMapsZoneOverrides)
	c := electricitymaps.New(cfg.ElectricityMapsKey, overrides)
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var out []finding
	if in, err := c.GetIntensity(cctx, "eu-central-1"); err != nil {
		out = append(out, fail("electricitymaps", "live request failed: "+sanitize(err), "check the API key, plan and zone access"))
	} else {
		out = append(out, ok("electricitymaps", fmt.Sprintf("live request works (eu-central-1: %.0f gCO2e/kWh)", in.GPerKWh)))
	}
	if avail, err := c.Zones(cctx); err != nil {
		out = append(out, warn("zones", "could not list the zones available to the key: "+sanitize(err), "verify the region -> zone map manually"))
	} else {
		var missing []string
		for region, zone := range c.ZoneMap() {
			if _, ok := avail[zone]; !ok {
				missing = append(missing, region+"="+zone)
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			out = append(out, warn("zones", "mapped zones not available to this key: "+strings.Join(missing, ", "),
				"those regions get no carbon data; fix with ELECTRICITYMAPS_ZONE_OVERRIDES=region=ZONE or upgrade the plan"))
		} else {
			out = append(out, ok("zones", fmt.Sprintf("all %d mapped regions resolve to available zones", len(c.ZoneMap()))))
		}
	}
	return out
}

func checkAWS(ctx context.Context, cfg config.Config) []finding {
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	ac, err := awsconfig.LoadDefaultConfig(cctx, awsconfig.WithRegion("us-east-1"))
	var id *sts.GetCallerIdentityOutput
	if err == nil {
		id, err = sts.NewFromConfig(ac).GetCallerIdentity(cctx, &sts.GetCallerIdentityInput{})
	}
	if err != nil {
		lvl := warn
		if !cfg.IsDev() {
			lvl = fail
		}
		return []finding{lvl("aws-identity", "no usable AWS identity for the platform: "+sanitize(err),
			"give the api/worker an IAM role (or credentials) that may call sts:AssumeRole on customer roles; see docs/setup/README.md, AWS")}
	}
	acct := ""
	if id.Account != nil {
		acct = *id.Account
	}
	switch {
	case cfg.PlatformAWSAccountID == "":
		return []finding{warn("aws-identity", "platform AWS identity is account "+acct+", but PLATFORM_AWS_ACCOUNT_ID is not set", "set PLATFORM_AWS_ACCOUNT_ID="+acct+": customers need it for the role trust policy")}
	case cfg.PlatformAWSAccountID != acct:
		return []finding{fail("aws-identity", "the platform runs as account "+acct+" but PLATFORM_AWS_ACCOUNT_ID says "+cfg.PlatformAWSAccountID, "customers would trust the wrong account and AssumeRole would fail: fix PLATFORM_AWS_ACCOUNT_ID or the credentials")}
	}
	return []finding{ok("aws-identity", "platform identity is account "+acct+" (matches PLATFORM_AWS_ACCOUNT_ID)")}
}

// checkGCP verifies the platform's own Google identity (the GCP connector is opt-in via PLATFORM_GCP_PROJECT).
func checkGCP(ctx context.Context, cfg config.Config) []finding {
	if cfg.PlatformGCPProject == "" {
		return []finding{info("gcp", "PLATFORM_GCP_PROJECT not set: the GCP connector is disabled")}
	}
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	p, err := gcp.New(cctx, cfg.PlatformGCPProject)
	if err != nil {
		return []finding{fail("gcp-identity", "no usable Google identity for the platform: "+sanitize(err),
			"give the api/worker Application Default Credentials (a service account key file via GOOGLE_APPLICATION_CREDENTIALS, or workload identity); see docs/setup/README.md, GCP")}
	}
	if err := p.Preflight(cctx); err != nil {
		return []finding{fail("gcp-bigquery", "BigQuery is not usable in project "+cfg.PlatformGCPProject+": "+sanitize(err),
			"enable the BigQuery API in that project and grant the platform service account roles/bigquery.jobUser on it")}
	}
	out := []finding{ok("gcp-bigquery", "BigQuery jobs can be created in project "+cfg.PlatformGCPProject)}
	if cfg.PlatformGCPServiceAccount == "" {
		out = append(out, warn("gcp-principal", "PLATFORM_GCP_SERVICE_ACCOUNT is not set",
			"set it to the service account's email: customers are told to grant it read access to their billing dataset"))
	} else {
		out = append(out, ok("gcp-principal", "customers are asked to share their billing dataset with "+cfg.PlatformGCPServiceAccount))
	}
	return out
}

func checkObservability(cfg config.Config) []finding {
	var out []finding
	if cfg.SentryDSN == "" {
		lvl := info
		if !cfg.IsDev() {
			out = append(out, warn("sentry", "SENTRY_DSN is not set: errors are only in logs", "create a Sentry project and set SENTRY_DSN"))
		} else {
			out = append(out, lvl("sentry", "not configured (optional in development)"))
		}
	} else {
		out = append(out, ok("sentry", "DSN configured"))
	}
	if cfg.OTLPEndpoint == "" {
		out = append(out, info("tracing", "OTEL_EXPORTER_OTLP_ENDPOINT not set: traces are disabled (metrics and logs still work)"))
	} else {
		out = append(out, ok("tracing", "OTLP endpoint configured"))
	}
	return out
}

// sanitize keeps error text useful but strips anything that could be a credential or full URL with userinfo.
var urlUserinfo = regexp.MustCompile(`(?i)(postgres(ql)?|redis|https?)://[^@\s/]+@`)

func sanitize(err error) string {
	s := urlUserinfo.ReplaceAllString(err.Error(), "${1}://***@")
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}
