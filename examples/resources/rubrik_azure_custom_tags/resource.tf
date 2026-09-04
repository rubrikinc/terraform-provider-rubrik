resource "rubrik_azure_custom_tags" "tags" {
  custom_tags = {
    "app"    = "RSC"
    "vendor" = "Rubrik"
  }

  excluded_tags = [
    "internal-cost-center",
    "temp-*",
  ]
}
