list "rubrik_user" "all" {
  provider = rubrik
}

list "rubrik_user" "by_email" {
  provider = rubrik

  config {
    email = "auditor@example.org"
  }
}
