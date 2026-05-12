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
