package domain

import (
	"errors"
	"testing"
)

const (
	testTenant = "99999999-8888-7777-6666-555555555555"
	testSub    = "11111111-2222-3333-4444-555555555555"
)

func TestParseAzureExportRef(t *testing.T) {
	good, err := ParseAzureExportRef("azblob://" + testTenant + "@acmebilling/exports/rlx/daily-focus")
	if err != nil {
		t.Fatal(err)
	}
	if good.TenantID != testTenant || good.Account != "acmebilling" || good.Container != "exports" || good.Dir != "rlx/daily-focus" {
		t.Errorf("parsed wrong: %+v", good)
	}
	for _, ref := range []string{
		"", "azblob://acmebilling/exports/dir", // no tenant
		"azblob://" + testTenant + "@acmebilling/exports",             // no directory: the export folder is required
		"azblob://" + testTenant + "@AcmeBilling/exports/dir",         // upper case account
		"azblob://" + testTenant + "@acmebilling.evil.com/exports/d",  // a host, not an account name
		"azblob://" + testTenant + "@acmebilling/Exports/dir",         // upper case container
		"azblob://" + testTenant + "@acmebilling/exports/../other",    // traversal
		"azblob://" + testTenant + "@acmebilling/exports//dir",        // empty segment
		"azblob://not-a-guid@acmebilling/exports/dir",                 // tenant must be a GUID
		"https://acmebilling.blob.core.windows.net/exports/dir",       // a URL, possibly with a SAS token
		"azblob://" + testTenant + "@acmebilling/exports/dir?sig=abc", // query string
	} {
		if _, err := ParseAzureExportRef(ref); !errors.Is(err, ErrInvalidConnection) {
			t.Errorf("%q must be rejected, got %v", ref, err)
		}
	}
}

func TestAzureConnectionValidate(t *testing.T) {
	c := Connection{ProjectID: "p", Provider: Azure, AccountRef: testSub, CredentialRef: "azblob://" + testTenant + "@acmebilling/exports/rlx"}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	c.AccountRef = "my-subscription"
	if err := c.Validate(); !errors.Is(err, ErrInvalidConnection) {
		t.Errorf("a subscription id must be a GUID: %v", err)
	}
}

func TestAzureOwnershipKeyIsAMetadataIdentifier(t *testing.T) {
	if got := AzureOwnershipKey("rlx-0a1b"); got != "rlx_0a1b" {
		t.Errorf("key = %q", got)
	}
}

func TestSubscriptionNotInExportNeedsTheCustomer(t *testing.T) {
	if !NeedsCustomerAction(ErrSubscriptionNotInExport) {
		t.Error("a wrong subscription cannot be fixed by retrying")
	}
}
