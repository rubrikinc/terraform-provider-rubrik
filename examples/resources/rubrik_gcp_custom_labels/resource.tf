resource "rubrik_gcp_custom_labels" "labels" {
  custom_labels = {
    "app"    = "RSC"
    "vendor" = "Rubrik"
  }

  excluded_labels = [
    "internal-cost-center",
    "temp-*",
  ]
}

# Scoped to a single cloud account.
resource "rubrik_gcp_custom_labels" "account_labels" {
  cloud_account_id = "b6c0b4a2-1d3e-4f5a-8b7c-9d0e1f2a3b4c"

  custom_labels = {
    "env" = "test"
  }
}
