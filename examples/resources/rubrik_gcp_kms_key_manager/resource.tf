# Register a GCP Cloud KMS key manager for a GCP project onboarded with the
# CLOUD_NATIVE_UEM_KEY_MANAGEMENT feature.
resource "rubrik_gcp_kms_key_manager" "production" {
  name             = "Production GCP KMS"
  description      = "Keys for the RCV archival locations."
  cloud_account_id = "f8c9c2a0-f02e-494d-bba9-0d4906917b8c"
  region           = "us-west1"
  key_ring         = "archive-key-ring"
}

# The key manager ID is the value an RCV archival location refers to it by.
resource "rubrik_gcp_rcv_archival_location" "archive" {
  name   = "us-west1-kms-archive"
  region = "us-west1"
  tier   = "BACKUP"

  kms_key {
    key_manager_id = rubrik_gcp_kms_key_manager.production.id
    key_name       = "archive-key"
    key_version    = "1"
  }
}
