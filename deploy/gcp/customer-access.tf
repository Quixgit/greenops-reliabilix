# Give this to a customer who manages Google Cloud with Terraform. It does the two things Reliabilix needs on the
# billing export dataset, and nothing else:
#   1. a label that proves the customer controls the dataset (the GCP counterpart of the AWS ExternalId)
#   2. read-only access (BigQuery Data Viewer) for the platform's service account
#
# Prerequisites (not created here): the BigQuery API is enabled in the project, and Cloud Billing export to BigQuery
# (Standard usage cost) writes into this dataset. Data is exported from the day the export is enabled (no history),
# and the first delivery can take up to 48 hours.

variable "project_id" { type = string }
variable "dataset_id" { type = string } # the dataset that receives the billing export

# Both values are shown by Reliabilix when you create the connection.
variable "external_id" {
  type        = string
  description = "The connection's ownership token (looks like rlx-<40 hex>). It becomes the label KEY."
}
variable "platform_service_account" {
  type        = string
  description = "The Reliabilix service account email, e.g. platform@<project>.iam.gserviceaccount.com."
}

# Adds the label without touching the rest of the dataset: if the dataset is managed elsewhere in Terraform, add the
# label to that resource instead and skip this block.
resource "google_bigquery_dataset_access" "reliabilix_reader" {
  project    = var.project_id
  dataset_id = var.dataset_id
  role       = "roles/bigquery.dataViewer"
  user_by_email = var.platform_service_account
}

# The label itself (lower-case key and value as BigQuery requires).
# bq equivalent: bq update --set_label <external_id>:1 <project>:<dataset>
# Terraform: labels = { "<external_id>" = "1" } on the google_bigquery_dataset resource that owns the dataset.
output "label_to_add" {
  value = { (var.external_id) = "1" }
}
