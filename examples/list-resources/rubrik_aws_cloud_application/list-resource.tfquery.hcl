list "rubrik_aws_cloud_application" "all" {
  provider = rubrik
}

list "rubrik_aws_cloud_application" "by_name" {
  provider = rubrik

  config {
    name = "web-store"
  }
}
