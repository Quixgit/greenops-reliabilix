// Package integrations holds the keys and addresses of the external services the platform talks to, and lets an
// operator manage them in the console (ADR-0029). A value comes from the database when an operator set one, else from
// the deployment environment, else it is absent. Secret values are sealed at rest and never returned to a client.
package integrations

import (
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
)

// Field is one setting of an integration.
type Field struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Help        string `json:"help,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Secret      bool   `json:"secret"`
	Required    bool   `json:"required"`
	Env         string `json:"env"` // the environment variable that provides the value when none is set in the console
	Default     string `json:"default,omitempty"`
	validate    func(string) error
}

// Integration is one external service.
type Integration struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Fields      []Field `json:"fields"`
	// Testable integrations have a connection test in the console.
	Testable bool `json:"testable"`
}

var (
	zonePair = regexp.MustCompile(`^[a-z0-9-]{2,40}=[A-Za-z0-9-]{2,40}$`)
	ErrField = errors.New("invalid value")
)

func validZoneOverrides(v string) error {
	for _, p := range strings.Split(v, ",") {
		if !zonePair.MatchString(strings.TrimSpace(p)) {
			return fmt.Errorf("%w: use region=ZONE pairs separated by commas (eu-central-1=DE)", ErrField)
		}
	}
	return nil
}

func validSecret(min int) func(string) error {
	return func(v string) error {
		if len(v) < min || len(v) > 500 || strings.ContainsAny(v, " \t\r\n") {
			return fmt.Errorf("%w: expected %d-500 characters without spaces", ErrField, min)
		}
		return nil
	}
}

var hostRe = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$`)

func validHost(v string) error {
	if !hostRe.MatchString(v) {
		return fmt.Errorf("%w: a host name such as smtp.example.com", ErrField)
	}
	return nil
}

func validPort(v string) error {
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("%w: a port from 1 to 65535", ErrField)
	}
	return nil
}

func validFrom(v string) error {
	if _, err := mail.ParseAddress(v); err != nil || len(v) > 254 || strings.ContainsAny(v, "\r\n") {
		return fmt.Errorf("%w: an address such as Reliabilix <notifications@example.com>", ErrField)
	}
	return nil
}

func validUser(v string) error {
	if len(v) > 254 || strings.ContainsAny(v, "\r\n") {
		return fmt.Errorf("%w: too long or contains a line break", ErrField)
	}
	return nil
}

// Catalog lists the integrations an operator can manage in the console. Services that are fixed by the deployment
// (identity provider, object storage, error tracking) are reported by the system view, not edited here.
func Catalog() []Integration {
	return []Integration{
		{
			ID: "electricitymaps", Name: "Electricity Maps", Testable: true,
			Description: "Grid carbon intensity for the carbon engine and the CI gate. Without a key only stored readings are used.",
			Fields: []Field{
				{Key: "api_key", Label: "API key", Secret: true, Required: true, Env: "ELECTRICITYMAPS_API_KEY", Help: "From the Electricity Maps dashboard. Stored sealed, shown only as configured.", validate: validSecret(8)},
				{Key: "zone_overrides", Label: "Zone overrides", Env: "ELECTRICITYMAPS_ZONE_OVERRIDES", Placeholder: "eu-central-1=DE,us-east-1=US-MIDA-PJM", Help: "Replace or add region to zone mappings. Optional.", validate: validZoneOverrides},
			},
		},
		{
			ID: "smtp", Name: "E-mail (SMTP)", Testable: true,
			Description: "Sends e-mail notifications to organization members. TLS is required.",
			Fields: []Field{
				{Key: "host", Label: "Server", Required: true, Env: "SMTP_HOST", Placeholder: "smtp.example.com", validate: validHost},
				{Key: "port", Label: "Port", Env: "SMTP_PORT", Default: "587", Placeholder: "587", Help: "587 (STARTTLS) or 465 (implicit TLS).", validate: validPort},
				{Key: "username", Label: "User name", Env: "SMTP_USERNAME", validate: validUser},
				{Key: "password", Label: "Password", Secret: true, Env: "SMTP_PASSWORD", Help: "Stored sealed, shown only as configured.", validate: validSecret(1)},
				{Key: "from", Label: "Sender address", Required: true, Env: "SMTP_FROM", Placeholder: "Reliabilix <notifications@example.com>", validate: validFrom},
			},
		},
	}
}

// Lookup finds an integration and one of its fields.
func Lookup(id, key string) (Integration, Field, bool) {
	for _, in := range Catalog() {
		if in.ID != id {
			continue
		}
		if key == "" {
			return in, Field{}, true
		}
		for _, f := range in.Fields {
			if f.Key == key {
				return in, f, true
			}
		}
	}
	return Integration{}, Field{}, false
}

// Validate checks a value for a field.
func (f Field) Validate(v string) error {
	if v == "" {
		return fmt.Errorf("%w: empty", ErrField)
	}
	if f.validate != nil {
		return f.validate(v)
	}
	return nil
}
