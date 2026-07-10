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
