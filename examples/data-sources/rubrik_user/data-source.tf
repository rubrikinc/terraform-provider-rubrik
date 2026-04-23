# Look up user by email address.
data "rubrik_user" "admin" {
  email = "admin@example.org"
}

# Look up user by email address and user domain.
data "rubrik_user" "admin" {
  email  = "admin@example.org"
  domain = "SSO"
}

# Look up user by user ID.
data "rubrik_user" "admin" {
  user_id = "<id>"
}
