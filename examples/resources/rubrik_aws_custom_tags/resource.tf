resource "rubrik_aws_custom_tags" "tags" {
  custom_tags = {
    "app"    = "RSC"
    "vendor" = "Rubrik"
  }

  excluded_tags = [
    "internal-cost-center",
    "temp-*",
  ]
}

# Scoped to a single cloud account.
resource "rubrik_aws_custom_tags" "account_tags" {
  cloud_account_id = "b6c0b4a2-1d3e-4f5a-8b7c-9d0e1f2a3b4c"

  custom_tags = {
    "env" = "test"
  }
}
