package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// Azure connections read the customer's Cost Management export (FOCUS cost and usage, CSV) from a Blob Storage
// container, with the platform's own Microsoft Entra application: the customer only creates that application's
// service principal in their tenant and grants it read access to the container. The platform stores no customer
// credential. Everything the connection carries is a reference:
//
//	account_ref    the Azure subscription id whose costs are ingested (a billing-scope export covers many)
//	credential_ref azblob://<tenant-id>@<storage-account>/<container>/<export directory>
//
// The export directory is the folder the export writes to (root folder and export name, as Cost Management lays
// them out: <root>/<export name>/<yyyymmdd-yyyymmdd>/<run>/part_*.csv). Ownership is proven with container
// metadata, the Azure counterpart of the AWS ExternalId and the GCP dataset label: the customer sets the
// metadata key AzureOwnershipKey(external_id) to "1", which only someone with write access to the container
// can do. Without it a tenant could name another customer's container (the platform's principal may legitimately
// read it) and pull that customer's billing data: the confused-deputy problem.
var (
	azureGUIDRe      = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	azureAccountRe   = regexp.MustCompile(`^[a-z0-9]{3,24}$`)
	azureContainerRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9]|-[a-z0-9]){2,62}$`)
)

// AzureExport locates the Cost Management export in Blob Storage.
type AzureExport struct {
	TenantID  string // the customer's Entra tenant: the token for the platform's application is requested there
	Account   string // storage account name
	Container string
	Dir       string // export directory, no leading or trailing slash
}

// ParseAzureExportRef parses and validates "azblob://<tenant-id>@<account>/<container>/<directory>".
func ParseAzureExportRef(ref string) (AzureExport, error) {
	bad := func(why string) (AzureExport, error) {
		return AzureExport{}, fmt.Errorf("%w: %s", ErrInvalidConnection, why)
	}
	rest, ok := strings.CutPrefix(ref, "azblob://")
	if !ok {
		return bad("credential_ref must look like azblob://<tenant-id>@<storage-account>/<container>/<export directory>")
	}
	tenant, rest, ok := strings.Cut(rest, "@")
	if !ok {
		return bad("credential_ref must look like azblob://<tenant-id>@<storage-account>/<container>/<export directory>")
	}
	parts := strings.Split(rest, "/")
	if len(parts) < 3 {
		return bad("credential_ref must name the storage account, the container and the export directory")
	}
	e := AzureExport{TenantID: strings.ToLower(tenant), Account: parts[0], Container: parts[1], Dir: strings.Join(parts[2:], "/")}
	switch {
	case !azureGUIDRe.MatchString(e.TenantID):
		return bad("the Azure tenant id must be a GUID")
	case !azureAccountRe.MatchString(e.Account):
		return bad("invalid storage account name (3-24 lowercase letters and digits)")
	case !azureContainerRe.MatchString(e.Container):
		return bad("invalid container name (3-63 lowercase letters, digits and single hyphens)")
	case len(parts)-2 > 6 || len(e.Dir) > 300:
		return bad("the export directory is too deep or too long")
	}
	for _, seg := range parts[2:] {
		if seg == "" || seg == "." || seg == ".." || !exportPartRe.MatchString(seg) {
			return bad("invalid export directory")
		}
	}
	return e, nil
}

// AzureOwnershipKey is the container metadata key whose value "1" proves the customer controls the container.
// Metadata names must be identifiers, so the connection's token loses its hyphen.
func AzureOwnershipKey(externalID string) string { return strings.ReplaceAll(externalID, "-", "_") }

// validateAzure checks an Azure connection: the subscription and the export location.
func (c Connection) validateAzure() error {
	if !azureGUIDRe.MatchString(c.AccountRef) {
		return fmt.Errorf("%w: the Azure subscription id must be a GUID", ErrInvalidConnection)
	}
	_, err := ParseAzureExportRef(c.CredentialRef)
	return err
}
