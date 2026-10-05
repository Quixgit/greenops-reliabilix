package domain

import "testing"

func TestConnectionValidate(t *testing.T) {
	ok := Connection{ //nolint:gosec // test fixture: an IAM role ARN is a reference, not a secret
		ProjectID: "p", Provider: AWS, AccountRef: "123456789012", CredentialRef: "arn:aws:iam::123456789012:role/ReliabilixReadOnly"}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := ok
	bad.CredentialRef = "AKIAIOSFODNN7EXAMPLE"
	if bad.Validate() == nil {
		t.Error("raw access key accepted as credential_ref")
	}
	bad = ok
	bad.Provider = "oracle"
	if bad.Validate() == nil {
		t.Error("unknown provider accepted")
	}
}
