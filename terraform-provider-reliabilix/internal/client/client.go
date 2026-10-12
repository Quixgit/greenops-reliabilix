// Package client is the small HTTP client of the Reliabilix API that the provider needs: projects, their policies and budgets.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrNotFound is returned for a 404, so a resource deleted outside Terraform is dropped from the state instead of failing.
var ErrNotFound = errors.New("not found")

type Client struct {
	base string
	key  string
	http *http.Client
}

// New takes the address of the API (https://host, without a path) and an API key of role iac.
func New(endpoint, key string) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(endpoint, "/"))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.Path != "" {
		return nil, fmt.Errorf("endpoint must look like https://api.example.com, got %q", endpoint)
	}
	if key == "" {
		return nil, errors.New("an API key is required")
	}
	return &Client{base: u.String() + "/api/v1", key: key, http: &http.Client{Timeout: 30 * time.Second}}, nil
}

// APIError carries the problem document of a failed call.
type APIError struct {
	Status int
	Title  string
	Detail string
}

func (e *APIError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s (HTTP %d): %s", e.Title, e.Status, e.Detail)
	}
	return fmt.Sprintf("%s (HTTP %d)", e.Title, e.Status)
}

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "terraform-provider-reliabilix")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if res.StatusCode >= 300 {
		var p struct{ Title, Detail string }
		_ = json.Unmarshal(data, &p)
		if p.Title == "" {
			p.Title = http.StatusText(res.StatusCode)
		}
		return &APIError{Status: res.StatusCode, Title: p.Title, Detail: p.Detail}
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

type Project struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	FunctionalUnit string `json:"functional_unit"`
	CreatedAt      string `json:"created_at"`
}

func (c *Client) CreateProject(ctx context.Context, name, unit string) (Project, error) {
	var p Project
	err := c.do(ctx, http.MethodPost, "/projects", map[string]string{"name": name, "functional_unit": unit}, &p)
	return p, err
}

func (c *Client) GetProject(ctx context.Context, id string) (Project, error) {
	var p Project
	err := c.do(ctx, http.MethodGet, "/projects/"+url.PathEscape(id), nil, &p)
	return p, err
}

func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	var out struct {
		Items []Project `json:"items"`
	}
	err := c.do(ctx, http.MethodGet, "/projects", nil, &out)
	return out.Items, err
}

func (c *Client) DeleteProject(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/projects/"+url.PathEscape(id), nil, nil)
}

// Policy is the data-residency and threshold policy of a project. AutoApplyTypes is not managed here: it is carried through
// unchanged, so applying Terraform never switches automatic application on or off.
type Policy struct {
	AllowedRegions       []string `json:"allowed_regions"`
	CICarbonWarnKg       *float64 `json:"ci_carbon_warn_kg"`
	CICarbonBlockKg      *float64 `json:"ci_carbon_block_kg"`
	CICostWarn           *float64 `json:"ci_cost_warn"`
	CICostBlock          *float64 `json:"ci_cost_block"`
	AlertCarbonMonthlyKg *float64 `json:"alert_carbon_monthly_kg"`
	AutoApplyTypes       []string `json:"auto_apply_types"`
}

func (c *Client) GetPolicy(ctx context.Context, projectID string) (Policy, error) {
	var p Policy
	err := c.do(ctx, http.MethodGet, "/projects/"+url.PathEscape(projectID)+"/policy", nil, &p)
	if p.AllowedRegions == nil {
		p.AllowedRegions = []string{}
	}
	return p, err
}

// PutPolicy writes the managed fields and keeps the others as the server has them.
func (c *Client) PutPolicy(ctx context.Context, projectID string, p Policy) (Policy, error) {
	cur, err := c.GetPolicy(ctx, projectID)
	if err != nil {
		return Policy{}, err
	}
	p.AutoApplyTypes = cur.AutoApplyTypes
	if p.AutoApplyTypes == nil {
		p.AutoApplyTypes = []string{}
	}
	if p.AllowedRegions == nil {
		p.AllowedRegions = []string{}
	}
	var out Policy
	err = c.do(ctx, http.MethodPut, "/projects/"+url.PathEscape(projectID)+"/policy", p, &out)
	return out, err
}

type Budget struct {
	ID        string  `json:"id"`
	ProjectID *string `json:"project_id"`
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
	Period    string  `json:"period"`
}

func (c *Client) CreateBudget(ctx context.Context, b Budget) (Budget, error) {
	var out Budget
	err := c.do(ctx, http.MethodPost, "/finops/budgets", map[string]any{"project_id": b.ProjectID, "amount": b.Amount, "currency": b.Currency, "period": b.Period}, &out)
	return out, err
}

func (c *Client) GetBudget(ctx context.Context, id string) (Budget, error) {
	var out struct {
		Items []Budget `json:"items"`
	}
	if err := c.do(ctx, http.MethodGet, "/finops/budgets", nil, &out); err != nil {
		return Budget{}, err
	}
	for _, b := range out.Items {
		if b.ID == id {
			return b, nil
		}
	}
	return Budget{}, ErrNotFound
}

func (c *Client) DeleteBudget(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/finops/budgets/"+url.PathEscape(id), nil, nil)
}
