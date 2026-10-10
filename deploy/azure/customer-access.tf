# Give this to a customer who manages Azure with Terraform. It does what Reliabilix needs on the Cost Management
# export container, and nothing else:
#   1. the platform application's service principal exists in the customer's tenant
#   2. container metadata that proves the customer controls the container (the Azure counterpart of the AWS
#      ExternalId and the GCP dataset label)
#   3. read-only access (Storage Blob Data Reader) for that service principal on the container only
#
# Prerequisites (not created here): a Cost Management export of type "Cost and usage details (FOCUS)", version 1.0,
# CSV, writes into this container. Data is exported from the day the export is created (use "Export all data" for
# history), and the first delivery can take up to 24 hours.

variable "storage_account_name" { type = string }
variable "container_name" { type = string } # the container that receives the export

# Both values are shown by Reliabilix when you create the connection.
variable "external_id" {
  type        = string
  description = "The connection's ownership token (looks like rlx-<40 hex>). Hyphens become underscores in the metadata key."
}
variable "platform_application_id" {
  type        = string
  description = "The Reliabilix application (client) id."
}

data "azurerm_storage_account" "export" {
  name                = var.storage_account_name
  resource_group_name = var.resource_group_name
}
variable "resource_group_name" { type = string }

# The platform application is multi-tenant: this creates its service principal in your tenant (no secret is shared).
resource "azuread_service_principal" "reliabilix" {
  client_id = var.platform_application_id
}

# Read-only, on the one container. If the container is managed elsewhere in Terraform, add the metadata entry to
# that resource instead and keep only the role assignment here.
resource "azurerm_role_assignment" "reliabilix_reader" {
  scope                = "${data.azurerm_storage_account.export.id}/blobServices/default/containers/${var.container_name}"
  role_definition_name = "Storage Blob Data Reader"
  principal_id         = azuread_service_principal.reliabilix.object_id
}

# The ownership proof. az equivalent:
#   az storage container metadata update --account-name <account> --name <container> \
#     --metadata <external_id with - replaced by _>=1 --auth-mode login
# Terraform: metadata = { "<key>" = "1" } on the azurerm_storage_container resource that owns the container.
output "metadata_to_add" {
  value = { (replace(var.external_id, "-", "_")) = "1" }
}
