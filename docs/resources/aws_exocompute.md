---
page_title: "rubrik_aws_exocompute Resource - terraform-provider-rubrik"
subcategory: ""
description: |-
  
The `rubrik_aws_exocompute` resource creates an RSC Exocompute configuration
for AWS workloads.

There are 3 types of Exocompute configurations:
 1. *RSC Managed Host* - When an RSC managed host configuration is created, RSC
    will automatically deploy the necessary resources in the specified AWS
    region to run the Exocompute service, including the AWS security groups.
 2. *Customer Managed Host* - When a customer managed host configuration is
    created, RSC will not deploy any resources. Instead it will use the AWS EKS
    cluster attached by the customer, using the
    `rubrik_aws_exocompute_cluster_attachment` resource, for all operations.
 3. *Application* - An application configuration is created by mapping the
    application cloud account to a host cloud account. The application cloud
    account will leverage the Exocompute resources deployed for the host
    configuration.

Items 1 and 2 above requires that the AWS account has been onboarded with the
`EXOCOMPUTE` feature.

Since there are 3 types of Exocompute configurations, there are 3 ways to create
a `rubrik_aws_exocompute` resource:
 1. Using the `account_id`, `region`, `vpc_id` and `subnets` or `subnet` fields
    creates an RSC managed host configuration. Use the `subnet` block when pod
    subnets are needed. The `cluster_access` field can be used to configure
    private EKS cluster access.
 2. Using the `account_id` and `region` fields creates a customer managed host
    configuration. Note, the `rubrik_aws_exocompute_cluster_attachment`
    resource must be used to attach an AWS EKS cluster to the Exocompute
    configuration.
 3. Using the `account_id` and `host_account_id` fields creates an
    application configuration. Set `automated_key_sharing` to `true` to
    request automated KMS key sharing for the application account.

-> **Note:** Customer managed Exocompute is sometimes referred to as Bring Your
   Own Kubernetes (BYOK). Using both host and application Exocompute
   configurations is sometimes referred to as shared Exocompute.

---

# rubrik_aws_exocompute (Resource)


The `rubrik_aws_exocompute` resource creates an RSC Exocompute configuration
for AWS workloads.

There are 3 types of Exocompute configurations:
 1. *RSC Managed Host* - When an RSC managed host configuration is created, RSC
    will automatically deploy the necessary resources in the specified AWS
    region to run the Exocompute service, including the AWS security groups.
 2. *Customer Managed Host* - When a customer managed host configuration is
    created, RSC will not deploy any resources. Instead it will use the AWS EKS
    cluster attached by the customer, using the
    `rubrik_aws_exocompute_cluster_attachment` resource, for all operations.
 3. *Application* - An application configuration is created by mapping the
    application cloud account to a host cloud account. The application cloud
    account will leverage the Exocompute resources deployed for the host
    configuration.

Items 1 and 2 above requires that the AWS account has been onboarded with the
`EXOCOMPUTE` feature.

Since there are 3 types of Exocompute configurations, there are 3 ways to create
a `rubrik_aws_exocompute` resource:
 1. Using the `account_id`, `region`, `vpc_id` and `subnets` or `subnet` fields
    creates an RSC managed host configuration. Use the `subnet` block when pod
    subnets are needed. The `cluster_access` field can be used to configure
    private EKS cluster access.
 2. Using the `account_id` and `region` fields creates a customer managed host
    configuration. Note, the `rubrik_aws_exocompute_cluster_attachment`
    resource must be used to attach an AWS EKS cluster to the Exocompute
    configuration.
 3. Using the `account_id` and `host_account_id` fields creates an
    application configuration. Set `automated_key_sharing` to `true` to
    request automated KMS key sharing for the application account.

-> **Note:** Customer managed Exocompute is sometimes referred to as Bring Your
   Own Kubernetes (BYOK). Using both host and application Exocompute
   configurations is sometimes referred to as shared Exocompute.



## Example Usage

```terraform
data "rubrik_aws_account" "host" {
  name = "host-account"
}

# RSC managed Exocompute and security groups.
resource "rubrik_aws_exocompute" "host" {
  account_id = data.rubrik_aws_account.host.id
  region     = "us-east-2"
  vpc_id     = "vpc-4859acb9"

  subnets = [
    "subnet-ea67b67b",
    "subnet-ea43ec78"
  ]
}

# RSC managed Exocompute with private cluster access.
resource "rubrik_aws_exocompute" "host_private" {
  account_id     = data.rubrik_aws_account.host.id
  region         = "us-east-2"
  vpc_id         = "vpc-4859acb9"
  cluster_access = "EKS_CLUSTER_ACCESS_TYPE_PRIVATE"

  subnets = [
    "subnet-ea67b67b",
    "subnet-ea43ec78"
  ]
}

# RSC managed Exocompute with pod subnets.
resource "rubrik_aws_exocompute" "host_pods" {
  account_id = data.rubrik_aws_account.host.id
  region     = "us-east-2"
  vpc_id     = "vpc-4859acb9"

  subnet {
    subnet_id     = "subnet-ea67b67b"
    pod_subnet_id = "subnet-0cf281be"
  }
  subnet {
    subnet_id     = "subnet-ea43ec78"
    pod_subnet_id = "subnet-0f6b8efa"
  }
}

# Customer managed Exocompute.
resource "rubrik_aws_exocompute" "host" {
  account_id = data.rubrik_aws_account.host.id
  region     = "us-east-2"
}

resource "rubrik_aws_exocompute_cluster_attachment" "cluster" {
  cluster_name  = "my-eks-cluster"
  exocompute_id = rubrik_aws_exocompute.host.id
}

data "rubrik_aws_account" "application" {
  name = "application-account"
}

# Application Exocompute.
resource "rubrik_aws_exocompute" "application" {
  account_id      = data.rubrik_aws_account.application.id
  host_account_id = data.rubrik_aws_account.host.id
}
```


## Schema

### Required

- `account_id` (String) RSC cloud account ID (UUID). Changing this forces a new resource to be created.

### Optional

- `automated_key_sharing` (Boolean) Request automated KMS key sharing for an application configuration. Requires
  `host_account_id`. New mappings default to disabled. When omitted, the existing mapping's setting is kept.
  Changing the value forces a new resource to be created. See [Automated KMS key sharing](#automated-kms-key-sharing).
- `cluster_access` (String) EKS cluster access type. Possible values are `EKS_CLUSTER_ACCESS_TYPE_PUBLIC` and `EKS_CLUSTER_ACCESS_TYPE_PRIVATE`. Can only be used with RSC managed configurations. Changing this forces a new resource to be created.
- `cluster_security_group_id` (String) AWS security group ID for the cluster. Changing this forces a new resource to be created.
- `host_account_id` (String) Exocompute host cloud account ID. Changing this forces a new resource to be created.
- `node_security_group_id` (String) AWS security group ID for the nodes. Changing this forces a new resource to be created.
- `region` (String) AWS region to run the Exocompute instance in. Changing this forces a new resource to be created.
- `subnet` (Block Set, Max: 2) AWS subnet for the cluster. Each subnet block accepts a `subnet_id` (Required) and an
  optional `pod_subnet_id`. Conflicts with `subnets`. Changing this forces a new resource to be created. (see [below for nested schema](#nestedblock--subnet))
  - `subnet_id` (String, Required) AWS subnet ID.
  - `pod_subnet_id` (String, Optional) AWS subnet ID for the pods.
- `subnets` (Set of String) AWS subnet IDs for the cluster subnets. Conflicts with `subnet`. Changing this forces a new
  resource to be created.
- `timeouts` (Block, Optional) (see [below for nested schema](#nestedblock--timeouts))
- `trigger_health_check` (Boolean) Trigger a health check of the Exocompute configuration when it's created, or when the field is changed to `true`, and wait for the result. A health check which fails or doesn't finish is reported as a warning. Ignored for customer managed configurations. Defaults to `false`. See [Health Checks](#health-checks).
- `vpc_id` (String) AWS VPC ID for the cluster network. Changing this forces a new resource to be created.

### Read-Only

- `cloud_formation_url` (String, Sensitive) CloudFormation launch URL returned by RSC when enabling key sharing
  requires a stack update. Available only from the create response; the URL can expire.
- `id` (String) Exocompute configuration ID (UUID), or `app-<account_id>` for an application configuration.
- `kms_provisioning_status` (String) RSC provisioning status of the application's KMS mapping. A mapping that is still
  waiting for an IAM change or stack update reports `KMS_MAPPING_PROVISIONING_STATUS_CONNECTING`.
- `kms_role_policy` (String) IAM policy JSON returned by RSC when creating a mapping with key sharing for a
  manually onboarded application account. Attach it to the application account's role, not the host's.
- `polaris_managed` (Boolean) If true the security groups are managed by RSC.

<a id="nestedblock--subnet"></a>
### Nested Schema for `subnet`

Required:

- `subnet_id` (String) AWS subnet ID.

Optional:

- `pod_subnet_id` (String) AWS subnet ID for the pods.

<a id="nestedblock--timeouts"></a>
### Nested Schema for `timeouts`

Optional:

- `create` (String) Create resource timeout (defaults to `60m`).
- `update` (String) Update resource timeout (defaults to `60m`).

## Health Checks

Setting `trigger_health_check` to `true` makes the provider start an RSC health check of the Exocompute configuration
after the configuration is created, and wait for the result. Changing `trigger_health_check` from `false` to `true` on an
existing configuration also starts a health check, without replacing the configuration. Changing it to `false` has no
effect. Importing a configuration with `trigger_health_check` set to `true` starts a health check on the first apply.

-> **Note:** RSC runs the health check on an RSC managed cluster in the cloud account. When there is no running cluster
   in the region, RSC first launches one, so the health check can take around 30 minutes. Use the `timeouts` block to
   change how long the provider waits.

~> **Note:** A health check which reports the configuration as anything but healthy, a health check which doesn't
   finish before the timeout, and a health check which RSC refuses to start, e.g. because one is already running, are
   reported as warnings. The Exocompute configuration is kept.

`trigger_health_check` is ignored for customer managed configurations, RSC runs their health checks when it reconciles
the attached cluster.

## Automated KMS key sharing

Set `automated_key_sharing = true` on an application configuration to let RSC create grants on the application
account's KMS keys for the Exocompute host. The application and host must be different accounts, and the
application account must use Cloud Native Protection or RDS Protection. Automated KMS key sharing must be enabled
for your RSC organization. This option does not create a gateway KMS key.

### Manually onboarded application accounts

Attach `kms_role_policy` to the application account's registered cross-account role. The example uses
`aws_iam_role_policy`; a managed policy with a separate attachment also works. After the attachment succeeds,
confirm the mapping with [rubrik_aws_exocompute_kms_confirmation](aws_exocompute_kms_confirmation.md).

Make the policy depend on the mapping and the confirmation depend on the policy, as in the example. This orders
policy removal before the mapping confirms the revoke on destroy. Set `replace_triggered_by` on the policy and
confirmation so the same ordering applies when the mapping is replaced. RSC does not check whether the policy
was actually attached or removed. If you manage the policy outside this Terraform configuration, remove it
before destroying the mapping. The example assumes a single-role account.

### CloudFormation and StackSet application accounts

Open `cloud_formation_url` and apply the proposed AWS stack update to complete setup. RSC finalizes the mapping
after the update succeeds; do not use the manual confirmation resource for a stack-based account. Review the
change before approving it, especially if Terraform also manages the stack.

When destroying a mapping that needs a stack update to revoke permissions, Terraform starts the revoke but keeps
the mapping in state. Complete the pending revoke in RSC by applying the AWS stack update, then rerun Terraform.

## Import

To import an application Exocompute configuration, use `app-<account_id>`, where `account_id` is the application
RSC cloud account UUID. Import reads the current sharing mode and provisioning status. It cannot recover
`kms_role_policy` or `cloud_formation_url` because RSC returns them only when it creates the mapping.

Import is supported using the following syntax:


In Terraform v1.5.0 and later, the [`import` block](https://developer.hashicorp.com/terraform/language/import) can be used with the `id` attribute, for example:

```terraform
import {
  to = rubrik_aws_exocompute.host
  id = "58e2a8bb-078d-4f67-8b66-5515fd701c8e"
}
```



The [`terraform import` command](https://developer.hashicorp.com/terraform/cli/commands/import) can be used, for example:

```terraform
% terraform import rubrik_aws_exocompute.host 58e2a8bb-078d-4f67-8b66-5515fd701c8e
```

