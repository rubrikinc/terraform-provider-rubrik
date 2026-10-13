---
page_title: "Upgrade Guide: v1.12.0"
---

# Upgrade Guide v1.12.0

## Before Upgrading

Review the [changelog](changelog.md) to understand what has changed and what might cause an issue when upgrading the
provider.

~> **Note:** If you are upgrading across multiple minor versions, review the upgrade guide for each intermediate version
as well. Each guide documents breaking changes and migration steps specific to that release.

## How to Upgrade

### If you are already using the `rubrikinc/rubrik` provider

Make sure that the `version` field is configured in a way which allows Terraform to upgrade to the v1.12.0 release. One
way of doing this is by using the pessimistic constraint operator `~>`, which allows Terraform to upgrade to the latest
release within the same minor version:
```terraform
terraform {
  required_providers {
    rubrik = {
      source  = "rubrikinc/rubrik"
      version = "~> 1.12.0"
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
instructions. Otherwise, refresh the state to the v1.12.0 version:
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
      version = "~> 1.12.0"
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
      version = "~> 1.12.0"
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
This will record the renames (Option 2) in state and migrate the local Terraform state to the v1.12.0 version.

## New Features

### UEM Key Management

RSC's `CLOUD_NATIVE_UEM_KEY_MANAGEMENT` feature allows a cloud account's key management service to be registered as a
key manager in RSC. A key manager is needed to encrypt an archival location with a customer managed key. The feature
has the `BASIC` permission group.

For the `rubrik_aws_account` and `rubrik_azure_subscription` resources, the feature is enabled with the new
`cloud_native_uem_key_management` block. For the `rubrik_aws_cnp_account` and `rubrik_gcp_project` resources, the
feature is enabled by giving its name in a `feature` block. The feature name is also accepted by the
`rubrik_aws_cnp_account_attachments` resource, and by the `rubrik_aws_cnp_artifacts`, `rubrik_aws_cnp_permissions`,
`rubrik_azure_permissions` and `rubrik_gcp_permissions` data sources.

```terraform
resource "rubrik_aws_account" "account" {
  profile = "default"

  cloud_native_uem_key_management {
    permission_groups = [
      "BASIC",
    ]

    regions = [
      "us-east-2",
    ]
  }
}
```

RSC doesn't use regions for the feature in an Azure subscription, so its block has no `regions` field:

```terraform
resource "rubrik_azure_subscription" "subscription" {
  subscription_id   = "31be1bb0-c76c-11eb-9217-afdffe83a002"
  subscription_name = "My Subscription"
  tenant_domain     = "mydomain.onmicrosoft.com"

  cloud_native_uem_key_management {
    permission_groups = [
      "BASIC",
    ]
  }
}
```

For a GCP project, the feature is given in a `feature` block:

```terraform
resource "rubrik_gcp_project" "project" {
  project        = "my-project"
  project_name   = "My Project"
  project_number = 123456789012

  feature {
    name              = "CLOUD_NATIVE_UEM_KEY_MANAGEMENT"
    permission_groups = ["BASIC"]
  }
}
```

~> **Note:** The feature only grants RSC permission to read the key vaults of an Azure subscription. The key operations
require RSC to be assigned the Key Vault Crypto User role on the key vault. RSC doesn't allow the feature to be removed
while a key manager references the cloud account.

~> **Note:** The key managers themselves are registered in the RSC UI, under Settings, Security, Encryption Management.

### KMS Key Managers

The new `rubrik_aws_kms_key_manager`, `rubrik_azure_kms_key_manager` and `rubrik_gcp_kms_key_manager` data sources look
up a key manager registered in RSC, by name or by ID. The key manager ID is generated by RSC and is required, alongside
the key details from the cloud provider, to encrypt an archival location with a customer managed key.

```terraform
data "rubrik_aws_kms_key_manager" "aws" {
  name = "Production AWS KMS"
}

data "rubrik_azure_kms_key_manager" "azure" {
  name = "Production Azure Key Vault"
}

data "rubrik_gcp_kms_key_manager" "gcp" {
  name = "Production GCP Cloud KMS"
}
```

### RCV Archival Locations

Rubrik Cloud Vault, RCV, is Rubrik hosted archival storage. Unlike the cloud native archival locations managed by the
`rubrik_aws_archival_location`, `rubrik_azure_archival_location` and `rubrik_gcp_archival_location` resources, the
cloud account and the storage container are owned by Rubrik. There is no cloud account to onboard and no bucket or
container to point at, so the new resources take a region and a tier and nothing else:

```terraform
resource "rubrik_aws_rcv_archival_location" "archive" {
  name   = "us-east-1-archive"
  region = "us-east-1"
  tier   = "BACKUP"
}

resource "rubrik_azure_rcv_archival_location" "archive" {
  name       = "eastus2-archive"
  region     = "eastus2"
  tier       = "BACKUP"
  redundancy = "MULTI_ZONE"
}

resource "rubrik_gcp_rcv_archival_location" "archive" {
  name   = "us-west1-archive"
  region = "us-west1"
  tier   = "ARCHIVE"
}
```

The cloud in each resource name is the destination cloud, the cloud hosting the vault, not the cloud the archived
workloads originate from.

RCV archival locations support the `BACKUP` and `ARCHIVE` tiers. Redundancy is one of `SINGLE_ZONE`, `MULTI_ZONE` or
`MULTI_REGION`, and applies to AWS and Azure only.

What RSC allows you to change after creation differs per cloud, and the resources force a new archival location for
everything else. AWS supports updating the `name` and the `tier`, Azure the `name`, the `tier` and the `redundancy`,
and GCP only the `name`. A customer managed KMS key can additionally be rotated in place on all three clouds, see
[RCV Encryption](#rcv-encryption).

~> **Note:** Replacing an archival location destroys the existing one. Review the plan before applying a change that
   shows a replacement.

On AWS the `region` is optional. Leaving it out creates a source region archival location, which follows the region of
the workload being archived rather than pinning the vault to one region. The computed `location_template` attribute
reports which of the two applies, `SPECIFIC_REGION` or `SOURCE_REGION`.

```terraform
resource "rubrik_aws_rcv_archival_location" "source_region_archive" {
  name = "source-region-archive"
  tier = "ARCHIVE"
}
```

Azure is the only RCV cloud accepting a `source_workload_cloud`, which lets an Azure hosted vault archive AWS or GCP
workloads. When it is left out, RSC derives it from the destination cloud.

```terraform
resource "rubrik_azure_rcv_archival_location" "aws_workload_archive" {
  name                  = "eastus2-aws-archive"
  region                = "eastus2"
  tier                  = "ARCHIVE"
  source_workload_cloud = "SOURCE_AWS"
}
```

Azure also reports the immutability lock period through the computed `lock_duration_days` attribute.

### RCV Encryption

By default RCV creates and manages the encryption key for an archival location, and no configuration is needed. Both
AWS and Azure additionally accept a customer managed key, in two mutually exclusive forms each. The computed
`encryption_type` attribute, available on all three resources, reports which mode RSC actually applied.

On AWS, the `kms_key` block selects client-side encryption with an AWS KMS key:

```terraform
resource "rubrik_aws_rcv_archival_location" "kms_archive" {
  name   = "us-east-1-kms-archive"
  region = "us-east-1"
  tier   = "BACKUP"

  kms_key {
    key_manager_id = "8d2d0ee8-6b5a-4a58-b2a1-cd0b5f4ee2a4"
    key_id         = "9c3f1b3e-9f2a-4c05-9d3f-1f2d0a5b8e77"
  }
}
```

The `customer_provided_key` block instead selects server-side encryption with an AES-256 key you supply, base64
encoded — generate one with `openssl rand -base64 32`. On Azure the two modes are the `key_vault_key` block and the
`rsa_key` block, both client-side. The provider rejects a configuration setting both modes for a cloud at plan time.

The key inside each block is
[write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments), so it never
reaches Terraform state and can be sourced from a secret manager or an ephemeral variable. Because a write-only
argument produces no difference in the plan on its own, each block pairs its key with a `version` acting as the nonce
that makes a new key take effect:

```terraform
variable "archive_aes_key" {
  type      = string
  sensitive = true
}

resource "rubrik_aws_rcv_archival_location" "cpk_archive" {
  name   = "us-east-1-cpk-archive"
  region = "us-east-1"
  tier   = "BACKUP"

  customer_provided_key {
    key     = var.archive_aes_key
    version = "1"
  }
}
```

GCP uses a `kms_key` block with a key manager ID, a key name and a key version. RCV GCP has no other key mode.

A customer managed KMS key, the `kms_key` block on AWS and GCP and the `key_vault_key` block on Azure, can be rotated
in place. Changing the key ID, or the key name, key version or key manager, rekeys the archival location: RSC moves
it to the new key without re-encrypting the archived data, and the provider waits for the rekey to complete. It can
take several minutes, so increase `timeouts.update` if it takes longer than the default of 20 minutes. Adding or
removing the block replaces the archival location.

RSC only accepts the `customer_provided_key` and `rsa_key` keys when the archival location is created — the update
APIs take no encryption field for them — so there is no way to rotate those in place. Changing `version` therefore
replaces the archival location rather than updating it.

RSC never returns these keys, so an imported archival location has no `customer_provided_key` or `rsa_key` block in the
state. Adding the block to the configuration of an imported archival location which RSC reports as using that kind of
key, an `encryption_type` of `SSE_CPK` on AWS and `RSA_KEY_BASED` on Azure, updates it in place instead of replacing
it. The update changes nothing in RSC apart from sending the unchanged settings again, and records the block in the
state. The provider can't check that the key in the configuration is the one RSC uses, so make sure it is. Adding the
block to an archival location using any other kind of key replaces it.

~> **Note:** Write-only arguments are supported in Terraform 1.11 and later.

### Listing Archival Locations

Each of the three resources has a matching list resource, so existing archival locations can be discovered without
importing them one at a time. Declare one in a `.tfquery.hcl` file:

```terraform
list "rubrik_aws_rcv_archival_location" "all" {
  provider = rubrik
}
```

Run `terraform query` to discover archival locations, or `terraform query -generate-config-out=generated.tf` to also
generate a `resource` block and a matching `import` block for each one. The optional `name` argument filters on a name
prefix; leaving it out lists every RCV archival location of that cloud.

Because the encryption keys are write-only, RSC does not return them and generated configuration leaves them out. An
archival location created with a customer managed key still imports and reads back correctly, and `encryption_type`
reports the mode in use, but the key itself has to be added to the configuration by hand if you want Terraform to
recreate the archival location with the same key.

## Significant Changes

### Permission groups are validated per feature

The `rubrik_aws_cnp_account` and `rubrik_gcp_project` resources and the `rubrik_aws_cnp_artifacts`,
`rubrik_aws_cnp_permissions`, `rubrik_azure_permissions` and `rubrik_gcp_permissions` data sources now check that each
permission group is supported by the feature it is given for. Previously, any known permission group was accepted for
any feature, and an unsupported combination was only rejected by RSC, if at all. For example, `EXPORT` is a permission
group of `CLOUD_NATIVE_S3_PROTECTION`, but not of `RDS_PROTECTION`:

```terraform
resource "rubrik_aws_cnp_account" "account" {
  name      = "My Account"
  native_id = "123456789123"

  feature {
    name = "RDS_PROTECTION"
    permission_groups = [
      "BASIC",
      "EXPORT", # Now fails the plan.
    ]
  }

  regions = [
    "us-east-2",
  ]
}
```

If the plan fails with an error about a permission group not being supported by a feature, remove the permission group
from the feature. The permission groups each feature supports are listed in the documentation of the resource or data
source.

### GCP project feature status

The `status` attribute of the features in the `rubrik_gcp_project` resource and data source, and of the
`cloud_native_protection` block in the `rubrik_gcp_project` resource, previously held a lower case value, e.g.
`connected`. It now holds the status as returned by RSC, e.g. `CONNECTED`, which matches the `rubrik_azure_subscription`
resource.

The attribute is computed, so no change to the resource configuration is needed and Terraform does not plan any changes
for it. If your configuration compares the status to a lower case value, for example in a `precondition` or an output,
update the comparison to use the upper case value.

## Significant Changes

### Permission groups are validated per feature

The `rubrik_aws_cnp_account` and `rubrik_gcp_project` resources and the `rubrik_aws_cnp_artifacts`,
`rubrik_aws_cnp_permissions`, `rubrik_azure_permissions` and `rubrik_gcp_permissions` data sources now check that each
permission group is supported by the feature it is given for. Previously, any known permission group was accepted for
any feature, and an unsupported combination was only rejected by RSC, if at all. For example, `EXPORT` is a permission
group of `CLOUD_NATIVE_S3_PROTECTION`, but not of `RDS_PROTECTION`:

```terraform
resource "rubrik_aws_cnp_account" "account" {
  name      = "My Account"
  native_id = "123456789123"

  feature {
    name = "RDS_PROTECTION"
    permission_groups = [
      "BASIC",
      "EXPORT", # Now fails the plan.
    ]
  }

  regions = [
    "us-east-2",
  ]
}
```

If the plan fails with an error about a permission group not being supported by a feature, remove the permission group
from the feature. The permission groups each feature supports are listed in the documentation of the resource or data
source.
