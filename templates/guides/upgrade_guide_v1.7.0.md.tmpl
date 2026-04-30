---
page_title: "Upgrade Guide: v1.7.0"
---

# Upgrade Guide v1.7.0

## Before Upgrading

Review the [changelog](changelog.md) to understand what has changed and what might cause an issue when upgrading the
provider.

The v1.7.0 release introduces the renamed `rubrikinc/rubrik` provider alongside the existing `rubrikinc/polaris`
provider. The `rubrikinc/polaris` provider will continue to be released and supported for some time, so there is no
need to switch right now. The `rubrikinc/polaris` provider will eventually be retired, however, and existing
configurations will need to migrate to `rubrikinc/rubrik` before then. The migration paths will improve over time as
more resources gain support for Terraform's `moved {}` block, making the switch progressively simpler.

If you choose to switch to the `rubrikinc/rubrik` provider now, the `polaris` prefixed resources and data sources
remain available as deprecated aliases of their `rubrik` counterparts so that existing configurations keep working
without changes.

## How to Upgrade

### If you are already using the `rubrikinc/rubrik` provider

This is a standard minor release upgrade. Make sure that the `version` field is configured in a way which allows
Terraform to upgrade to the v1.7.0 release. One way of doing this is by using the pessimistic constraint operator `~>`,
which allows Terraform to upgrade to the latest release within the same minor version:
```terraform
terraform {
  required_providers {
    rubrik = {
      source  = "rubrikinc/rubrik"
      version = "~> 1.7.0"
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
If you get an error or an unwanted diff, please see the _Significant Changes_ and _New Features_ sections below for
additional instructions. Otherwise, refresh the state to the v1.7.0 version:
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
      version = "~> 1.7.0"
    }
  }
}
```
The renamed provider knows about both the `polaris` and `rubrik` resource and data source prefixes, so existing
configurations and state continue to work without changes. Terraform will emit a deprecation warning for each
`polaris` resource or data source you reference, but no state surgery is required.

#### Option 2: Switch source to `rubrikinc/rubrik` and change the local name to `rubrik`

This is the cleaner end state. It is only realistic for modules that contain only data sources, only resources that
support `moved {}`, or other resources you are willing to remove from state and re-import. Update both the local name
and the source:
```terraform
terraform {
  required_providers {
    rubrik = {
      source  = "rubrikinc/rubrik"
      version = "~> 1.7.0"
    }
  }
}
```
If your configuration contains an explicit `provider "polaris" {}` block, rename it to `provider "rubrik" {}`.

The following resources support state migration via Terraform's `moved {}` block:

* `polaris_custom_role` → `rubrik_custom_role`
* `polaris_role_assignment` → `rubrik_role_assignment`
* `polaris_sso_group` → `rubrik_sso_group`
* `polaris_user` → `rubrik_user`

For each of these resources, rename the `resource` block to use the `rubrik` prefix and add a `moved {}` block
referencing the old and new addresses. For example, a `polaris_custom_role` resource named `compliance_auditor` would
become:
```terraform
moved {
  from = polaris_custom_role.compliance_auditor
  to   = rubrik_custom_role.compliance_auditor
}

resource "rubrik_custom_role" "compliance_auditor" {
  # ... existing configuration ...
}
```
Data sources do not have state, so they only need their prefix renamed. For example, a `polaris_role` data source named
`viewer` would become:
```terraform
data "rubrik_role" "viewer" {
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
unwanted diff, see the _New Features_ and _Significant Changes_ sections below for additional context. Otherwise,
proceed by running:
```shell
% terraform apply
```
This will record the renames (Option 2) in state and migrate the local Terraform state to the v1.7.0 version.

## New Features

### Initial Terraform Search Support

An initial set of three resource types now support Terraform list resources, allowing them to be discovered with
`terraform query`. This includes resources that are not managed by Terraform. Additional resource types will gain
`terraform query` support in future releases. For background on the feature, see HashiCorp's blog post
[Terraform Search and Import: Find resources and bring them into Terraform](https://www.hashicorp.com/en/blog/terraform-search-and-import-find-resources-and-bring-them-into-terraform).

#### rubrik_custom_role

List all custom roles in RSC, or filter by name:
```terraform
list "rubrik_custom_role" "all" {
  provider = rubrik
}

list "rubrik_custom_role" "by_name" {
  provider = rubrik

  config {
    name = "Compliance Auditor"
  }
}
```

#### rubrik_user

List all users in RSC, or filter by email:
```terraform
list "rubrik_user" "all" {
  provider = rubrik
}

list "rubrik_user" "by_email" {
  provider = rubrik

  config {
    email = "auditor@example.org"
  }
}
```

#### rubrik_sso_group

List all SSO groups in RSC, or filter by name and optionally by auth domain ID:
```terraform
list "rubrik_sso_group" "all" {
  provider = rubrik
}

list "rubrik_sso_group" "by_name_and_domain" {
  provider = rubrik

  config {
    name           = "Auditors"
    auth_domain_id = "1a5629cb-2681-4ea4-b36c-ea8b2f3990cd"
  }
}
```

### Initial `moved {}` Block Support

An initial set of four resources now support Terraform's `moved {}` block for state migration. This makes it possible
to rename a resource from the `polaris` to the `rubrik` prefix without losing state. Additional resources will gain
`moved {}` block support in future releases.

The following resources support `moved {}`:

* `rubrik_custom_role`
* `rubrik_role_assignment`
* `rubrik_sso_group`
* `rubrik_user`

See [Option 2 in How to Upgrade](#option-2-switch-source-to-rubrikincrubrik-and-change-the-local-name-to-rubrik) for
the migration procedure.

## Significant Changes

### Provider Rename

The provider is now published under two source addresses: the original `rubrikinc/polaris` and the renamed
`rubrikinc/rubrik`. Both will continue to be released and supported for the foreseeable future. The renamed provider
recognizes both the `polaris` and `rubrik` resource and data source prefixes, with the `polaris` names retained as
deprecated aliases. This makes it possible to switch the `source` to `rubrikinc/rubrik` without renaming any
resources. See [How to Upgrade](#how-to-upgrade) for the available paths.

### Environment Variables

The provider environment variables have been renamed from `RUBRIK_POLARIS_*` to `RUBRIK_*`. For example,
`RUBRIK_POLARIS_SERVICEACCOUNT_FILE` is now `RUBRIK_SERVICEACCOUNT_FILE`. The `RUBRIK_POLARIS_*` variants continue to
work via a fallback, so existing setups do not need to change immediately.

The Terraform logging environment variables have also been renamed:

* `TF_LOG_PROVIDER_POLARIS` is replaced by `TF_LOG_PROVIDER_RUBRIK`. Terraform derives this variable from the provider
  name automatically.
* `TF_LOG_PROVIDER_POLARIS_API` is replaced by `TF_LOG_PROVIDER_RUBRIK_API`.

## Bug Fixes

### polaris_sla_domain: object-specific config drift fix

Optional retention unit fields in the `sap_hana_config`, `db2_config`, `oracle_config` and `informix_config` blocks of
the `polaris_sla_domain` resource now mirror the schema default when the matching duration is unset, eliminating
spurious diffs after apply. In addition, the `storage_snapshot_config` block in `sap_hana_config` is only emitted when
it has data, and omitted retention fields in all object-specific configuration blocks are left out of the API request
instead of being sent with empty values.

### polaris_sla_domain: db2_config.log_archival_method default

The `log_archival_method` field in the `db2_config` block of the `polaris_sla_domain` resource now defaults to
`LOGARCHMETH1`, matching the RSC backend default. Previously, omitting the field produced a drift on subsequent plans
because the API returned `LOGARCHMETH1` while the schema treated the field as unset.
