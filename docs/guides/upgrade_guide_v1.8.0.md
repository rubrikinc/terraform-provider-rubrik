---
page_title: "Upgrade Guide: v1.8.0"
subcategory: "Upgrade Guides"
---

# Upgrade Guide v1.8.0

The v1.8.0 release migrates the AWS IAM roles workflow resources and data sources to the Terraform Plugin Framework,
adds Terraform search support for AWS accounts onboarded with that workflow, and deprecates the
`rubrik_aws_cnp_account_trust_policy` resource. It also adds Multi-AZ resiliency and write-only credential attributes
to the cloud cluster resources, and a `RECOVERY` permission group for RDS and DynamoDB. Finally, it introduces a new
`rubrik_cluster_settings` resource for managing CDM package downloads and upgrades on Rubrik clusters registered with
RSC, along with `rubrik_cluster_settings` and `rubrik_cluster_versions` data sources to drive it.

As part of the framework migration, the `rubrik_aws_cnp_account` and `rubrik_aws_cnp_account_attachments` resources now
support Terraform's `moved {}` block. This makes migrating an AWS IAM roles workflow module from the `rubrikinc/polaris`
provider to `rubrikinc/rubrik` significantly easier than it was in v1.7.0, where these resources had to be removed from
state and re-imported. See [How to Upgrade](#how-to-upgrade) for the migration paths.

## Before Upgrading

Review the [changelog](changelog.md) to understand what has changed and what might cause an issue when upgrading the
provider.

~> **Note:** If you are upgrading across multiple minor versions, review the upgrade guide for each intermediate version
as well. Each guide documents breaking changes and migration steps specific to that release.

~> **Note:** Some resources in this version of the provider require **Terraform v1.11.0 or later**. See the
[Significant Changes](#significant-changes) section below for details on which resources are affected.

## How to Upgrade

### If you are already using the `rubrikinc/rubrik` provider

Make sure that the `version` field is configured in a way which allows Terraform to upgrade to the v1.8.0 release. One
way of doing this is by using the pessimistic constraint operator `~>`, which allows Terraform to upgrade to the latest
release within the same minor version:
```terraform
terraform {
  required_providers {
    rubrik = {
      source  = "rubrikinc/rubrik"
      version = "~> 1.8.0"
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
If you get an error or an unwanted diff, please see the _New Features_ and _Significant Changes_
sections below for additional instructions. Otherwise, refresh the state to the v1.8.0 version:
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
      version = "~> 1.8.0"
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
      version = "~> 1.8.0"
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

The first two are new in v1.8.0. A module that onboards AWS accounts with the IAM roles workflow can now take this
option, where in v1.7.0 it was limited to Option 1.

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
unwanted diff, see the _New Features_ and _Significant Changes_ sections below for additional context. Otherwise,
proceed by running:
```shell
% terraform apply
```
This will record the renames (Option 2) in state and migrate the local Terraform state to the v1.8.0 version.

## New Features

### Cluster Upgrade Lifecycle

A new `rubrik_cluster_settings` resource manages the CDM package download and upgrade lifecycle of a single Rubrik
cluster registered with RSC. Setting `version` drives the cluster to a target installed version: the provider
downloads the matching package and upgrades the cluster, blocking until the cluster reports the target version. When
the target is more than one release ahead, the provider automatically traverses the required intermediate releases
one hop at a time within a single apply.

Setting only `downloaded_version` pre-stages a package without upgrading. Both may be set together to upgrade to
`version` and pre-stage a newer `downloaded_version` for a future upgrade in the same apply. Setting `upgrade_mode`
toggles the cluster between `FAST` and `ROLLING` upgrades.

The companion `rubrik_cluster_versions` data source lists the CDM releases available to a cluster, including the
release recommended by RSC. The `rubrik_cluster_settings` data source returns the current upgrade state of a single
cluster. Both are typically used together to drive the resource:

```terraform
data "rubrik_cluster_versions" "cluster" {
  cluster_id = "db34f042-79ea-48b1-bab8-c40dfbf2ab82"
}

resource "rubrik_cluster_settings" "cluster" {
  cluster_id = data.rubrik_cluster_versions.cluster.cluster_id
  version    = data.rubrik_cluster_versions.cluster.recommended_version
}
```

The resource also supports air-gapped environments via the `package_url` and `package_md5` fields, which bypass the
Rubrik support portal. A custom package can only drive a single direct hop, not a multi-hop upgrade.

~> **Note:** A multi-hop upgrade runs each hop sequentially within a single apply, so the total time scales with the
number of intermediate releases. The default `timeouts.update` of 6 hours bounds the whole chain, not a single hop —
increase it when a target is several releases ahead.

Deleting the resource only removes it from Terraform state; the cluster and its installed version are left untouched.

The `rubrik_cluster_settings` resource also supports `terraform query`, so you can discover the upgrade and download
state of Rubrik clusters registered with RSC, including clusters not managed by Terraform:

```terraform
list "rubrik_cluster_settings" "all" {
  provider = rubrik
}

list "rubrik_cluster_settings" "by_version" {
  provider = rubrik

  config {
    version = "9.2.0-p1-25184"
  }
}
```

For more details, see the [rubrik_cluster_settings resource documentation](../resources/cluster_settings.md), the
[rubrik_cluster_settings data source documentation](../data-sources/cluster_settings.md) and the
[rubrik_cluster_versions data source documentation](../data-sources/cluster_versions.md).

### Terraform Search Support for the AWS IAM Roles Workflow

The `rubrik_aws_cnp_account` and `rubrik_aws_cnp_account_attachments` resources now support Terraform list resources,
allowing AWS accounts onboarded via the AWS IAM roles workflow to be discovered with `terraform query`. This includes
accounts that are not managed by Terraform. For background on the feature, see HashiCorp's blog post
[Terraform Search and Import: Find resources and bring them into Terraform](https://www.hashicorp.com/en/blog/terraform-search-and-import-find-resources-and-bring-them-into-terraform).

List all AWS accounts onboarded with the AWS IAM roles workflow in RSC, or filter by account name and AWS account ID:
```terraform
list "rubrik_aws_cnp_account" "all" {
  provider = rubrik
}

list "rubrik_aws_cnp_account" "by_name" {
  provider = rubrik

  config {
    name = "My Account"
  }
}
```

List the artifact attachments of AWS accounts onboarded with the AWS IAM roles workflow, or filter them by the parent
account:
```terraform
list "rubrik_aws_cnp_account_attachments" "all" {
  provider = rubrik
}

list "rubrik_aws_cnp_account_attachments" "by_native_id" {
  provider = rubrik

  config {
    native_id = "123456789012"
  }
}
```

### Extended `moved {}` Block Support

The `rubrik_aws_cnp_account` and `rubrik_aws_cnp_account_attachments` resources now support Terraform's `moved {}`
block, extending the set of resources from the v1.7.0 release. See
[Option 2 in How to Upgrade](#option-2-switch-source-to-rubrikincrubrik-and-change-the-local-name-to-rubrik) for the
migration procedure.

### RECOVERY permission group for RDS and DynamoDB

The `RDS_PROTECTION` and `CLOUD_NATIVE_DYNAMODB_PROTECTION` features now support a separate `RECOVERY` permission group
alongside `BASIC`. `BASIC` covers backup; `RECOVERY` grants the elevated AWS permissions required to perform recovery
operations. This split lets you keep the day-to-day footprint minimal and grant elevated privileges only when needed.

The new group is available on the `rubrik_aws_account` and `rubrik_aws_cnp_account` resources, and on the
`rubrik_aws_cnp_permissions` data source. To opt in, add `RECOVERY` to the `permission_groups` set on the matching
feature block in `rubrik_aws_cnp_account` (or, for `rubrik_aws_account`, enable the relevant sub-fields in the
`cloud_native_dynamodb_protection` or `rds_protection` block):

```terraform
resource "rubrik_aws_cnp_account" "account" {
  # ...
  feature {
    name              = "RDS_PROTECTION"
    permission_groups = ["BASIC", "RECOVERY"]
  }

  feature {
    name              = "CLOUD_NATIVE_DYNAMODB_PROTECTION"
    permission_groups = ["BASIC", "RECOVERY"]
  }
}
```

~> **Note:** The split between `BASIC` and `RECOVERY` is gated on the RSC account by the
`REL_ENABLE_AWS_PAAS_DB_PRIVILEGE_ELEVATION` feature flag. Until the flag is enabled, RSC returns the combined set of
actions under `BASIC` and listing `RECOVERY` has no effect. You can check the flag using the `rubrik_feature_flag`
data source. There is no need to re-run `terraform apply` once RSC enables the flag — RSC begins returning the split
catalog immediately and the next plan against the affected configuration will pick up the new permission set.

## Significant Changes

### Plugin Framework Migration

The following resources and data sources have been reimplemented using the Terraform Plugin Framework:

* `rubrik_aws_account` _(Data Source)_
* `rubrik_aws_cnp_account` _(Resource)_
* `rubrik_aws_cnp_account_attachments` _(Resource)_
* `rubrik_aws_cnp_artifacts` _(Data Source)_
* `rubrik_aws_cnp_permissions` _(Data Source)_

Schema, validation rules and stored state are preserved, so existing configurations continue to work without changes
and no diff is expected after upgrading.

### Write-Only Attributes on Cloud Cluster Resources

The `admin_email` and `admin_password` fields on the `rubrik_aws_cloud_cluster` and `rubrik_azure_cloud_cluster`
resources now use write-only attributes, which require **Terraform v1.11.0 or later**. These fields are only used during
initial cluster creation and cannot be changed after deployment, so they no longer need to be stored in state.

If you are running an older version of Terraform, you will see the following error when applying your configuration:

```
Error: Write-only Attribute Not Allowed

The resource contains a non-null value for write-only attribute
"admin_email" Write-only attributes are only supported in Terraform
1.11 and later.
```

### Multi-AZ Resiliency for Cloud Clusters

The `rubrik_aws_cloud_cluster` and `rubrik_azure_cloud_cluster` resources now support deploying clusters across
multiple availability zones for AZ resiliency. This is controlled by two new fields:

- `az_resilient` (bool) - Set to `true` to enable Multi-AZ deployment.
- `subnet_az_config` (block list in `vm_config`) - Specifies a subnet for each availability zone. Required when
  `az_resilient` is `true`.

When `az_resilient` is enabled:
- `use_placement_groups` must be `false` (AWS only).
- At least 3 availability zones should be specified in `subnet_az_config`.
- For AWS, `subnet_id` in `vm_config` becomes optional.
- For Azure, `availability_zone` and `subnet` in `vm_config` are replaced by `subnet_az_config` entries.

#### AWS Example

```terraform
resource "rubrik_aws_cloud_cluster" "multi_az" {
  cloud_account_id     = "12345678-1234-1234-1234-123456789012"
  region               = "us-west-2"
  az_resilient         = true
  use_placement_groups = false

  cluster_config {
    cluster_name            = "my-multi-az-cluster"
    admin_email             = "admin@example.com"
    admin_password          = "SecurePassword123!"
    dns_name_servers        = ["8.8.8.8"]
    ntp_servers             = ["pool.ntp.org"]
    num_nodes               = 3
    bucket_name             = "my-s3-bucket"
    enable_immutability     = true
    keep_cluster_on_failure = false
  }

  vm_config {
    cdm_version           = "9.4.0-p2-30507"
    instance_type         = "M6I_2XLARGE"
    instance_profile_name = "RubrikCloudClusterInstanceProfile"
    vpc_id                = "vpc-12345678"
    security_group_ids    = ["sg-12345678"]

    subnet_az_config {
      availability_zone = "us-west-2a"
      subnet            = "subnet-11111111"
    }

    subnet_az_config {
      availability_zone = "us-west-2b"
      subnet            = "subnet-22222222"
    }

    subnet_az_config {
      availability_zone = "us-west-2c"
      subnet            = "subnet-33333333"
    }
  }
}
```

#### Azure Example

```terraform
resource "rubrik_azure_cloud_cluster" "multi_az" {
  cloud_account_id = "12345678-1234-1234-1234-123456789012"
  az_resilient     = true

  cluster_config {
    cluster_name            = "my-multi-az-cluster"
    admin_email             = "admin@example.com"
    admin_password          = "SecurePassword123!"
    dns_name_servers        = ["8.8.8.8"]
    ntp_servers             = ["pool.ntp.org"]
    num_nodes               = 3
    keep_cluster_on_failure = false
  }

  vm_config {
    cdm_version                     = "9.2.3-p7-29713"
    instance_type                   = "STANDARD_D8S_V5"
    location                        = "westus"
    resource_group                  = "my-resource-group"
    network_resource_group          = "my-network-resource-group"
    vnet_resource_group             = "my-vnet-resource-group"
    vnet                            = "my-vnet"
    network_security_group          = "my-network-security-group"
    network_security_resource_group = "my-network-security-resource-group"
    vm_type                         = "EXTRA_DENSE"
    storage_account                 = "my-storage-account"
    container_name                  = "my-container"
    enable_immutability             = true
    user_assigned_managed_identity  = "my-managed-identity"

    subnet_az_config {
      availability_zone = "1"
      subnet            = "subnet-zone-1"
    }

    subnet_az_config {
      availability_zone = "2"
      subnet            = "subnet-zone-2"
    }

    subnet_az_config {
      availability_zone = "3"
      subnet            = "subnet-zone-3"
    }
  }
}
```

For more details, see the [rubrik_aws_cloud_cluster documentation](../resources/aws_cloud_cluster.md) and the
[rubrik_azure_cloud_cluster documentation](../resources/azure_cloud_cluster.md).

~> **Note:** Multi-AZ resiliency requires the `CCES_AZ_RESILIENCY_ENABLED` feature flag to be enabled on the RSC
account. You can verify this using the `rubrik_feature_flag` data source:

```terraform
data "rubrik_feature_flag" "az_resiliency" {
  name = "CCES_AZ_RESILIENCY_ENABLED"
}

output "az_resiliency_enabled" {
  value = data.rubrik_feature_flag.az_resiliency.enabled
}
```

If the feature flag is not enabled, contact Rubrik support to enable it before using Multi-AZ resiliency.

### Deprecation: `rubrik_aws_cnp_account_trust_policy`

The `rubrik_aws_cnp_account_trust_policy` resource is deprecated. Use the `trust_policies` field of the
`rubrik_aws_cnp_account` resource instead, which returns the IAM trust policies for all role keys in a single field and
supports role chaining.

The resource continues to work for now, but new configurations should read the trust policies from the account
resource. Where you previously defined a trust policy resource per role key:
```terraform
resource "rubrik_aws_cnp_account_trust_policy" "trust_policy" {
  for_each   = data.rubrik_aws_cnp_artifacts.artifacts.role_keys
  account_id = rubrik_aws_cnp_account.account.id
  features   = rubrik_aws_cnp_account.account.feature.*.name
  role_key   = each.key
}

resource "aws_iam_role" "role" {
  for_each           = data.rubrik_aws_cnp_artifacts.artifacts.role_keys
  assume_role_policy = rubrik_aws_cnp_account_trust_policy.trust_policy[each.key].policy
  name_prefix        = "rubrik-${lower(each.key)}-"
}
```
read the trust policies directly from the account resource instead:
```terraform
locals {
  trust_policies = {
    for policy in rubrik_aws_cnp_account.account.trust_policies : policy.role_key => policy.policy
  }
}

resource "aws_iam_role" "role" {
  for_each           = data.rubrik_aws_cnp_artifacts.artifacts.role_keys
  assume_role_policy = local.trust_policies[each.key]
  name_prefix        = "rubrik-${lower(each.key)}-"
}
```
See the [AWS IAM roles workflow](aws_cnp_account.md) guide for the full onboarding example.

### Deprecation: `features` field on `rubrik_aws_cnp_account_attachments`

The `features` field on the `rubrik_aws_cnp_account_attachments` resource is now deprecated. The set of features
(and their permission groups) is read directly from the cloud account managed by `rubrik_aws_cnp_account` when
artifacts are registered, so the attachments resource no longer needs to track the feature list. New permission
groups configured on `rubrik_aws_cnp_account` — including `RECOVERY` — flow through automatically without any change
to the attachments resource.

No action is required for existing configurations — the field is retained for backwards compatibility and is
populated from the cloud account when omitted. You will see a deprecation warning during `terraform plan`. The field
will be removed in a future major release; at that point you will be able to drop it entirely.
