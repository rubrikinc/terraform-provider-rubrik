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
