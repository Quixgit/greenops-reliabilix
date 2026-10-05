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
		"foreign account":  func(c *Connection) { c.CredentialRef = "arn:aws:iam::999999999999:role/X" },
		"user not role":    func(c *Connection) { c.CredentialRef = "arn:aws:iam::123456789012:user/bob" },
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
