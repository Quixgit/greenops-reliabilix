package domain

import (
	"errors"
	"strings"
	"testing"
)

func valid() Connection {
	return Connection{ProjectID: "p", Provider: AWS, AccountRef: "123456789012", //nolint:gosec // fixture: an IAM role ARN is a reference
		CredentialRef: "arn:aws:iam::123456789012:role/ReliabilixReadOnly"}
}

func TestConnectionValidate(t *testing.T) {
	if err := valid().Validate(); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*Connection){
		"raw access key":   func(c *Connection) { c.CredentialRef = "AKIAIOSFODNN7EXAMPLE" },
		"unknown provider": func(c *Connection) { c.Provider = "oracle" },
		"short account":    func(c *Connection) { c.AccountRef = "123" },
		"not an arn":       func(c *Connection) { c.CredentialRef = "ReliabilixReadOnly" },
		"foreign account":  func(c *Connection) { c.CredentialRef = "arn:aws:iam::999999999999:role/ReliabilixX" },
		"user not role":    func(c *Connection) { c.CredentialRef = "arn:aws:iam::123456789012:user/bob" },
		"other role name":  func(c *Connection) { c.CredentialRef = "arn:aws:iam::123456789012:role/AdministratorAccess" },
		"role with path":   func(c *Connection) { c.CredentialRef = "arn:aws:iam::123456789012:role/team/ReliabilixReadOnly" },
		"traversal":        func(c *Connection) { c.CredentialRef = "arn:aws:iam::123456789012:role/Reliabilix/../Admin" },
	}
	for name, mut := range cases {
		c := valid()
		mut(&c)
		if err := c.Validate(); !errors.Is(err, ErrInvalidConnection) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestExternalID(t *testing.T) {
	a, _ := NewExternalID()
	b, _ := NewExternalID()
	if a == b || !strings.HasPrefix(a, "rlx-") || len(a) < 30 {
		t.Errorf("weak external ids: %q %q", a, b)
	}
}

func TestRegistry(t *testing.T) {
	if _, err := NewRegistry().Get(Azure); !errors.Is(err, ErrUnsupportedProvider) {
		t.Errorf("err = %v", err)
	}
}

func TestGCPConnectionValidation(t *testing.T) {
	ok := Connection{Provider: GCP, ProjectID: "p", AccountRef: "acme-prod-123",
		CredentialRef: "bq://acme-billing-1/billing_export/gcp_billing_export_v1_AAAAAA_BBBBBB_CCCCCC"}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid GCP connection rejected: %v", err)
	}
	resource := ok
	resource.CredentialRef = "bq://acme-billing-1/billing_export/gcp_billing_export_resource_v1_AAAAAA_BBBBBB_CCCCCC"
	if err := resource.Validate(); err != nil {
		t.Fatalf("detailed export table rejected: %v", err)
	}
	bad := map[string]func(*Connection){
		"uppercase project":     func(c *Connection) { c.AccountRef = "Acme-Prod" },
		"short project":         func(c *Connection) { c.AccountRef = "abc" },
		"aws style account":     func(c *Connection) { c.AccountRef = "123456789012" },
		"not a bq ref":          func(c *Connection) { c.CredentialRef = "projects/acme/datasets/x" },
		"any other table":       func(c *Connection) { c.CredentialRef = "bq://acme-billing-1/ds/customers" },
		"quote in dataset":      func(c *Connection) { c.CredentialRef = "bq://acme-billing-1/ds`x/gcp_billing_export_v1_A" },
		"extra path segment":    func(c *Connection) { c.CredentialRef = "bq://acme-billing-1/ds/gcp_billing_export_v1_A/extra" },
		"key material in a ref": func(c *Connection) { c.CredentialRef = "-----BEGIN PRIVATE KEY-----" },
	}
	for name, mutate := range bad {
		c := ok
		mutate(&c)
		if err := c.Validate(); !errors.Is(err, ErrInvalidConnection) {
			t.Errorf("%s: want ErrInvalidConnection, got %v", name, err)
		}
	}
}
