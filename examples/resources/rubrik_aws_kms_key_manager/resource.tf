# Register an AWS KMS key manager for an AWS account onboarded with the
# CLOUD_NATIVE_UEM_KEY_MANAGEMENT feature.
resource "rubrik_aws_kms_key_manager" "production" {
  name             = "Production AWS KMS"
  description      = "KMS keys for the RCV archival locations."
  cloud_account_id = "f8c9c2a0-f02e-494d-bba9-0d4906917b8c"
  region           = "us-east-2"
}

# The key manager ID is the value an RCV archival location refers to it by.
resource "rubrik_aws_rcv_archival_location" "archive" {
  name   = "us-east-2-kms-archive"
  region = "us-east-2"
  tier   = "BACKUP"

  kms_key {
    key_manager_id = rubrik_aws_kms_key_manager.production.id
    key_id         = "9c3f1b3e-9f2a-4c05-9d3f-1f2d0a5b8e77"
  }
}
