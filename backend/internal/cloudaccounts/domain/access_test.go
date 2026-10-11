package domain

import (
	"errors"
	"strings"
	"testing"
)

const saKey = `{"type":"service_account","project_id":"acme-prod-123","private_key_id":"abc","private_key":"-----BEGIN PRIVATE KEY-----\nMIIE\n-----END PRIVATE KEY-----\n","client_email":"reliabilix@acme-prod-123.iam.gserviceaccount.com","token_uri":"https://oauth2.googleapis.com/token"}`

func TestAWSAccessKeyReference(t *testing.T) {
	c := Connection{ProjectID: "p", Provider: AWS, AccountRef: "123456789012", CredentialRef: "awskey://AKIAIOSFODNN7EXAMPLE"}
	if err := c.Validate(); err != nil {
		t.Fatalf("an access key id is an identifier and must be accepted: %v", err)
	}
	if c.AccessMode() != AccessAWSKey {
		t.Errorf("mode = %s", c.AccessMode())
	}
	for _, ref := range []string{"awskey://ASIAIOSFODNN7EXAMPLE", "awskey://AKIA", "awskey://", "awskey://AKIAIOSFODNN7EXAMPLE/x"} {
		c.CredentialRef = ref
		if err := c.Validate(); !errors.Is(err, ErrInvalidConnection) {
			t.Errorf("%q must be refused: %v", ref, err)
		}
	}
	// a raw key is still not a valid reference for the role mode
	c.CredentialRef = "AKIAIOSFODNN7EXAMPLE"
	if err := c.Validate(); !errors.Is(err, ErrInvalidConnection) {
		t.Errorf("raw key: %v", err)
	}
	if !ValidAWSSecretKey("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY") || ValidAWSSecretKey("short") {
		t.Error("secret access key shape")
	}
}

func TestServiceAccountKeyIsCheckedAndOnlyThatKind(t *testing.T) {
	k, err := ParseServiceAccountKey(saKey)
	if err != nil || k.ProjectID != "acme-prod-123" || !strings.HasSuffix(k.ClientEmail, ".iam.gserviceaccount.com") {
		t.Fatalf("%+v %v", k, err)
	}
	for name, bad := range map[string]string{
		"empty":        "",
		"not json":     "nope",
		"user creds":   `{"type":"authorized_user","client_id":"x","refresh_token":"y"}`,
		"no key":       strings.Replace(saKey, "BEGIN PRIVATE KEY", "BEGIN", 1),
		"foreign host": strings.Replace(saKey, "https://oauth2.googleapis.com/token", "https://evil.example/token", 1),
		"bad email":    strings.Replace(saKey, "reliabilix@acme-prod-123.iam.gserviceaccount.com", "me@gmail.com", 1),
	} {
		if _, err := ParseServiceAccountKey(bad); !errors.Is(err, ErrInvalidConnection) {
			t.Errorf("%s must be refused: %v", name, err)
		}
	}
	// the refusal must not echo the key
	if _, err := ParseServiceAccountKey(strings.Replace(saKey, "service_account", "other", 1)); err == nil || strings.Contains(err.Error(), "MIIE") {
		t.Errorf("error leaks the key: %v", err)
	}
	g := Connection{Provider: GCP, SealedSecret: "x"}
	if g.AccessMode() != AccessGCPKey || (Connection{Provider: GCP}).AccessMode() != AccessGCPExport {
		t.Error("gcp modes")
	}
}
