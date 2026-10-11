package domain

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Access modes: how the platform authenticates to the customer's cloud. Shown on a connection (`access`) so the
// console can explain what to check.
const (
	AccessAWSRole          = "role"                // AWS: AssumeRole into the customer's role (ExternalId)
	AccessAWSKey           = "access_key"          // AWS: the customer's read-only IAM user key, sealed
	AccessGCPExport        = "dataset_share"       // GCP: the platform's service account reads the shared dataset
	AccessGCPKey           = "service_account_key" // GCP: the customer's service account key, sealed
	AccessAzureExport      = "export"              // Azure: Cost Management export read from Blob Storage
	AccessAzurePrincipal   = "service_principal"   // Azure: the customer's service principal, sealed secret
	awsKeyScheme           = "awskey://"
	maxServiceAccountBytes = 16 << 10
)

var (
	awsAccessKeyIDRe = regexp.MustCompile(`^AKIA[A-Z0-9]{16}$`)
	awsSecretKeyRe   = regexp.MustCompile(`^[A-Za-z0-9/+=]{40}$`)
	gcpSAEmailRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{4,28}[a-z0-9]@[a-z][a-z0-9-]{4,28}[a-z0-9]\.iam\.gserviceaccount\.com$`)
)

// AccessMode returns the connection's access mode.
func (c Connection) AccessMode() string {
	switch c.Provider {
	case AWS:
		if IsAWSKeyRef(c.CredentialRef) {
			return AccessAWSKey
		}
		return AccessAWSRole
	case GCP:
		if c.SealedSecret != "" {
			return AccessGCPKey
		}
		return AccessGCPExport
	case Azure:
		if IsAzureCostRef(c.CredentialRef) {
			return AccessAzurePrincipal
		}
		return AccessAzureExport
	}
	return ""
}

// IsAWSKeyRef reports whether a credential_ref names an IAM access key ("awskey://<access key id>"). The key id is an
// identifier, not a secret (the secret access key is sealed separately).
func IsAWSKeyRef(ref string) bool { return strings.HasPrefix(ref, awsKeyScheme) }

// ParseAWSKeyRef returns the access key id of "awskey://<access key id>".
func ParseAWSKeyRef(ref string) (string, error) {
	id, ok := strings.CutPrefix(ref, awsKeyScheme)
	if !ok || !awsAccessKeyIDRe.MatchString(id) {
		return "", fmt.Errorf("%w: credential_ref must look like awskey://AKIA… (the access key id of a long-term IAM user key)", ErrInvalidConnection)
	}
	return id, nil
}

// ValidAWSSecretKey reports whether the value has the shape of a secret access key (40 characters).
func ValidAWSSecretKey(s string) bool { return awsSecretKeyRe.MatchString(s) }

// ServiceAccountKey is what the platform needs to know about a pasted Google service account key without ever
// returning it.
type ServiceAccountKey struct{ ClientEmail, ProjectID string }

// ParseServiceAccountKey checks that the text is a Google service account key file: the JSON of type
// "service_account" with an email, a project and a private key. Nothing else is accepted (an OAuth client file or a
// user's credentials file would be a broader grant than intended).
func ParseServiceAccountKey(s string) (ServiceAccountKey, error) {
	bad := func(why string) (ServiceAccountKey, error) {
		return ServiceAccountKey{}, fmt.Errorf("%w: client_secret must be the JSON key file of a service account (%s)", ErrInvalidConnection, why)
	}
	if len(s) == 0 || len(s) > maxServiceAccountBytes {
		return bad("empty or too large")
	}
	var k struct {
		Type        string `json:"type"`
		ProjectID   string `json:"project_id"`
		PrivateKey  string `json:"private_key"`
		ClientEmail string `json:"client_email"`
		TokenURI    string `json:"token_uri"`
	}
	if err := json.Unmarshal([]byte(s), &k); err != nil {
		return bad("not JSON")
	}
	switch {
	case k.Type != "service_account":
		return bad(`"type" must be "service_account"`)
	case !gcpSAEmailRe.MatchString(k.ClientEmail):
		return bad("client_email is not a service account address")
	case !gcpProjectRe.MatchString(k.ProjectID):
		return bad("project_id is missing or invalid")
	case !strings.Contains(k.PrivateKey, "BEGIN PRIVATE KEY"):
		return bad("private_key is missing")
	case k.TokenURI != "" && k.TokenURI != "https://oauth2.googleapis.com/token":
		return bad("token_uri is not Google's")
	}
	return ServiceAccountKey{ClientEmail: k.ClientEmail, ProjectID: k.ProjectID}, nil
}
