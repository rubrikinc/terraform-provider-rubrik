---
page_title: "Upgrade Guide: v1.11.0"
---

# Upgrade Guide v1.11.0

## Before Upgrading

Review the [changelog](changelog.md) to understand what has changed and what might cause an issue when upgrading the
provider.

~> **Note:** If you are upgrading across multiple minor versions, review the upgrade guide for each intermediate version
as well. Each guide documents breaking changes and migration steps specific to that release.

## How to Upgrade

### If you are already using the `rubrikinc/rubrik` provider

Make sure that the `version` field is configured in a way which allows Terraform to upgrade to the v1.11.0 release. One
way of doing this is by using the pessimistic constraint operator `~>`, which allows Terraform to upgrade to the latest
release within the same minor version:
```terraform
terraform {
  required_providers {
    rubrik = {
      source  = "rubrikinc/rubrik"
      version = "~> 1.11.0"
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
instructions. Otherwise, refresh the state to the v1.11.0 version:
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
      version = "~> 1.11.0"
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
      version = "~> 1.11.0"
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
This will record the renames (Option 2) in state and migrate the local Terraform state to the v1.11.0 version.

## New Features

### GCP Exocompute on a Shared VPC

The `regional_config` block of the `rubrik_gcp_exocompute` resource gains two optional fields, `host_project_id` and
`secondary_range_name`.

`host_project_id` is the GCP project ID of the project owning the VPC network. It is only needed when the network is a
Shared VPC, in which case the network and the subnet belong to a host project rather than to the project running
Exocompute. Without it, RSC looks for the subnet in the Exocompute project and the GKE cluster setup fails. Note that
this is the GCP project ID of the host project, not the RSC cloud account ID.

`secondary_range_name` is the name of the GKE pods secondary IP range on the subnet. It defaults to `pods-cidr-range`,
which is the name RSC uses when the regional configuration does not specify one.

```terraform
data "rubrik_gcp_project" "shared_vpc_host" {
  name = "my-shared-vpc-host-project"
}

resource "rubrik_gcp_exocompute" "exocompute" {
  cloud_account_id = rubrik_gcp_project.project.id

  regional_config {
    region               = "us-west1"
    subnet_name          = "my-shared-vpc-subnet-01"
    vpc_name             = "my-shared-vpc-01"
    host_project_id      = data.rubrik_gcp_project.shared_vpc_host.project_id
    secondary_range_name = "my-secondary-range"
  }
}
```

~> **Note:** Running Exocompute on a Shared VPC requires the host project to be onboarded with the
`GCP_SHARED_VPC_HOST` feature.

### Cloud Applications

RSC's Cloud Applications feature, `CLOUD_NATIVE_CONFIG_PROTECTION`, protects the AWS configuration surrounding an
application, such as VPC and networking, IAM, KMS and load balancers. It is enabled with the new
`cloud_native_config_protection` block in the `rubrik_aws_account` resource. The feature name is also accepted by the
`rubrik_aws_cnp_account`, `rubrik_aws_cnp_account_attachments` and `rubrik_aws_cnp_account_trust_policy` resources, and
by the `rubrik_aws_cnp_artifacts` and `rubrik_aws_cnp_permissions` data sources.

The feature has the `BASIC`, `BASIC_2`, `RECOVERY`, `RECOVERY_2`, `RECOVERY_3` and `RECOVERY_4` permission groups. The
numbered groups exist only because a single AWS managed policy cannot hold the whole permission set. They carry no
separate meaning, so grant `BASIC` together with `BASIC_2`, and `RECOVERY` together with `RECOVERY_2`, `RECOVERY_3` and
`RECOVERY_4`. The deprecated `RECOVERY_NETWORKING` permission group is not accepted. RSC drops it on accounts using the
current permission layout, so accepting it would only cause a permanent plan diff. Use the
`rubrik_aws_permission_groups` data source to read the permission groups currently available for the feature.

```terraform
resource "rubrik_aws_account" "account" {
  profile = "default"

  cloud_native_config_protection {
    permission_groups = [
      "BASIC",
      "BASIC_2",
      "RECOVERY",
      "RECOVERY_2",
      "RECOVERY_3",
      "RECOVERY_4",
    ]

    regions = [
      "us-east-2",
    ]
  }
}
```

The same feature in the `rubrik_aws_cnp_account` resource:

```terraform
resource "rubrik_aws_cnp_account" "account" {
  name      = "My Account"
  native_id = "123456789123"

  feature {
    name = "CLOUD_NATIVE_CONFIG_PROTECTION"
    permission_groups = [
      "BASIC",
      "BASIC_2",
      "RECOVERY",
      "RECOVERY_2",
      "RECOVERY_3",
      "RECOVERY_4",
    ]
  }

  regions = [
    "us-east-2",
  ]
}
```

~> **Note:** The `cloud_discovery` block of the `rubrik_aws_account` resource cannot be removed while the Cloud
Applications feature, or any other protection feature, is enabled. The plan fails with an error.

The configuration captured by the feature is protected with the new `AWS_CONFIG_OBJECT_TYPE` object type in the
`rubrik_sla_domain` resource. The provider checks the following rules for an SLA Domain with this object type during
`terraform plan`, so a violation fails the plan instead of failing part way through an apply:

* The object type cannot be combined with other object types.
* The `minute_schedule` block is not supported.
* An `hourly_schedule` must have a `frequency` of at least 6 hours.
* At most one `backup_location` is allowed, and it is optional.

```terraform
resource "rubrik_sla_domain" "aws_config" {
  name         = "aws-config"
  description  = "AWS Config SLA"
  object_types = ["AWS_CONFIG_OBJECT_TYPE"]

  hourly_schedule {
    frequency      = 6
    retention      = 7
    retention_unit = "DAYS"
  }
}
```

~> **Note:** Since v1.10.0, the `rubrik_sla_domain` resource rejects `backup_location` for object types which do not
support one. `AWS_CONFIG_OBJECT_TYPE` does support one, so the optional single `backup_location` is accepted for it.

### Google BigQuery Protection

RSC can now protect Google BigQuery datasets. Two new features in the `rubrik_gcp_project` resource,
`GCP_BIGQUERY_PROTECTION` and `GCP_BIGQUERY_RESERVATION`, onboard a GCP project for it, and the new
`GCP_BIGQUERY_OBJECT_TYPE` object type in the `rubrik_sla_domain` resource protects the datasets.

`GCP_BIGQUERY_PROTECTION` enables backup and restore of the BigQuery datasets in the project. It has the `BASIC` and
`EXPORT_AND_RESTORE` permission groups. `GCP_BIGQUERY_RESERVATION` designates the project where RSC creates the BigQuery
slot reservation it runs BigQuery backup and recovery jobs on. It has the `BASIC` permission group. The permissions
required by the two features are returned by the `rubrik_gcp_permissions` data source for the same new features.

Only one project per RSC account can have the `GCP_BIGQUERY_RESERVATION` feature, so to move it to another project,
remove it from the current project before adding it to the new one. If the reservation project also contains datasets to
protect, add both features to the same `rubrik_gcp_project` resource.

```terraform
resource "rubrik_gcp_project" "bigquery" {
  project        = "my-bigquery-project"
  project_name   = "My BigQuery Project"
  project_number = 123456789012

  feature {
    name              = "GCP_BIGQUERY_PROTECTION"
    permission_groups = ["BASIC", "EXPORT_AND_RESTORE"]
  }
}

resource "rubrik_gcp_project" "bigquery_reservation" {
  project        = "my-reservation-project"
  project_name   = "My Reservation Project"
  project_number = 210987654321

  feature {
    name              = "GCP_BIGQUERY_RESERVATION"
    permission_groups = ["BASIC"]
  }
}
```

~> **Note:** Both BigQuery features require BigQuery protection to be enabled for the RSC account.

The datasets are protected by an SLA Domain with the `GCP_BIGQUERY_OBJECT_TYPE` object type. BigQuery backs up directly
to its backup locations, so the SLA Domain requires a `backup_location` and does not use the `archival` block.

```terraform
data "rubrik_gcp_archival_location" "archival_location" {
  name = "my-archival-location"
}

resource "rubrik_sla_domain" "bigquery" {
  name         = "gcp-bigquery"
  description  = "GCP BigQuery SLA"
  object_types = ["GCP_BIGQUERY_OBJECT_TYPE"]

  daily_schedule {
    frequency      = 1
    retention      = 30
    retention_unit = "DAYS"
  }

  backup_location {
    archival_group_id = data.rubrik_gcp_archival_location.archival_location.id
  }
}
```

The provider checks the following rules for an SLA Domain with this object type during `terraform plan`, so a violation
fails the plan instead of failing part way through an apply:

* The object type cannot be combined with other object types.
* At least one `backup_location` is required, and the `archival` block is not supported.
* Replication is not supported.
* The `minute_schedule` block is not supported.
* The most frequent schedule must take a snapshot at least every 7 days, which means an `hourly_schedule` with a
  `frequency` of at most 168, a `daily_schedule` with a `frequency` of at most 7, or a `weekly_schedule` with a
  `frequency` of 1. A `monthly_schedule`, `quarterly_schedule` or `yearly_schedule` is always further apart than that,
  so it cannot be the only schedule, but it can be combined with one of the others.

## Significant Changes

### `regional_config` now manages the host project and the secondary range

If an Exocompute configuration has been modified outside of Terraform, e.g. using the GraphQL API, to use a Shared VPC
or a named secondary range, declare the `host_project_id` and `secondary_range_name` fields in `regional_config`. Both
fields are now read from RSC, so the first plan after upgrading shows a diff until the Terraform configuration matches
what the Exocompute configuration already uses.

### The permission group data sources only return supported permission groups

The `rubrik_aws_permission_groups` and `rubrik_azure_permission_groups` data sources now only return the permission
groups which the account resources accept for the requested feature. Previously they returned every permission group
RSC offered for the feature, including groups the provider does not support yet. Passing those groups to the
`rubrik_aws_account`, `rubrik_aws_cnp_account` or `rubrik_azure_subscription` resources failed validation during plan.

When this was written, RSC offered the following permission groups which are no longer returned:

| Cloud | Feature                        | Permission groups no longer returned                                     |
|-------|--------------------------------|--------------------------------------------------------------------------|
| AWS   | `EXOCOMPUTE`                   | `ADVANCED_DIAGNOSTICS`, `SURGICAL_RECOVERY`, `RECOVERY_RDS_CONNECTIVITY` |
| AWS   | `RDS_PROTECTION`               | `RECOVER_TO_S3`                                                          |
| AWS   | `OUTPOST`                      | `RSC_MANAGED_CLUSTER`                                                    |
| Azure | `CLOUD_NATIVE_BLOB_PROTECTION` | `INVENTORY_GENERATION`                                                   |
| Azure | `EXOCOMPUTE`                   | `ENCRYPTION`                                                             |

A feature which the account resources do not accept at all now returns an empty `permission_groups` set.

No change is needed for configurations which pass the result of the data sources to the account resources. A
configuration which uses the result in some other way, for example in an output or in a condition on
`permission_groups`, now sees the filtered set. The `id` attribute is a hash of the returned permission groups and
statements, so it changes for the features listed above.
