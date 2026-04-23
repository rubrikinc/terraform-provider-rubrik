list "rubrik_sso_group" "all" {
  provider = rubrik
}

list "rubrik_sso_group" "by_name" {
  provider = rubrik

  config {
    name = "Auditors"
  }
}

list "rubrik_sso_group" "by_name_and_domain" {
  provider = rubrik

  config {
    name            = "Auditors"
    auth_domain_id  = "1a5629cb-2681-4ea4-b36c-ea8b2f3990cd"
  }
}
