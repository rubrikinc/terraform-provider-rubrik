resource "rubrik_gcp_rcv_archival_location" "archive" {
  name   = "us-west1-archive"
  region = "us-west1"
  tier   = "ARCHIVE"
}

# Client-side encryption with a customer managed Cloud KMS key. The key manager
# must already be registered in RSC.
data "rubrik_gcp_kms_key_manager" "production" {
  name = "Production GCP Cloud KMS"
}

resource "rubrik_gcp_rcv_archival_location" "kms_archive" {
  name   = "us-west1-kms-archive"
  region = "us-west1"
  tier   = "BACKUP"

  kms_key {
    key_manager_id = data.rubrik_gcp_kms_key_manager.production.id
    key_name       = "rcv-archive-key"
    key_version    = "1"
  }
}
