---
page_title: "Upgrade Guide: v1.10.0"
---

# Upgrade Guide v1.10.0

## Before Upgrading

Review the [changelog](changelog.md) to understand what has changed and what might cause an issue when upgrading the
provider.

~> **Note:** If you are upgrading across multiple minor versions, review the upgrade guide for each intermediate version
as well. Each guide documents breaking changes and migration steps specific to that release.

## How to Upgrade

### If you are already using the `rubrikinc/rubrik` provider

Make sure that the `version` field is configured in a way which allows Terraform to upgrade to the v1.10.0 release. One
way of doing this is by using the pessimistic constraint operator `~>`, which allows Terraform to upgrade to the latest
release within the same minor version:
```terraform
terraform {
  required_providers {
    rubrik = {
      source  = "rubrikinc/rubrik"
      version = "~> 1.10.0"
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
instructions. Otherwise, refresh the state to the v1.10.0 version:
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
      version = "~> 1.10.0"
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
      version = "~> 1.10.0"
    }
  }
}
```
If your configuration contains an explicit `provider "polaris" {}` block, rename it to `provider "rubrik" {}`.

The following resources support state migration via Terraform's `moved {}` block:

* `polaris_aws_account_managed` → `rubrik_aws_account_managed`
* `polaris_aws_account_managed_stack` → `rubrik_aws_account_managed_stack`
* `polaris_aws_cnp_account` → `rubrik_aws_cnp_account`
* `polaris_aws_cnp_account_attachments` → `rubrik_aws_cnp_account_attachments`
* `polaris_aws_custom_tags` → `rubrik_aws_custom_tags`
* `polaris_azure_custom_tags` → `rubrik_azure_custom_tags`
* `polaris_azure_devops_organization` → `rubrik_azure_devops_organization`
* `polaris_custom_role` → `rubrik_custom_role`
* `polaris_gcp_cloud_cluster` → `rubrik_gcp_cloud_cluster`
* `polaris_gcp_custom_labels` → `rubrik_gcp_custom_labels`
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
This will record the renames (Option 2) in state and migrate the local Terraform state to the v1.10.0 version.

## New Features

### Azure SQL Managed Instance Credentials

The new `rubrik_azure_sql_managed_instance_credentials` resource configures the credentials RSC uses to back up an
Azure SQL Managed Instance server. Look the server up with the `rubrik_object` data source, which now supports the
`AzureSqlManagedInstanceServer` object type and reports the authentication mechanisms the server supports as
`auth_type`.

Which credentials RSC needs depends on `auth_type` and on whether the RSC setup script has already been run against the
managed instance:

* `setup_script_installed = false`, the default — RSC connects to the managed instance using the `sql_credentials`
  block and creates the user it backs up as. The credentials are an administrator login with permission to do so, used
  only for the setup and not stored by RSC. Applies when `auth_type` is `SQL_AUTH_ONLY` or `SQL_AUTH_AND_AAD`.
* `setup_script_installed = true` and `auth_type` is `SQL_AUTH_ONLY` — the script has already created the backup user,
  so `sql_credentials` is that user's own login and must match the login and password the script was run with.
* `setup_script_installed = true` and `auth_type` is `SQL_AUTH_AND_AAD` or `AAD_ONLY` — RSC authenticates using
  Microsoft Entra ID, so no credentials are sent at all and the `sql_credentials` block is left out entirely.

```terraform
data "rubrik_object" "sql_mi" {
  name        = "my-sql-managed-instance"
  object_type = "AzureSqlManagedInstanceServer"
}

resource "rubrik_azure_sql_managed_instance_credentials" "creds" {
  server_id = data.rubrik_object.sql_mi.id

  sql_credentials {
    sql_username = var.sql_username
    sql_password = var.sql_password
  }

  sql_credential_version = "1"
}
```

~> **Note:** `sql_credentials` is write-only, so the values never reach Terraform state and changing them produces no
difference in the plan on their own. Change `sql_credential_version` to send them again, for example after rotating the
password. The two fields are required together. Write-only arguments require Terraform v1.11.0 or later.

### Azure Database for PostgreSQL Flexible Servers

Protection of Azure Database for PostgreSQL flexible servers is supported end to end, across four objects:

1. The `rubrik_azure_permissions` data source supports the `AZURE_POSTGRES_FLEXIBLE_SERVER_PROTECTION` feature, with
   the `BASIC` and `RECOVERY` permission groups.
2. The `rubrik_azure_subscription` resource has a new `postgres_flexible_server_protection` feature block.
3. The `rubrik_object` data source supports the `AzurePostgresFlexibleServer` object type.
4. The `rubrik_sla_domain` resource supports the `AZURE_POSTGRES_FLEXIBLE_SERVER_OBJECT_TYPE` object type and a new
   `azure_postgres_flexible_server_config` block.

Unlike the other Azure features, `postgres_flexible_server_protection` requires both an Azure resource group and a
user-assigned managed identity, so those fields are mandatory. RSC assigns the identity to the temporary and recovery
flexible servers it creates, and requires it to be in the feature's own resource group — a configuration where
`user_assigned_managed_identity_resource_group_name` does not match `resource_group_name` is rejected during plan.
Create the identity out of band, for example with the `azurerm_user_assigned_identity` resource.

```terraform
data "rubrik_azure_permissions" "postgres" {
  feature           = "AZURE_POSTGRES_FLEXIBLE_SERVER_PROTECTION"
  permission_groups = ["BASIC", "RECOVERY"]
}

resource "rubrik_azure_subscription" "subscription" {
  subscription_id = "31be1bb0-c76c-11eb-9217-afdffe83a002"
  tenant_domain   = "my-domain.onmicrosoft.com"

  postgres_flexible_server_protection {
    permissions           = data.rubrik_azure_permissions.postgres.id
    permission_groups     = data.rubrik_azure_permissions.postgres.permission_groups
    resource_group_name   = "my-postgres-rg"
    resource_group_region = "eastus2"

    regions = [
      "eastus2",
    ]

    user_assigned_managed_identity_name                = azurerm_user_assigned_identity.postgres.name
    user_assigned_managed_identity_principal_id        = azurerm_user_assigned_identity.postgres.principal_id
    user_assigned_managed_identity_region              = "eastus2"
    user_assigned_managed_identity_resource_group_name = "my-postgres-rg"
  }
}
```

An SLA Domain protecting flexible servers uses the object type on its own — it cannot be combined with any other
object type — stores its backup location in `backup_location` rather than the `archival` block, and does not support
replication. The optional `azure_postgres_flexible_server_config` block sets the point-in-time restore retention,
between 7 and 35 days, that RSC enforces on the source server. Omit the block to leave the server's existing
Azure-side retention untouched.

```terraform
resource "rubrik_sla_domain" "postgres" {
  name         = "postgres-flexible-server"
  object_types = ["AZURE_POSTGRES_FLEXIBLE_SERVER_OBJECT_TYPE"]

  hourly_schedule {
    frequency      = 1
    retention      = 1
    retention_unit = "DAYS"
  }

  azure_postgres_flexible_server_config {
    backup_retention_in_days = 7
  }

  backup_location {
    archival_group_id = data.rubrik_azure_archival_location.archival_location.id
  }
}
```

-> **Note:** RSC returns only the name and the principal ID of a user-assigned managed identity, not its region or its
resource group name. Those two fields keep whatever is already in state, so drift in them is not detected and they are
left empty after an import.

### Google Cloud SQL

The `CLOUD_SQL_PROTECTION` feature enables backup and in-place restore of Google Cloud SQL instances. It is supported
by the `rubrik_gcp_permissions` data source and the `rubrik_gcp_project` resource, and has the `BASIC` and
`EXPORT_AND_RESTORE` permission groups.

RSC runs Cloud SQL archival and archived recovery on Exocompute, so the `EXOCOMPUTE` feature additionally needs the new
`CLOUDSQL` permission group. It grants the Private Service Access networking permissions and the temporary Cloud SQL
instance permissions those operations use. When Exocompute uses a VPC network in a shared VPC host project, add the
same permission group to the `GCP_SHARED_VPC_HOST` feature of the host project as well.

```terraform
data "rubrik_gcp_permissions" "cloud_sql" {
  feature           = "CLOUD_SQL_PROTECTION"
  permission_groups = ["BASIC", "EXPORT_AND_RESTORE"]
}

data "rubrik_gcp_permissions" "exocompute" {
  feature           = "EXOCOMPUTE"
  permission_groups = ["BASIC", "CLOUDSQL"]
}

resource "rubrik_gcp_project" "project" {
  project        = "my-project"
  project_name   = "My Project"
  project_number = 123456789012

  feature {
    name              = "CLOUD_SQL_PROTECTION"
    permission_groups = ["BASIC", "EXPORT_AND_RESTORE"]
    permissions       = data.rubrik_gcp_permissions.cloud_sql.id
  }

  feature {
    name              = "EXOCOMPUTE"
    permission_groups = ["BASIC", "CLOUDSQL"]
    permissions       = data.rubrik_gcp_permissions.exocompute.id
  }
}
```

~> **Note:** Cloud SQL protection must be enabled for the RSC account before the `CLOUDSQL` permission group can be
used, otherwise RSC rejects the permission group.

### Data Security Policies

The new `rubrik_data_security_policy` resource creates and manages data security policies in RSC, and the matching
`rubrik_data_security_policy` data source looks one up by `name` or by `policy_id`.

A policy matches on up to two groups of conditions: object conditions in the `object_filter` block, and identity
conditions in the `identity_filter` block. At least one of the two is required. Conditions within a block are joined by
that block's `op` field, and the two blocks are always joined by AND. This is the only filter structure RSC accepts,
and it mirrors the RSC data security policy editor. Which block a condition belongs to follows from its `filter_type`:
`SECURITY_DOCUMENT_*` and `SECURITY_SNAPPABLE_*` are object conditions, `SECURITY_IDENTITY_*` and `SECURITY_GPO_*` are
identity conditions.

```terraform
resource "rubrik_data_security_policy" "overexposed_sensitive_data" {
  name        = "Overexposed Sensitive Data"
  description = "Highly sensitive documents without backup protection"
  category    = "OVEREXPOSED"
  severity    = "CRITICAL"

  object_filter {
    op = "AND"

    condition {
      filter_type  = "SECURITY_DOCUMENT_SENSITIVITY"
      values       = ["HIGH", "MEDIUM"]
      relationship = "IS"
    }

    condition {
      filter_type  = "SECURITY_SNAPPABLE_BACKUP"
      values       = ["Unprotected"]
      relationship = "IS"
    }
  }
}
```

`category` is one of `MISPLACED`, `OVEREXPOSED`, `REDUNDANT` and `UNPROTECTED`, and `severity` one of `LOW`, `MEDIUM`,
`HIGH` and `CRITICAL`. The optional `threshold_filter` block holds a single condition deciding how many matches raise a
violation, typically a `SECURITY_DOCUMENT_HIT_COUNT` condition.

### Self-Serve Rolling Upgrade

The new `rubrik_self_serve_rolling_upgrade` resource manages the account-wide self-serve rolling upgrade setting in
RSC. The setting is a singleton, so only one instance of the resource is meaningful per RSC tenant.

```terraform
resource "rubrik_self_serve_rolling_upgrade" "account" {
  enabled = true
}
```

Because the setting is account-wide rather than an object with an ID of its own, the import ID is ignored — importing
the resource works with any value.

### S3 Recovery Permission Groups

The `CLOUD_NATIVE_S3_PROTECTION` feature gains the `EXPORT` and `RECOVERY` permission groups in the
`rubrik_aws_account` and `rubrik_aws_cnp_account` resources, and in the `rubrik_aws_cnp_artifacts` and
`rubrik_aws_cnp_permissions` data sources. `RECOVERY` grants the AWS permissions required to write objects back into an
existing bucket, and `EXPORT` those required to export an S3 recovery to a newly created target bucket.

~> **Note:** Both permission groups require S3 recovery to be enabled for the RSC account. Use the
`rubrik_aws_permission_groups` data source to read the permission groups currently available for a feature.

### New `rubrik_object` Object Types

Besides `AzurePostgresFlexibleServer` and `AzureSqlManagedInstanceServer`, covered above, the `rubrik_object` data
source supports the `CloudNativeTagRule` object type, resolving a cloud native tag rule to its RSC ID by name for use
with the `rubrik_sla_domain_assignment` resource.

The data source also gains an `auth_type` attribute, reporting the authentication mechanisms an
`AzureSqlManagedInstanceServer` supports. It is null for every other object type, which is why it is only useful with
the `rubrik_azure_sql_managed_instance_credentials` resource.

## Significant Changes

### The `timeouts` block in `rubrik_object` is now a nested attribute

The optional `timeouts` block in the `rubrik_object` data source is now a nested attribute rather than a block. If you
set a custom read timeout, change the block syntax to an attribute assignment. This is a result of migrating the data
source to the Terraform Plugin Framework; lookups themselves behave the same.
```terraform
# Before
data "rubrik_object" "account" {
  name        = "my-account"
  object_type = "AwsNativeAccount"

  timeouts {
    read = "10m"
  }
}

# After
data "rubrik_object" "account" {
  name        = "my-account"
  object_type = "AwsNativeAccount"

  timeouts = {
    read = "10m"
  }
}
```
Configurations that do not set a `timeouts` block are unaffected.

### `rubrik_object` validate optional attributes at plan time

In the `rubrik_object` data source, the `subscription_id`, `org_id` and `project_id` fields each apply only to specific
object types:

* `subscription_id` — `AzureNativeResourceGroup`
* `org_id` — `AzureDevOpsProject`, `AzureDevOpsRepository`, `GitHubRepository`
* `project_id` — `AzureDevOpsRepository`

Previously, setting one of these fields for any other `object_type` was silently ignored. The data source now validates
this at plan time and returns an error identifying the offending field. If your configuration set one of these fields
for an `object_type` it does not apply to, remove it; the field had no effect before, so removing it does not change the
resolved object.

In addition, `subscription_id` is no longer required when `object_type` is `AzureNativeResourceGroup`. A resource group
is now looked up by name alone; set `subscription_id` only to disambiguate a resource group name that is shared across
subscriptions. Existing configurations that set `subscription_id` continue to work unchanged.

### Custom tags resources can exclude tags from snapshots

The `rubrik_aws_custom_tags` and `rubrik_azure_custom_tags` resources have a new optional `excluded_tags` field, and the
`rubrik_gcp_custom_labels` resource a new optional `excluded_labels` field. Tags and labels whose key matches one of the
patterns are excluded from snapshots. A pattern is either an exact key or a prefix wildcard, such as `temp-*`.
```terraform
resource "rubrik_aws_custom_tags" "tags" {
  custom_tags = {
    "owner" = "backup-team"
  }

  excluded_tags = [
    "internal-cost-center",
    "temp-*",
  ]
}
```
The new fields follow the same ownership model as `custom_tags`: a resource manages only the patterns listed in its own
configuration and leaves any other patterns in RSC untouched. Patterns already configured in RSC, whether through the
UI or another resource, are not adopted, so existing configurations are unaffected and continue to plan clean.

To support managing exclusions on their own, `custom_tags` and `custom_labels` are now optional. When specified, they
must contain at least one tag or label, as must `excluded_tags` and `excluded_labels` — an empty collection is
rejected at plan time. Omit a field entirely rather than setting it to `{}` or `[]`. At least one of the two fields
must be specified, so a resource with neither is rejected.
```terraform
resource "rubrik_aws_custom_tags" "exclusions_only" {
  excluded_tags = [
    "internal-cost-center",
  ]
}
```

Import is the exception. As with custom tags, importing one of these resources takes ownership of every excluded tag
pattern configured for that scope, not only the ones you intend to manage.

### Custom tags resources can be scoped to a single cloud account

The three resources have a new optional `cloud_account_id` field holding an RSC cloud account ID. When omitted, the
custom tags and excluded tags apply to all cloud accounts of the cloud vendor, which is how the resources have always
behaved, so existing configurations are unaffected. When specified, they apply only to that cloud account. RSC calls
the two scopes global and granular, and keeps them as independent configurations — changing one does not affect the
other, and the same tag key can exist in both with different values.
```terraform
resource "rubrik_aws_custom_tags" "test_account" {
  cloud_account_id = rubrik_aws_account.test_account.id

  custom_tags = {
    "env" = "test"
  }
}
```
Changing `cloud_account_id` on an existing resource replaces it, removing the tags from the old scope before adding
them to the new one.

#### The import ID now identifies the scope

The import ID used to be ignored, and earlier releases told you to pass a dummy ID. It now selects which scope to
import. Pass the cloud account ID to import an account-scoped resource, or `global` to import the scope covering all
cloud accounts of the cloud vendor:
```shell
% terraform import rubrik_aws_custom_tags.account b6c0b4a2-1d3e-4f5a-8b7c-9d0e1f2a3b4c
% terraform import rubrik_aws_custom_tags.global global
```
Any other import ID is now rejected. This is deliberate: were a malformed cloud account ID accepted, the import would
silently fall back to the global scope and take ownership of every custom tag and excluded tag in it.

If you have an `import {}` block still in your configuration with `id = "dummy"`, change it to `id = "global"`.
Nothing else needs to change — the import ID is not recorded in state, so a `terraform import` completed against an
earlier release is unaffected.

### The `rubrik_sla_domain` resource rejects `backup_location` for unsupported object types

The `backup_location` block in the `rubrik_sla_domain` resource is now only accepted for the object types which have a
backup location:

* `AWS_S3_OBJECT_TYPE`
* `AZURE_POSTGRES_FLEXIBLE_SERVER_OBJECT_TYPE`
* `AZURE_SQL_DATABASE_OBJECT_TYPE` and `AZURE_SQL_MANAGED_INSTANCE_OBJECT_TYPE`, when the `CNP_AZURE_SQL_SLA_REVAMP`
  feature is enabled for the RSC account

Previously the block was sent as an AWS S3 configuration whatever the object type, and RSC ignored it for anything but
an AWS S3 SLA Domain. Setting it for any other object type now fails with `backup_location is not supported by the
configured object types`.

The error is raised during apply rather than plan, so a configuration carrying a stray `backup_location` block still
plans clean. Remove the block from any SLA Domain whose `object_types` are not in the list above — it had no effect
before, so removing it does not change the SLA Domain.

Note that Azure SQL Database and Azure SQL Managed Instance SLA Domains are only in the list when the
`CNP_AZURE_SQL_SLA_REVAMP` feature is enabled for the RSC account. Without the feature they carry their archival
location in the `archival` block, and a `backup_location` block is now rejected rather than silently sent as an AWS S3
configuration.

### Security group fields in the AWS Exocompute resource are deprecated

The `cluster_security_group_id` and `node_security_group_id` fields in the `rubrik_aws_exocompute` resource are
deprecated. RSC now always creates and manages the security groups for RSC managed Exocompute configurations, and a
future RSC release will reject configurations that supply them.

RSC scopes its security group permissions on the name and tags of the security group it creates, notably the
`rk_managed` tag. It cannot apply that tag to a security group you created without holding `CreateTags` on every
security group in the account, so customer-supplied groups can fail with an authorization error during some
operations.

Setting either field still works in this release and produces a deprecation warning. To resolve the warning, remove
both fields and let RSC create the security groups:
```terraform
# Before
resource "rubrik_aws_exocompute" "host" {
  account_id                = data.rubrik_aws_account.host.id
  cluster_security_group_id = "sg-005656347687b8170"
  node_security_group_id    = "sg-00e147656785d7e2f"
  region                    = "us-east-2"
  vpc_id                    = "vpc-4859acb9"

  subnets = [
    "subnet-ea67b67b",
    "subnet-ea43ec78"
  ]
}

# After
resource "rubrik_aws_exocompute" "host" {
  account_id = data.rubrik_aws_account.host.id
  region     = "us-east-2"
  vpc_id     = "vpc-4859acb9"

  subnets = [
    "subnet-ea67b67b",
    "subnet-ea43ec78"
  ]
}
```
Run `terraform plan` before applying the change and read the plan. Both fields are marked `ForceNew`, so if the plan
does show a change to either of them it replaces the Exocompute configuration, which tears down and redeploys the
Exocompute cluster. Treat a replacement in the plan as a maintenance operation rather than applying it straight away.

Leaving the fields in place is the riskier option over time. Once RSC manages the security groups for a configuration,
a configuration that still supplies security group IDs differs from what RSC reports for it, and because both fields
force a new resource that difference is planned as a replacement of the Exocompute configuration.

Customer managed Exocompute — where you attach your own EKS cluster with the
`rubrik_aws_exocompute_cluster_attachment` resource — never used these fields and is unaffected.

### `CLOUD_COST_REPORT` is no longer tracked on AWS IAM roles accounts

RSC enables the `CLOUD_COST_REPORT` feature on its own for any AWS account carrying a workload feature which accrues
AWS spend, whatever feature set was passed when onboarding. It cannot be declared in the `feature` block of the
`rubrik_aws_cnp_account` resource, so tracking it produced a persistent diff removing it. Applying that diff silently
disabled cost reporting for the account, and RSC added the feature back the next time the features were onboarded.

Only the features which can be declared are tracked now. Expect a one-time change on upgrade for accounts where RSC
enabled cost reporting: the feature drops out of state on the first refresh, and the diff removing it goes with it.
Nothing needs to change in the configuration, and cost reporting in RSC is left alone.

The same filtering applies to the `features` field of the `rubrik_aws_cnp_account_attachments` resource, to imports of
both resources, and to the configuration generated by the `rubrik_aws_cnp_account` and
`rubrik_aws_cnp_account_attachments` list resources.

Destroying a `rubrik_aws_cnp_account` now removes cost reporting explicitly. RSC removes an account once its last
feature is removed, but it never removes `CLOUD_COST_REPORT` along with the features it was enabled for, so removing
only the declared features could leave the feature — and with it the account — behind in RSC.
