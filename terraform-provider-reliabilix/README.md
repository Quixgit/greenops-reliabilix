# Terraform provider for Reliabilix

Manages, as code: projects, their data-residency and threshold policies, and budgets. Version follows the platform (`VERSIONS.md`, `tool/terraform-provider`).

```hcl
terraform {
  required_providers {
    reliabilix = { source = "reliabilix/reliabilix" }
  }
}

provider "reliabilix" {
  endpoint = "https://api.example.com" # or RELIABILIX_ENDPOINT
  api_key  = var.reliabilix_api_key    # or RELIABILIX_API_KEY
}

resource "reliabilix_project" "web" {
  name            = "web"
  functional_unit = "api_request"
}

resource "reliabilix_project_policy" "web" {
  project_id              = reliabilix_project.web.id
  allowed_regions         = ["eu-north-1", "eu-west-1"]
  ci_carbon_block_kg      = 20
  alert_carbon_monthly_kg = 500
}

resource "reliabilix_budget" "web" {
  project_id = reliabilix_project.web.id
  amount     = 1500
  currency   = "USD"
  period     = "monthly"
}
```

## The key

Settings > API keys > access **Terraform (projects and budgets)** (role `iac`, shown once). It reads and writes projects, policies and budgets and nothing
else: it cannot touch cloud connections, members, keys, settings, notifications or automation, and cannot approve anything.

## What to know

- The API has no update for projects and budgets, so changing their attributes **replaces** them. A project that has cloud connections or data cannot be deleted
  (the apply fails with the reason).
- `reliabilix_project_policy` never changes which recommendation types may be applied automatically; that list stays as it is in the console.
  Deleting the resource clears the regions and thresholds, not the project.
- Changes made in the console are noticed at the next plan (drift) and put back at the next apply.
- Import: `terraform import reliabilix_project.web <project id>`, `reliabilix_project_policy.web <project id>`, `reliabilix_budget.x <budget id>`.
- Data source `reliabilix_project` finds a project made elsewhere by name.

## Build and try

```bash
go build -o ./bin/terraform-provider-reliabilix .
# ~/.terraformrc: provider_installation { dev_overrides { "reliabilix/reliabilix" = "<path to ./bin>" } direct {} }
```

Not published to a registry yet; the dev override above is the way to use it.
