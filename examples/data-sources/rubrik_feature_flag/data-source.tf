# Check if a feature flag is enabled for the RSC account.
data "rubrik_feature_flag" "flag" {
  name = "MY_FEATURE_FLAG"
}

output "enabled" {
  value = data.rubrik_feature_flag.flag.enabled
}
