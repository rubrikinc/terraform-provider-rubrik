# Required role keys and IAM policies.
data "rubrik_aws_cnp_artifacts" "artifacts" {
  dynamic "feature" {
    for_each = rubrik_aws_cnp_account.account.feature
    content {
      name              = feature.value["name"]
      permission_groups = feature.value["permission_groups"]
    }
  }
}

data "rubrik_aws_cnp_permissions" "permissions" {
  for_each = data.rubrik_aws_cnp_artifacts.artifacts.role_keys
  role_key = each.key

  dynamic "feature" {
    for_each = rubrik_aws_cnp_account.account.feature
    content {
      name              = feature.value["name"]
      permission_groups = feature.value["permission_groups"]
    }
  }
}

# Basic example.
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

# Role-chained variant, using a previously onboarded role-chaining account.
resource "rubrik_aws_cnp_account_attachments" "role_chained_attachments" {
  account_id               = rubrik_aws_cnp_account.role_chained.id
  features                 = rubrik_aws_cnp_account.role_chained.feature.*.name
  role_chaining_account_id = rubrik_aws_cnp_account.role_chaining.id

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
