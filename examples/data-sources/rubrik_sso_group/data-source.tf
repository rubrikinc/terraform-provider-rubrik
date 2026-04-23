# Look up SSO group by name.
data "rubrik_sso_group" "admins" {
  name = "Administrators"
}

# Look up SSO group by ID.
data "rubrik_sso_group" "admins" {
  sso_group_id = "<id>"
}
