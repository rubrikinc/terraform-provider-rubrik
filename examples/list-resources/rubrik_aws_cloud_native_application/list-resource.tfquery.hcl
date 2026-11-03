list "rubrik_aws_cloud_native_application" "all" {
  provider = rubrik
}

list "rubrik_aws_cloud_native_application" "by_name" {
  provider = rubrik

  config {
    name = "web-store"
  }
}
