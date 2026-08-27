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
