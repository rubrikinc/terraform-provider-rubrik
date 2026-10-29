---
page_title: "Upgrade Guide: v1.13.0"
---

# Upgrade Guide v1.13.0

## Before Upgrading

Review the [changelog](changelog.md) to understand what has changed and what might cause an issue when upgrading the
provider.

~> **Note:** If you are upgrading across multiple minor versions, review the upgrade guide for each intermediate version
as well. Each guide documents breaking changes and migration steps specific to that release.

## How to Upgrade

### If you are already using the `rubrikinc/rubrik` provider

Make sure that the `version` field is configured in a way which allows Terraform to upgrade to the v1.13.0 release. One
way of doing this is by using the pessimistic constraint operator `~>`, which allows Terraform to upgrade to the latest
release within the same minor version:
```terraform
terraform {
  required_providers {
    rubrik = {
      source  = "rubrikinc/rubrik"
      version = "~> 1.13.0"
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
If you get an error or an unwanted diff, please see the _New Features_ section below for additional instructions.
Otherwise, refresh the state to the v1.13.0 version:
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
      version = "~> 1.13.0"
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
      version = "~> 1.13.0"
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
unwanted diff, see the _New Features_ section below for additional context. Otherwise, proceed by running:
```shell
% terraform apply
```
This will record the renames (Option 2) in state and migrate the local Terraform state to the v1.13.0 version.

## New Features

### AWS accounts using multiple roles

RSC can onboard an AWS account using the IAM roles workflow with one IAM role per feature, instead of a single
`CROSSACCOUNT` role carrying the permissions of every feature. Which role mode an account uses is decided by RSC, and
fixed when the roles of the account are registered with the `rubrik_aws_cnp_account_attachments` resource. The
`trust_policies` field of the `rubrik_aws_cnp_account` resource always holds the trust policies of the role mode of the
account.

The `rubrik_aws_cnp_artifacts` and `rubrik_aws_cnp_permissions` data sources return the roles and policies of an account
using a single role, unless they are given the AWS account ID of the account with the new `native_id` field. With
`native_id`, the data sources return the roles and policies of the role mode of the account, matching its trust
policies. For an account using multiple roles, that is one role per feature, with the policies of each feature on the
role of that feature. For an account onboarded with a single role, it is the `CROSSACCOUNT` role, as before. The
`native_id` field can be given before the account has been onboarded, in which case the data sources return the roles
and policies RSC uses for a new account.

Pass the AWS account ID to the data sources as a value known when planning, e.g. a variable, and not as a reference to
the `rubrik_aws_cnp_account` resource. The role keys of the artifacts data source are then known when planning, so a new
account is onboarded with a single `terraform apply`:
```terraform
variable "account_id" {
  description = "AWS account ID."
  type        = string
}

locals {
  features = {
    CLOUD_DISCOVERY         = ["BASIC"]
    CLOUD_NATIVE_PROTECTION = ["BASIC"]
  }
}

data "rubrik_aws_cnp_artifacts" "artifacts" {
  native_id = var.account_id

  dynamic "feature" {
    for_each = local.features
    content {
      name              = feature.key
      permission_groups = feature.value
    }
  }
}

data "rubrik_aws_cnp_permissions" "permissions" {
  for_each  = data.rubrik_aws_cnp_artifacts.artifacts.role_keys
  native_id = var.account_id
  role_key  = each.key

  dynamic "feature" {
    for_each = local.features
    content {
      name              = feature.key
      permission_groups = feature.value
    }
  }
}

resource "rubrik_aws_cnp_account" "account" {
  name      = "My Account"
  native_id = var.account_id
  regions   = ["us-east-2"]

  dynamic "feature" {
    for_each = local.features
    content {
      name              = feature.key
      permission_groups = feature.value
    }
  }
}
```
Referencing `rubrik_aws_cnp_account.account.native_id` from the data sources instead makes their role keys unknown until
the account exists, and Terraform then cannot plan the roles of a new account.

An account using multiple roles has a role for every feature, so role key names are longer than with a single role. A
role name prefix built from the full role key, e.g. `"rubrik-${lower(each.key)}-"`, is longer than the 38 characters
AWS allows for `CLOUD_NATIVE_DYNAMODB_PROTECTION` and `CLOUD_NATIVE_UEM_KEY_MANAGEMENT`. Shorten the prefix, e.g.
`"${substr("rubrik-${lower(each.key)}", 0, 37)}-"`, which only truncates the role keys exceeding the limit. Role keys
within the limit keep the same prefix, so existing roles are not replaced.

Without `native_id`, the data sources behave as before. An account using multiple roles then gets trust policies for
one role per feature, but roles and policies for a single `CROSSACCOUNT` role, so the roles built from them do not
match.
