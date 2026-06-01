---
page_title: "rubrik_aws_cnp_account Resource - terraform-provider-rubrik"
subcategory: ""
description: |-
  
The `rubrik_aws_cnp_account` resource onboards an AWS account to RSC using the
AWS IAM roles workflow. To grant RSC permissions to perform certain operations
on the account, IAM roles need to be created and communicated to RSC using the
`rubrik_aws_cnp_attachments` resource.
The roles and permissions needed by RSC can be looked up using the
`rubrik_aws_cnp_artifact` and `rubrik_aws_cnp_permissions` data sources.

The `CLOUD_DISCOVERY` feature enables RSC to discover resources in the AWS
account without enabling protection. It is currently optional but will become
required when onboarding protection features. Once onboarded, it cannot be
removed unless all protection features are removed first.

-> **Note:** The `feature` block is shown as Optional in the schema below for
   technical reasons, but at least one `feature` block must be specified. The
   block-style syntax is preserved to remain compatible with existing Terraform
   configurations.

-> **Note:** To onboard an account using a CloudFormation stack instead of IAM
   roles, use the `rubrik_aws_account` resource.

## Permission Groups
Following is a list of features and their applicable permission groups. These
are used when specifying the feature set.

`CLOUD_DISCOVERY`
  * `BASIC` - Represents the basic set of permissions required to onboard the
    feature.

`CLOUD_NATIVE_ARCHIVAL`
  * `BASIC` - Represents the basic set of permissions required to onboard the
    feature.

`CLOUD_NATIVE_PROTECTION`
  * `BASIC` - Represents the basic set of permissions required to onboard the
    feature.
  * `DOWNLOAD_FILE` - Represents the set of permissions required to download
    files from snapshots.
  * `EXPORT_POWER_OFF` - Represents the set of permissions required to export
    EC2 instances and leave them powered off.
  * `EXPORT_POWER_ON` - Represents the set of permissions required to export
    EC2 instances and power them on.
  * `RESTORE` - Represents the set of permissions required to restore from
    snapshots.

`CLOUD_NATIVE_DYNAMODB_PROTECTION`
  * `BASIC` - Represents the basic set of permissions required to onboard the
    feature.
  * `RECOVERY` - Represents the set of elevated permissions required to perform
    recovery operations.

`CLOUD_NATIVE_S3_PROTECTION`
  * `BASIC` - Represents the basic set of permissions required to onboard the
    feature.

`EXOCOMPUTE`
  * `BASIC` - Represents the basic set of permissions required to onboard the
    feature.
  * `RSC_MANAGED_CLUSTER` - Represents the set of permissions required for the
    Rubrik-managed Exocompute cluster.

`KUBERNETES_PROTECTION`
  * `BASIC` - Represents the basic set of permissions required to onboard the
    feature.

`RDS_PROTECTION`
  * `BASIC` - Represents the basic set of permissions required to onboard the
    feature.
  * `RECOVERY` - Represents the set of elevated permissions required to perform
    recovery operations.

`ROLE_CHAINING`
  * `BASIC` - Represents the basic set of permissions required to onboard the
    feature.

`SERVERS_AND_APPS`
  * `CLOUD_CLUSTER_ES` - Represents the basic set of permissions required to
    onboard the feature.

-> **Note:** When permission groups are specified, the `BASIC` permission group
   is always required except for the `SERVERS_AND_APPS` feature.

---

# rubrik_aws_cnp_account (Resource)

The `rubrik_aws_cnp_account` resource onboards an AWS account to RSC using the
AWS IAM roles workflow. To grant RSC permissions to perform certain operations
on the account, IAM roles need to be created and communicated to RSC using the
`rubrik_aws_cnp_attachments` resource.
The roles and permissions needed by RSC can be looked up using the
`rubrik_aws_cnp_artifact` and `rubrik_aws_cnp_permissions` data sources.

The `CLOUD_DISCOVERY` feature enables RSC to discover resources in the AWS
account without enabling protection. It is currently optional but will become
required when onboarding protection features. Once onboarded, it cannot be
removed unless all protection features are removed first.

-> **Note:** The `feature` block is shown as Optional in the schema below for
   technical reasons, but at least one `feature` block must be specified. The
   block-style syntax is preserved to remain compatible with existing Terraform
   configurations.

-> **Note:** To onboard an account using a CloudFormation stack instead of IAM
   roles, use the `rubrik_aws_account` resource.

## Permission Groups
Following is a list of features and their applicable permission groups. These
are used when specifying the feature set.

`CLOUD_DISCOVERY`
  * `BASIC` - Represents the basic set of permissions required to onboard the
    feature.

`CLOUD_NATIVE_ARCHIVAL`
  * `BASIC` - Represents the basic set of permissions required to onboard the
    feature.

`CLOUD_NATIVE_PROTECTION`
  * `BASIC` - Represents the basic set of permissions required to onboard the
    feature.
  * `DOWNLOAD_FILE` - Represents the set of permissions required to download
    files from snapshots.
  * `EXPORT_POWER_OFF` - Represents the set of permissions required to export
    EC2 instances and leave them powered off.
  * `EXPORT_POWER_ON` - Represents the set of permissions required to export
    EC2 instances and power them on.
  * `RESTORE` - Represents the set of permissions required to restore from
    snapshots.

`CLOUD_NATIVE_DYNAMODB_PROTECTION`
  * `BASIC` - Represents the basic set of permissions required to onboard the
    feature.
  * `RECOVERY` - Represents the set of elevated permissions required to perform
    recovery operations.

`CLOUD_NATIVE_S3_PROTECTION`
  * `BASIC` - Represents the basic set of permissions required to onboard the
    feature.

`EXOCOMPUTE`
  * `BASIC` - Represents the basic set of permissions required to onboard the
    feature.
  * `RSC_MANAGED_CLUSTER` - Represents the set of permissions required for the
    Rubrik-managed Exocompute cluster.

`KUBERNETES_PROTECTION`
  * `BASIC` - Represents the basic set of permissions required to onboard the
    feature.

`RDS_PROTECTION`
  * `BASIC` - Represents the basic set of permissions required to onboard the
    feature.
  * `RECOVERY` - Represents the set of elevated permissions required to perform
    recovery operations.

`ROLE_CHAINING`
  * `BASIC` - Represents the basic set of permissions required to onboard the
    feature.

`SERVERS_AND_APPS`
  * `CLOUD_CLUSTER_ES` - Represents the basic set of permissions required to
    onboard the feature.

-> **Note:** When permission groups are specified, the `BASIC` permission group
   is always required except for the `SERVERS_AND_APPS` feature.

## Example Usage

```terraform
# Basic example.
resource "rubrik_aws_cnp_account" "account" {
  name      = "My Account"
  native_id = "123456789123"

  feature {
    name = "CLOUD_NATIVE_PROTECTION"
    permission_groups = [
      "BASIC",
    ]
  }

  feature {
    name = "EXOCOMPUTE"
    permission_groups = [
      "BASIC",
      "RSC_MANAGED_CLUSTER",
    ]
  }

  regions = [
    "us-east-2",
  ]
}

# Role-chaining account, can be used by one or more role-chained accounts.
resource "rubrik_aws_cnp_account" "role_chaining" {
  name      = "Role-chaining Account"
  native_id = "123456789123"

  feature {
    name = "ROLE_CHAINING"
    permission_groups = [
      "BASIC",
    ]
  }

  regions = [
    "us-east-2",
  ]
}

# Role-chained account, using a previously onboarded role-chaining account.
resource "rubrik_aws_cnp_account" "role_chained" {
  name                     = "Role-Chained Account"
  native_id                = "234567891234"
  role_chaining_account_id = rubrik_aws_cnp_account.role_chaining.id

  feature {
    name = "CLOUD_NATIVE_PROTECTION"
    permission_groups = [
      "BASIC",
    ]
  }

  feature {
    name = "EXOCOMPUTE"
    permission_groups = [
      "BASIC",
      "RSC_MANAGED_CLUSTER",
    ]
  }

  regions = [
    "us-east-2",
    "us-west-2",
  ]
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `native_id` (String) AWS account ID. Changing this forces a new resource to be created.
- `regions` (Set of String) AWS regions.

### Optional

- `cloud` (String) AWS cloud type. Possible values are `STANDARD`, `CHINA` and `GOV`. Default value is `STANDARD`. Changing this forces a new resource to be created.
- `delete_snapshots_on_destroy` (Boolean) Should snapshots be deleted when the resource is destroyed. Default value is `false`.
- `external_id` (String) External ID used in the AWS IAM trust policy. When omitted, RSC generates a random external ID at onboarding. Once set the value cannot be changed; changing this field forces a new resource to be created.
- `feature` (Block Set) RSC feature with permission groups. At least one `feature` block must be specified. (see [below for nested schema](#nestedblock--feature))
- `name` (String) Account name.
- `role_chaining_account_id` (String) RSC cloud account ID of the role chaining account. When specified, the account will use cross-account role chaining. Changing this forces a new resource to be created.

### Read-Only

- `id` (String) RSC cloud account ID (UUID).
- `trust_policies` (Attributes Set) AWS IAM trust policies required by RSC. The `policy` field should be used with the `assume_role_policy` of the `aws_iam_role` resource. (see [below for nested schema](#nestedatt--trust_policies))

<a id="nestedblock--feature"></a>
### Nested Schema for `feature`

Required:

- `name` (String) RSC feature name. Possible values are `CLOUD_DISCOVERY`, `CLOUD_NATIVE_ARCHIVAL`, `CLOUD_NATIVE_DYNAMODB_PROTECTION`, `CLOUD_NATIVE_PROTECTION`, `CLOUD_NATIVE_S3_PROTECTION`, `EXOCOMPUTE`, `KUBERNETES_PROTECTION`, `RDS_PROTECTION`, `ROLE_CHAINING` and `SERVERS_AND_APPS`.
- `permission_groups` (Set of String) RSC permission groups for the feature. Possible values are `BASIC`, `CLOUD_CLUSTER_ES`, `DOWNLOAD_FILE`, `EXPORT_POWER_ON`, `EXPORT_POWER_OFF`, `RECOVERY`, `RESTORE` and `RSC_MANAGED_CLUSTER`. For backwards compatibility, `[]` is interpreted as all applicable permission groups.


<a id="nestedatt--trust_policies"></a>
### Nested Schema for `trust_policies`

Read-Only:

- `policy` (String) AWS IAM trust policy.
- `role_key` (String) RSC artifact key for the AWS role. Possible values are `CROSSACCOUNT`, `EXOCOMPUTE_EKS_MASTERNODE`, `EXOCOMPUTE_EKS_WORKERNODE` and `EXOCOMPUTE_EKS_LAMBDA`.

## Import

If an `external_id` was specified when the account was onboarded, it must also be
provided when importing the account. With identity-based import this goes in the
`external_id` field of the `identity` block. With the legacy string-id form the
external ID is appended to the account ID separated by either `:` or `-`, e.g.
an account onboarded with `external_id` set to `ExternalID` is imported as:
```text
f503742e-0a15-4a53-8579-54c2f978e49d:ExternalID
```

The `:` separator matches the convention used by other composite-id imports in
the provider; the `-` form is also accepted for backwards compatibility.

For accounts onboarded without an `external_id` (RSC generates one in that case),
omit the `external_id` field from the identity block, or pass just the account
ID with no separator suffix.

If the wrong external ID is specified, the next refresh will fail with an error
similar to:
```text
Error: failed to get trust policies: Already a value is registered as an external id.
```

Import is supported using the following syntax:

In Terraform v1.12.0 and later, the [`import` block](https://developer.hashicorp.com/terraform/language/import) can be used with the `identity` attribute, for example:

```terraform
import {
  to = rubrik_aws_cnp_account.account
  identity = {
    id = "3553bc74-7061-40e3-bac5-d2639e58bb7e"
    # Specify external_id only if one was provided when the account was
    # onboarded. Omit for accounts onboarded with an RSC-generated external ID.
    external_id = "ExternalID"
  }
}
```

<!-- schema generated by tfplugindocs -->
### Identity Schema

#### Required

- `id` (String) RSC cloud account ID (UUID).

#### Optional

- `external_id` (String) External ID set when the account was onboarded. Omit for accounts onboarded without an external ID (RSC generates one in that case). The value is stored as provided and is not verified against RSC, since RSC does not return external IDs.

In Terraform v1.5.0 and later, the [`import` block](https://developer.hashicorp.com/terraform/language/import) can be used with the `id` attribute, for example:

```terraform
import {
  to = rubrik_aws_cnp_account.account
  id = "3553bc74-7061-40e3-bac5-d2639e58bb7e:external-id"
}
```

The [`terraform import` command](https://developer.hashicorp.com/terraform/cli/commands/import) can be used, for example:

```shell
% terraform import rubrik_aws_cnp_account.account 3553bc74-7061-40e3-bac5-d2639e58bb7e:external-id
```
