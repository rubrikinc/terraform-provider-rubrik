# Data security policy looked up by name.
data "rubrik_data_security_policy" "by_name" {
  name = "Overexposed Sensitive Data"
}

# Data security policy looked up by ID.
data "rubrik_data_security_policy" "by_id" {
  policy_id = "6ba9c3a1-2f4e-4d5c-9b1a-8e7f0d2c4a63"
}
