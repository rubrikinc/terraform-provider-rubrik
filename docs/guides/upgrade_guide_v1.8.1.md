---
page_title: "Upgrade Guide: v1.8.1"
---

# Upgrade Guide v1.8.1

The v1.8.1 release changes how the AWS IAM roles workflow surfaces the artifact for a role-chaining account: the
role-chaining role is now exposed under the `ROLE_CHAINING` artifact key instead of `CROSSACCOUNT`. The release also
adds support for the GCP `SERVERS_AND_APPS` feature and fixes a bug in the `rubrik_aws_cnp_account_attachments`
resource. See the [changelog](changelog.md) for the full list.

## Before Upgrading

Review the [changelog](changelog.md) to understand what has changed and what might cause an issue when upgrading the
provider.

~> **Note:** If you are upgrading across multiple minor versions, review the upgrade guide for each intermediate version
as well. Each guide documents breaking changes and migration steps specific to that release.

## How to Upgrade

### If you are already using the `rubrikinc/rubrik` provider

Make sure that the `version` field is configured in a way which allows Terraform to upgrade to the v1.8.1 release. One
way of doing this is by using the pessimistic constraint operator `~>`, which allows Terraform to upgrade to the latest
release within the same minor version:
```terraform
terraform {
  required_providers {
    rubrik = {
      source  = "rubrikinc/rubrik"
      version = "~> 1.8.1"
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
instructions. Otherwise, refresh the state to the v1.8.1 version:
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
      version = "~> 1.8.1"
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
      version = "~> 1.8.1"
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
source) unless you onboard role-chaining accounts — see [Role Chaining Artifact Key](#role-chaining-artifact-key). For
Option 2, the plan should show the moved resources with no other changes. If you get an error or an unwanted diff, see
the _Significant Changes_ section below for additional context. Otherwise, proceed by running:
```shell
% terraform apply
```
This will record the renames (Option 2) in state and migrate the local Terraform state to the v1.8.1 version.

## Significant Changes

### AWS Role Chaining Artifact Key

Earlier releases surfaced the role-chaining role under the `CROSSACCOUNT` artifact key. It is now surfaced under the
`ROLE_CHAINING` key, which matches the account's feature. This affects only role-chaining accounts, that is accounts
whose sole feature is `ROLE_CHAINING`; other accounts are unchanged.

After upgrading, a role-chaining account shows a one-time diff that settles after a single `apply`. The
`rubrik_aws_cnp_artifacts` data source returns `ROLE_CHAINING` in `role_keys` where it previously returned
`CROSSACCOUNT`, and the `rubrik_aws_cnp_permissions` data source reports the policy under the `ROLE_CHAINING` artifact
with the policy name `RoleChainingPolicy` instead of `RoleChaining`. The permission policy document itself does not
change — it is still an `sts:AssumeRole` policy with the same statements, and the role's trust relationship is also
unchanged. Only the artifact key and the reported policy name differ.

The diff matters because the [AWS IAM roles workflow](aws_cnp_account.md) guide keys its AWS resources on these values:
`aws_iam_role` on `role_keys` and `aws_iam_policy` on the policy name. When those `for_each` keys change, Terraform
destroys and recreates the role-chaining `aws_iam_role` and its `aws_iam_policy` under new ARNs, re-attaches them, and
`rubrik_aws_cnp_account_attachments` re-registers the role under the `ROLE_CHAINING` key with the recreated role's ARN.
The new role carries the same trust relationship and permissions as the old one.

No configuration change is required if your configuration iterates the artifacts and permissions data sources, as in
that guide — the data sources now emit `ROLE_CHAINING` automatically and the role keys follow. Review the plan, then
run `terraform apply` to let the diff settle.

A configuration change is only required where the `CROSSACCOUNT` key is hardcoded, for example as the `role_key` of a
`rubrik_aws_cnp_permissions` data source. Update those references to `ROLE_CHAINING`:

```terraform
data "rubrik_aws_cnp_permissions" "role_chaining" {
  role_key = "ROLE_CHAINING"

  feature {
    name              = "ROLE_CHAINING"
    permission_groups = ["BASIC"]
  }
}
```
