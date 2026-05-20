list "rubrik_aws_cnp_account" "all" {
  provider = rubrik
}

list "rubrik_aws_cnp_account" "by_name" {
  provider = rubrik

  config {
    name = "production"
  }
}

list "rubrik_aws_cnp_account" "by_native_id" {
  provider = rubrik

  config {
    native_id = "123456789012"
  }
}
