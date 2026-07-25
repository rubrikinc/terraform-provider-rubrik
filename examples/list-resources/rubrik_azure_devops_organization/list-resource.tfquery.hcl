list "rubrik_azure_devops_organization" "all" {
  provider = rubrik
}

list "rubrik_azure_devops_organization" "by_native_id" {
  provider = rubrik

  config {
    native_id = "my-org"
  }
}
