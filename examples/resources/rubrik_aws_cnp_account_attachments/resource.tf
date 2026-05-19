# Minimal literal example for one feature. Shows the schema directly:
# one role block per RSC artifact key. The role itself must already
# exist in AWS, with the IAM policy returned by
# data.rubrik_aws_cnp_permissions attached to it.
resource "rubrik_aws_cnp_account_attachments" "attachments" {
  account_id = rubrik_aws_cnp_account.account.id
  features   = ["CLOUD_NATIVE_PROTECTION"]

  role {
    key         = "CROSSACCOUNT"
    arn         = "arn:aws:iam::123456789012:role/Rubrik-CROSSACCOUNT"
    permissions = data.rubrik_aws_cnp_permissions.crossaccount.id
  }
}

data "rubrik_aws_cnp_permissions" "crossaccount" {
  role_key = "CROSSACCOUNT"
  feature {
    name              = "CLOUD_NATIVE_PROTECTION"
    permission_groups = ["BASIC"]
  }
}

# Production form. The set of artifact keys required for a feature
# combination is discovered with rubrik_aws_cnp_artifacts; the IAM
# policy each role must carry comes from rubrik_aws_cnp_permissions.
# IAM roles and instance profiles are then created keyed by RSC
# artifact key so they line up with the dynamic blocks below.
data "rubrik_aws_cnp_artifacts" "artifacts" {
  feature {
    name              = "CLOUD_NATIVE_PROTECTION"
    permission_groups = ["BASIC"]
  }
  feature {
    name              = "EXOCOMPUTE"
    permission_groups = ["BASIC", "RSC_MANAGED_CLUSTER"]
  }
}

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

# One aws_iam_role per data.rubrik_aws_cnp_artifacts.artifacts.role_keys
# entry, plus one aws_iam_instance_profile per
# data.rubrik_aws_cnp_artifacts.artifacts.instance_profile_keys entry,
# both keyed by the artifact key. The trust policy for each role is
# rubrik_aws_cnp_account.account.trust_policies; the IAM policy is
# data.rubrik_aws_cnp_permissions.permissions[<key>].

resource "rubrik_aws_cnp_account_attachments" "attachments_full" {
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

# Role-chained variant. Same shape as above, plus role_chaining_account_id
# pointing at the role-chaining account. The role-chaining account itself
# is onboarded with its own rubrik_aws_cnp_account_attachments resource
# (using the standard form, above).
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
