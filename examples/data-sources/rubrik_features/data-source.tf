# Output the features enabled for the RSC account.
data "rubrik_features" "features" {}

output "features_enabled" {
  value = data.rubrik_features.features.features
}
