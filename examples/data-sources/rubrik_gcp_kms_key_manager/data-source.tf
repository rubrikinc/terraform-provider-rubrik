# Look up a GCP Cloud KMS key manager by the name it was registered with in
# RSC.
data "rubrik_gcp_kms_key_manager" "production" {
  name = "Production GCP Cloud KMS"
}

# A key manager can also be looked up by ID, to read the rest of its details.
data "rubrik_gcp_kms_key_manager" "by_id" {
  id = "8d2d0ee8-6b5a-4a58-b2a1-cd0b5f4ee2a4"
}
