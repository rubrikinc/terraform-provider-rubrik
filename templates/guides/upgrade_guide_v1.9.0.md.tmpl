---
page_title: "Upgrade Guide: v1.9.0"
---

# Upgrade Guide v1.9.0

## Before Upgrading

Review the [changelog](changelog.md) to understand what has changed and what might cause an issue when upgrading the
provider.

~> **Note:** If you are upgrading across multiple minor versions, review the upgrade guide for each intermediate version
as well. Each guide documents breaking changes and migration steps specific to that release.

## How to Upgrade

### If you are already using the `rubrikinc/rubrik` provider

Make sure that the `version` field is configured in a way which allows Terraform to upgrade to the v1.9.0 release. One
way of doing this is by using the pessimistic constraint operator `~>`, which allows Terraform to upgrade to the latest
release within the same minor version:
```terraform
terraform {
  required_providers {
    rubrik = {
      source  = "rubrikinc/rubrik"
      version = "~> 1.9.0"
    }
  }
}
```
Then upgrade the provider by running:
```shell
% terraform init -upgrade
```
Validate the configuration:
```shell
% terraform plan
```
If you get an error or an unwanted diff, please see the _Significant Changes_ section below for additional
instructions. Otherwise, refresh the state to the v1.9.0 version:
```shell
% terraform apply -refresh-only
```
The rest of this section covers users coming from the `rubrikinc/polaris` provider and does not apply to you.

### If you are coming from the `rubrikinc/polaris` provider

There are two realistic upgrade paths. Pick the one that matches what your configuration uses today.

Note that migration is per-module, not per-resource. The local provider name in `required_providers` dictates the
prefix every resource and data source of that provider must use within the module: a module configured with the local
name `polaris` uses the `polaris` prefix throughout, and a module configured with `rubrik` uses the `rubrik` prefix
throughout. Mixing the two prefixes in a single module is not possible.

#### Option 1: Switch source to `rubrikinc/rubrik` but keep the `polaris` local name

This is the lowest-friction way to move to the renamed provider, and the recommended path for any module that contains
a resource which does not yet support the `moved {}` block (see Option 2 for the list of resources that do). Update
only the `source` field in `required_providers`, leaving the local provider name as `polaris`:
```terraform
terraform {
  required_providers {
    polaris = {
      source  = "rubrikinc/rubrik"
      version = "~> 1.9.0"
    }
  }
}
```
The renamed provider knows about both the `polaris` and `rubrik` resource and data source prefixes, so existing
configurations and state continue to work without changes. Terraform will emit a deprecation warning for each
`polaris` resource or data source you reference, but no state surgery is required.

#### Option 2: Switch source to `rubrikinc/rubrik` and change the local name to `rubrik`

This is the cleaner end state. It is realistic for modules that contain only data sources, only resources that support
`moved {}`, or other resources you are willing to remove from state and re-import. Update both the local name and the
source:
```terraform
terraform {
  required_providers {
    rubrik = {
      source  = "rubrikinc/rubrik"
      version = "~> 1.9.0"
    }
  }
}
```
If your configuration contains an explicit `provider "polaris" {}` block, rename it to `provider "rubrik" {}`.

The following resources support state migration via Terraform's `moved {}` block:

* `polaris_aws_cnp_account` → `rubrik_aws_cnp_account`
* `polaris_aws_cnp_account_attachments` → `rubrik_aws_cnp_account_attachments`
* `polaris_azure_devops_organization` → `rubrik_azure_devops_organization`
* `polaris_custom_role` → `rubrik_custom_role`
* `polaris_role_assignment` → `rubrik_role_assignment`
* `polaris_sso_group` → `rubrik_sso_group`
* `polaris_user` → `rubrik_user`

For each of these resources, rename the `resource` block to use the `rubrik` prefix and add a `moved {}` block
referencing the old and new Terraform addresses. For example, a `polaris_aws_cnp_account` resource named `account` would
become:
```terraform
moved {
  from = polaris_aws_cnp_account.account
  to   = rubrik_aws_cnp_account.account
}

resource "rubrik_aws_cnp_account" "account" {
  # ... existing configuration ...
}
```
Data sources do not have state, so they only need their prefix renamed. For example, a `polaris_aws_cnp_artifacts` data
source named `artifacts` would become:
```terraform
data "rubrik_aws_cnp_artifacts" "artifacts" {
  # ... existing configuration ...
}
```
Any other resource in the module must be removed from state and re-imported, or recreated. This is potentially
destructive — if you are not willing to do this for every such resource in the module, use Option 1 instead.

#### Applying the upgrade

Once you have updated the configuration for whichever option you chose, install the renamed provider by running:
```shell
% terraform init -upgrade
```
Then validate the configuration:
```shell
% terraform plan
```
For Option 1, the plan should show no changes (apart from deprecation warnings for each `polaris` resource and data
source). For Option 2, the plan should show the moved resources with no other changes. If you get an error or an
unwanted diff, see the _Significant Changes_ section below for additional context. Otherwise, proceed by running:
```shell
% terraform apply
```
This will record the renames (Option 2) in state and migrate the local Terraform state to the v1.9.0 version.

## Significant Changes

### Azure SQL Database and Managed Instance SLAs (feature-gated)

When the `CNP_AZURE_SQL_SLA_REVAMP` feature is enabled for your account, Azure SQL Database and Managed Instance SLAs
in the `rubrik_sla_domain` resource follow a new V1/V2 model:

* A **V1** (Azure-managed, long-term retention) SLA carries a new `ltr_config` block (weekly, monthly, and yearly
  retention) and takes no Rubrik snapshot schedule or backup location.
* A **V2** (Rubrik-managed) SLA omits `ltr_config` and specifies a Rubrik snapshot schedule together with a
  `backup_location` block.

~> **Note:** This behavior is controlled by the `CNP_AZURE_SQL_SLA_REVAMP` account-level feature flag, not by the
provider version — enabling it affects any provider version managing Azure SQL SLAs for that account. If the feature
is not enabled for your account, Azure SQL SLAs are unaffected and **no configuration changes are required**.

With the feature enabled, the way an Azure SQL SLA specifies its backup location changes:

* **Before:** an Azure SQL Database SLA required exactly one top-level `archival` block with instant archival enabled,
  and an Azure SQL Managed Instance SLA could not specify an archival location.
* **After:** a V2 Azure SQL SLA specifies its location with a top-level `backup_location` block (the same block used by
  AWS S3 multiple backup locations) and must not use the `archival` block.

If the feature is enabled and you have an existing Azure SQL Database SLA that uses the `archival` block, replace it
with a `backup_location` block:
```terraform
# Before
resource "rubrik_sla_domain" "azure_sql" {
  name         = "azure-sql"
  object_types = ["AZURE_SQL_DATABASE_OBJECT_TYPE"]

  hourly_schedule {
    frequency      = 1
    retention      = 1
    retention_unit = "DAYS"
  }

  azure_sql_database_config {
    log_retention = 7
  }

  archival {
    archival_location_id = data.rubrik_azure_archival_location.example.id
    threshold            = 0
  }
}

# After
resource "rubrik_sla_domain" "azure_sql" {
  name         = "azure-sql"
  object_types = ["AZURE_SQL_DATABASE_OBJECT_TYPE"]

  hourly_schedule {
    frequency      = 1
    retention      = 1
    retention_unit = "DAYS"
  }

  azure_sql_database_config {
    log_retention = 7
  }

  backup_location {
    archival_group_id = data.rubrik_azure_archival_location.example.id
  }
}
```

To manage Azure native long-term retention, configure a V1 SLA with `ltr_config` and no schedule or backup location:
```terraform
resource "rubrik_sla_domain" "azure_sql_v1" {
  name         = "azure-sql-v1"
  object_types = ["AZURE_SQL_DATABASE_OBJECT_TYPE"]

  azure_sql_database_config {
    log_retention = 7
    ltr_config {
      weekly_retention {
        retention      = 4
        retention_unit = "WEEKS"
      }
      monthly_retention {
        retention      = 12
        retention_unit = "MONTHS"
      }
      yearly_retention {
        retention      = 7
        retention_unit = "YEARS"
        week_of_year   = 1
      }
    }
  }
}
```

~> **Note:** An existing SLA cannot be switched between V1 (Azure-managed) and V2 (Rubrik-managed) in place — the
provider rejects a change that adds or removes `ltr_config` on an existing `rubrik_sla_domain`. To change the backup
type, create a new SLA Domain and reassign the affected databases to it. This matches the RSC UI, which disables the
backup-service selector when editing an existing SLA.

The release also adds a computed `backup_type` attribute (`NATIVE` for V1, `RUBRIK` for V2) and allows combining the
Azure SQL Database and Managed Instance object types in a single SLA.

## New Features

### Azure DevOps Onboarding

A new `rubrik_azure_devops_organization` resource onboards an Azure DevOps organization to RSC using a customer-supplied
application (non-OAuth). Onboarding has three steps that map to three Terraform objects:

1. Register the customer application for the Azure DevOps use case with a `rubrik_azure_service_principal` resource,
   setting the new `use_case = "AZURE_DEVOPS"` field.
2. Generate the onboarding scripts with the `rubrik_azure_devops_script` data source and run them against the
   organization out of band. The provider does not run the scripts — run each one with the Azure CLI signed in
   (`az login`) as a Project Collection Administrator in the organization; the script mints a short-lived Azure DevOps
   token from that session, so no personal access token is required.
3. Onboard the organization with the `rubrik_azure_devops_organization` resource.

```terraform
resource "rubrik_azure_service_principal" "devops" {
  app_id        = "25c2b42a-c76b-11eb-9767-6ff6b5b7e72b"
  app_name      = "My DevOps App"
  app_secret    = "<my-apps-secret>"
  tenant_domain = "mydomain.onmicrosoft.com"
  tenant_id     = "2bfdaef8-c76b-11eb-8d3d-4706c14a88f0"
  use_case      = "AZURE_DEVOPS"
}

data "rubrik_azure_devops_script" "onboard" {
  org_native_ids = ["my-org"]
  tenant_domain  = rubrik_azure_service_principal.devops.tenant_domain

  feature {
    name = "AZURE_DEVOPS_PROTECTION"
  }
  feature {
    name = "AZURE_DEVOPS_REPOSITORY_PROTECTION"
  }
}

resource "rubrik_azure_devops_organization" "org" {
  native_id            = "my-org"
  tenant_domain        = rubrik_azure_service_principal.devops.tenant_domain
  exocompute_host_type = "RUBRIK_HOST"
  exocompute_region    = "eastus"
  storage_type         = "RCV"

  feature {
    name = "AZURE_DEVOPS_PROTECTION"
  }
  feature {
    name = "AZURE_DEVOPS_REPOSITORY_PROTECTION"
  }

  depends_on = [rubrik_azure_service_principal.devops]
}
```

The `use_case` field on `rubrik_azure_service_principal` selects whether the application is registered for cloud native
protection (the default) or Azure DevOps. Credentials are stored separately per use case, so a tenant that uses both
declares one service principal per use case. Omitting the field preserves the existing cloud native protection behavior,
so existing service principal configurations are unaffected.

### Reading Azure DevOps Objects

Three new data sources read onboarded Azure DevOps objects by RSC ID: `rubrik_azure_devops_organization`,
`rubrik_azure_devops_project` and `rubrik_azure_devops_repository`.

The `rubrik_object` data source also gains support for the `AzureDevOpsOrganization`, `AzureDevOpsProject` and
`AzureDevOpsRepository` object types, resolving an object to its RSC ID by name for use with the
`rubrik_sla_domain_assignment` resource. Because project and repository names are only unique within their parent, set
the optional `org_id` (for a project) or `project_id` (for a repository) to disambiguate a name shared across parents:

```terraform
data "rubrik_object" "repo" {
  object_type = "AzureDevOpsRepository"
  name        = "my-repo"
  project_id  = data.rubrik_object.project.id
}
```

### Discovery and Bulk Import

A new `rubrik_azure_devops_organization` list resource lists onboarded Azure DevOps organizations, so you can discover
them with `terraform query` or bring existing organizations under management with an `import` block:

```terraform
variable "clouds" {
  type        = map(string)
  description = "Map of Azure DevOps organization native_id to cloud type (PUBLIC, CHINA or USGOV)."
  default     = {}
}

list "rubrik_azure_devops_organization" "all" {
  provider = rubrik
}

import {
  for_each = list.rubrik_azure_devops_organization.all.results
  to       = rubrik_azure_devops_organization.org[each.value.identity.id]
  identity = {
    id    = each.value.identity.id
    cloud = lookup(var.clouds, each.value.resource.native_id, "PUBLIC")
  }
}
```

RSC does not return the enabled `feature` blocks or the `cloud` type for onboarded organizations, so neither is
populated in list results. After generating configuration, set at least one `feature` block on each organization before
applying. The `cloud` type defaults to `PUBLIC` on import; for any non-public organization supply it in the import
`identity` block, e.g. with a `var.clouds` map keyed on the organization `native_id` as shown above. For details,
see the [rubrik_azure_devops_organization list resource documentation](../list-resources/azure_devops_organization.md).

### `moved {}` Block Support

The `rubrik_azure_devops_organization` resource supports Terraform's `moved {}` block. This enables in-place migration
from the deprecated `polaris_azure_devops_organization` resource type to the `rubrik_azure_devops_organization` resource
type without offboarding the organization from RSC and re-onboarding it.
