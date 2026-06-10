---
page_title: "AWS IAM Roles Workflow"
---

# Adding an AWS account using the IAM roles workflow
The `rubrik_aws_account` resource onboards an AWS account using the AWS CloudFormation workflow. The permissions
granted to RSC through the CloudFormation stack can be difficult to understand and audit, and RSC requests the stack
to be updated whenever new features require new permissions.

The AWS IAM roles workflow lets you grant the same permissions transparently from Terraform. It uses the following
resources and data sources:
 * `rubrik_aws_cnp_account` _(Resource)_
 * `rubrik_aws_cnp_account_attachments` _(Resource)_
 * `rubrik_aws_cnp_artifacts`  _(Data Source)_
 * `rubrik_aws_cnp_permissions`  _(Data Source)_
 * `rubrik_account` _(Data Source)_

Start by discovering the IAM artifacts RSC needs for the chosen feature set using the `rubrik_aws_cnp_artifacts` data
source:
```terraform
data "rubrik_aws_cnp_artifacts" "artifacts" {
  feature {
    name = "CLOUD_NATIVE_PROTECTION"

    permission_groups = [
      "BASIC",
    ]
  }
}
```
One or more `feature` blocks lists the RSC features to enable for the AWS account. Use the `rubrik_account` data
source to obtain a list of RSC features available for the RSC account. The `rubrik_aws_cnp_artifacts` data source
returns the instance profile keys and role keys, referred to as _artifacts_ by RSC, required by the IAM roles workflow.

Next, use the `rubrik_aws_cnp_permissions` data source to obtain the IAM policies, both AWS managed and customer
managed, that must be attached to each role:
```terraform
data "rubrik_aws_cnp_permissions" "permissions" {
  for_each = data.rubrik_aws_cnp_artifacts.artifacts.role_keys
  role_key = each.key

  dynamic "feature" {
    for_each = data.rubrik_aws_cnp_artifacts.artifacts.feature
    content {
      name              = feature.value["name"]
      permission_groups = feature.value["permission_groups"]
    }
  }
}
```
For each role key, the data source returns the AWS managed policy ARNs to attach in `managed_policies` and a list of
named policy documents Rubrik defines in `customer_managed_policies`.

After defining the two data sources, use the `rubrik_aws_cnp_account` resource to start the onboarding of the AWS
account:
```terraform
resource "rubrik_aws_cnp_account" "account" {
  name      = "My Account"
  native_id = "123456789123"

  regions = [
    "us-east-2",
    "us-west-2",
  ]

  dynamic "feature" {
    for_each = data.rubrik_aws_cnp_artifacts.artifacts.feature
    content {
      name              = feature.value["name"]
      permission_groups = feature.value["permission_groups"]
    }
  }
}
```
`name` is the name given to the AWS account in RSC, `native_id` is the AWS account ID and `regions` the AWS regions to
protect with RSC. When Terraform processes this resource, the AWS account will show up in the connecting state in the
RSC UI.

In addition to the fields mentioned above, the `rubrik_aws_cnp_account` resource has a computed field called
`trust_policies`, which holds the IAM trust policies allowing RSC to assume the roles to elevate its privileges for
various tasks.

The next step is to create the required IAM roles, customer managed IAM policies, and instance profiles using the AWS
Terraform provider. Each customer managed policy becomes its own `aws_iam_policy`; both the AWS managed and the
customer managed ARNs are then attached to each role:
```terraform
locals {
  trust_policies = {
    for policy in rubrik_aws_cnp_account.account.trust_policies : policy.role_key => policy.policy
  }

  # Flatten the per-role customer managed policies into a single map keyed by
  # the policy name.
  customer_managed_policies = merge([
    for key, value in data.rubrik_aws_cnp_permissions.permissions : {
      for p in value.customer_managed_policies : p.name => {
        role_key = key
        policy   = p.policy
      }
    }
  ]...)
}

resource "aws_iam_role" "role" {
  for_each           = data.rubrik_aws_cnp_artifacts.artifacts.role_keys
  assume_role_policy = local.trust_policies[each.key]
  name_prefix        = "rubrik-${lower(each.key)}-"
}

resource "aws_iam_policy" "customer_managed" {
  for_each    = local.customer_managed_policies
  name_prefix = "rubrik-${each.key}-"
  policy      = each.value.policy
}

resource "aws_iam_role_policy_attachment" "customer_managed" {
  for_each   = local.customer_managed_policies
  role       = aws_iam_role.role[each.value.role_key].name
  policy_arn = aws_iam_policy.customer_managed[each.key].arn
}

resource "aws_iam_role_policy_attachments_exclusive" "policies" {
  for_each  = data.rubrik_aws_cnp_permissions.permissions
  role_name = aws_iam_role.role[each.key].name
  policy_arns = concat(
    each.value.managed_policies,
    [for k, v in local.customer_managed_policies : aws_iam_policy.customer_managed[k].arn if v.role_key == each.key]
  )
}

resource "aws_iam_instance_profile" "profile" {
  for_each    = data.rubrik_aws_cnp_artifacts.artifacts.instance_profile_keys
  name_prefix = "rubrik-${lower(each.key)}-"
  role        = aws_iam_role.role[each.value].name
}
```
`aws_iam_role_policy_attachment` is the resource that actually attaches each customer managed policy to its role and,
critically, calls `DetachRolePolicy` when the resource is destroyed. `aws_iam_role_policy_attachments_exclusive` then
declares the authoritative list of policy ARNs that should be attached to the role (AWS managed plus customer managed),
which lets Terraform detect and reconcile drift. A detailed explanation of the AWS resources can be found in the AWS
Terraform provider [documentation](https://registry.terraform.io/providers/hashicorp/aws/latest/docs).

Lastly, to finalize the onboarding of the AWS account, use the `rubrik_aws_cnp_account_attachments` resource:
```terraform
resource "rubrik_aws_cnp_account_attachments" "attachments" {
  account_id = rubrik_aws_cnp_account.account.id
  features   = rubrik_aws_cnp_account.account.feature.*.name

  dynamic "instance_profile" {
    for_each = aws_iam_instance_profile.profile
    content {
      key  = instance_profile.key
      name = instance_profile.value["arn"]
    }
  }

  dynamic "role" {
    for_each = aws_iam_role.role
    content {
      key         = role.key
      arn         = role.value["arn"]
      permissions = data.rubrik_aws_cnp_permissions.permissions[role.key].id
    }
  }
}
```
This attaches the instance profiles and roles to the AWS account in RSC. When Terraform processes this resource the AWS
account will transition from the connecting state to the connected state in the RSC UI.
