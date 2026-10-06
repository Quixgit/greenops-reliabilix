package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// GCP connections read the customer's Cloud Billing export in BigQuery. Everything the platform needs is a
// reference, never a credential:
//
//	account_ref    the GCP project id whose costs are ingested (a billing account export covers many projects)
//	credential_ref bq://<project>/<dataset>/<table> of the billing export table
//
// Ownership is proven with a dataset label (the GCP counterpart of the AWS ExternalId): the customer adds the
// label "<external_id>: 1" to the dataset, which only someone with write access to it can do. Without it a
// tenant could name another customer's dataset (the platform's service account may legitimately read it) and
// pull that customer's billing data: the confused-deputy problem.
var (
	gcpProjectRe = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`)
	bqNameRe     = regexp.MustCompile(`^[A-Za-z0-9_]{1,128}$`)
	// Cloud Billing writes exactly these table names; requiring the prefix keeps the platform from reading any
	// other table that happens to live in the same dataset.
	bqBillingTableRe = regexp.MustCompile(`^gcp_billing_export_(?:resource_)?v1_[A-Za-z0-9_]{1,100}$`)
)

// BigQueryTable locates the billing export table.
type BigQueryTable struct{ Project, Dataset, Table string }

// ParseBigQueryRef parses and validates "bq://<project>/<dataset>/<table>".
func ParseBigQueryRef(ref string) (BigQueryTable, error) {
	rest, ok := strings.CutPrefix(ref, "bq://")
	parts := strings.Split(rest, "/")
	if !ok || len(parts) != 3 {
		return BigQueryTable{}, fmt.Errorf("%w: credential_ref must look like bq://<project>/<dataset>/<table>", ErrInvalidConnection)
	}
	t := BigQueryTable{Project: parts[0], Dataset: parts[1], Table: parts[2]}
	switch {
	case !gcpProjectRe.MatchString(t.Project):
		return BigQueryTable{}, fmt.Errorf("%w: invalid BigQuery project id", ErrInvalidConnection)
	case !bqNameRe.MatchString(t.Dataset):
		return BigQueryTable{}, fmt.Errorf("%w: invalid BigQuery dataset name", ErrInvalidConnection)
	case !bqBillingTableRe.MatchString(t.Table):
		return BigQueryTable{}, fmt.Errorf("%w: the table must be a Cloud Billing export (gcp_billing_export_v1_... or gcp_billing_export_resource_v1_...)", ErrInvalidConnection)
	}
	return t, nil
}

// validateGCP checks a GCP connection: the cost project and the export table reference.
func (c Connection) validateGCP() error {
	if !gcpProjectRe.MatchString(c.AccountRef) {
		return fmt.Errorf("%w: GCP project id must be 6-30 lowercase letters, digits or hyphens", ErrInvalidConnection)
	}
	_, err := ParseBigQueryRef(c.CredentialRef)
	return err
}
