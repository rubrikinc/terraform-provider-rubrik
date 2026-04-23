list "rubrik_custom_role" "all" {
  provider = rubrik
}

list "rubrik_custom_role" "by_name" {
  provider = rubrik

  config {
    name = "Compliance Auditor"
  }
}
