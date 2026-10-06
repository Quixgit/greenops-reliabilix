package gcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
)

// BigQuery is the slice of the BigQuery REST API the provider needs (faked in tests).
type BigQuery interface {
	// DatasetLabels returns the labels of a dataset (needs bigquery.datasets.get).
	DatasetLabels(ctx context.Context, project, dataset string) (map[string]string, error)
	// Query runs a parameterized standard-SQL query in the platform's own job project and returns every row as
	// strings. DryRun validates the query and the caller's access without reading or billing anything.
	Query(ctx context.Context, q Query) ([][]string, error)
}

// Query is one BigQuery job. Identifiers are validated by the caller; values travel only as parameters.
type Query struct {
	JobProject       string
	SQL              string
	Params           map[string]Param
	MaxBytesBilled   int64
	DryRun           bool
	MaxRows          int
	Timeout          time.Duration
	ExpectedColumnsN int // the number of result columns; a mismatch means the schema changed
}

// Param is a named query parameter.
type Param struct{ Type, Value string } // Type: STRING | TIMESTAMP

const (
	defaultBase = "https://bigquery.googleapis.com/bigquery/v2"
	// maxResponseBytes bounds a single API response: results are customer-influenced data.
	maxResponseBytes = 32 << 20
)

// RESTClient talks to BigQuery with an authorized http.Client (the platform's own Google identity).
type RESTClient struct {
	HTTP *http.Client
	Base string // overridden in tests
}

func (c *RESTClient) base() string {
	if c.Base != "" {
		return c.Base
	}
	return defaultBase
}

func (c *RESTClient) do(ctx context.Context, method, rawURL string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return err
	}
	if len(raw) > maxResponseBytes {
		return errors.New("gcp: BigQuery response is too large")
	}
	if resp.StatusCode/100 != 2 {
		return classify(resp.StatusCode, raw)
	}
	return json.Unmarshal(raw, out)
}

// errAPIDisabled means "BigQuery API has not been used in project X or it is disabled". Whose project that is
// depends on the call, so callers translate it: for the customer's dataset it is their action to take, for the
// platform's job project it is a platform misconfiguration that must not be blamed on the customer.
var errAPIDisabled = errors.New("gcp: BigQuery API is disabled")

// classify maps BigQuery failures to errors. Messages from Google are not passed on: they can contain project
// and table names that belong to the customer.
func classify(status int, raw []byte) error {
	var e struct {
		Error struct {
			Errors  []struct{ Reason string } `json:"errors"`
			Details []struct{ Reason string } `json:"details"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &e)
	reasons := map[string]bool{}
	for _, x := range e.Error.Errors {
		reasons[x.Reason] = true
	}
	for _, x := range e.Error.Details {
		reasons[x.Reason] = true
	}
	switch {
	case reasons["accessNotConfigured"] || reasons["SERVICE_DISABLED"] || reasons["serviceDisabled"]:
		return errAPIDisabled
	case reasons["bytesBilledLimitExceeded"]:
		return errors.New("gcp: the query would scan more data than the configured limit")
	case status == http.StatusNotFound || reasons["notFound"]:
		return domain.ErrExportNotFound
	case status == http.StatusUnauthorized || status == http.StatusForbidden || reasons["accessDenied"] || reasons["billingNotEnabled"]:
		return domain.ErrAccessDenied
	}
	return fmt.Errorf("gcp: BigQuery request failed (status %d)", status)
}

func (c *RESTClient) DatasetLabels(ctx context.Context, project, dataset string) (map[string]string, error) {
	var out struct {
		Labels map[string]string `json:"labels"`
	}
	u := fmt.Sprintf("%s/projects/%s/datasets/%s", c.base(), url.PathEscape(project), url.PathEscape(dataset))
	if err := c.do(ctx, http.MethodGet, u, nil, &out); err != nil {
		if errors.Is(err, errAPIDisabled) {
			return nil, domain.ErrBigQueryDisabled // the dataset's own project: the customer must enable BigQuery
		}
		return nil, err
	}
	return out.Labels, nil
}

type queryResponse struct {
	JobComplete  bool `json:"jobComplete"`
	JobReference struct {
		JobID    string `json:"jobId"`
		Location string `json:"location"`
	} `json:"jobReference"`
	Schema struct {
		Fields []json.RawMessage `json:"fields"`
	} `json:"schema"`
	Rows []struct {
		F []struct {
			V any `json:"v"`
		} `json:"f"`
	} `json:"rows"`
	PageToken string `json:"pageToken"`
}

func (c *RESTClient) Query(ctx context.Context, q Query) ([][]string, error) {
	timeout := q.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	params := make([]map[string]any, 0, len(q.Params))
	for name, p := range q.Params {
		params = append(params, map[string]any{"name": name, "parameterType": map[string]string{"type": p.Type}, "parameterValue": map[string]string{"value": p.Value}})
	}
	body := map[string]any{
		"query": q.SQL, "useLegacySql": false, "parameterMode": "NAMED", "queryParameters": params,
		"maximumBytesBilled": fmt.Sprint(q.MaxBytesBilled), "dryRun": q.DryRun, "timeoutMs": 30000,
	}
	var res queryResponse
	if err := c.do(ctx, http.MethodPost, fmt.Sprintf("%s/projects/%s/queries", c.base(), url.PathEscape(q.JobProject)), body, &res); err != nil {
		if errors.Is(err, errAPIDisabled) {
			// Queries run in the PLATFORM's project: this is our misconfiguration, never the customer's fault.
			return nil, errors.New("gcp: the BigQuery API is disabled in the platform's job project (operator action required)")
		}
		return nil, err
	}
	if q.DryRun {
		return nil, nil
	}
	var rows [][]string
	for polls := 0; ; {
		if res.JobComplete {
			for _, r := range res.Rows {
				if q.ExpectedColumnsN > 0 && len(r.F) != q.ExpectedColumnsN {
					return nil, fmt.Errorf("gcp: unexpected result shape (%d columns)", len(r.F))
				}
				row := make([]string, len(r.F))
				for i, cell := range r.F {
					if cell.V != nil {
						row[i] = fmt.Sprint(cell.V)
					}
				}
				rows = append(rows, row)
				if q.MaxRows > 0 && len(rows) > q.MaxRows {
					return nil, errors.New("gcp: the billing export returned too many lines (limit exceeded)")
				}
			}
			if res.PageToken == "" {
				return rows, nil
			}
		} else if polls++; polls > 60 {
			return nil, errors.New("gcp: the BigQuery job did not finish in time")
		}
		next := url.Values{"maxResults": {"10000"}, "location": {res.JobReference.Location}, "timeoutMs": {"30000"}}
		if res.JobComplete {
			next.Set("pageToken", res.PageToken)
		}
		u := fmt.Sprintf("%s/projects/%s/queries/%s?%s", c.base(), url.PathEscape(q.JobProject), url.PathEscape(res.JobReference.JobID), next.Encode())
		res = queryResponse{}
		if err := c.do(ctx, http.MethodGet, u, nil, &res); err != nil {
			return nil, err
		}
	}
}
