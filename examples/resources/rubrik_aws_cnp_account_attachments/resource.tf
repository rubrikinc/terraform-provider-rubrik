# Attach artifacts to an account. Artifacts are IAM roles and instance
# profiles. The artifacts required can be looked up using the
# rubrik_aws_cnp_artifacts and rubrik_aws_cnp_permissions data
# sources. The configuration assumes that one AWS IAM instance profile
# and role has been defined for each RSC artifact.
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

# Attach artifacts to a role-chained account. To attach artifacts to
# the role-chaining account, use the above example.
resource "rubrik_aws_cnp_account_attachments" "attachments" {
  account_id               = rubrik_aws_cnp_account.account.id
  features                 = rubrik_aws_cnp_account.account.feature.*.name
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
