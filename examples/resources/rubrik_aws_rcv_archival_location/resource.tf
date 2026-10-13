# Archival location in a specific region.
resource "rubrik_aws_rcv_archival_location" "archive" {
  name   = "us-east-1-archive"
  region = "us-east-1"
  tier   = "BACKUP"
}

# Archival location following the region of the workload being archived.
resource "rubrik_aws_rcv_archival_location" "source_region_archive" {
  name = "source-region-archive"
  tier = "ARCHIVE"
}

# Client-side encryption with a customer managed KMS key.
resource "rubrik_aws_rcv_archival_location" "kms_archive" {
  name       = "us-east-1-kms-archive"
  region     = "us-east-1"
  tier       = "BACKUP"
  redundancy = "MULTI_ZONE"

  kms_key {
    key_manager_id = "8d2d0ee8-6b5a-4a58-b2a1-cd0b5f4ee2a4"
    key_id         = "9c3f1b3e-9f2a-4c05-9d3f-1f2d0a5b8e77"
  }
}

# Server-side encryption with a customer provided AES key, generated with
# `openssl rand -base64 32`. The key is write-only, so it never reaches Terraform
# state and can be sourced from a secret manager or an ephemeral variable.
variable "archive_aes_key" {
  type      = string
  sensitive = true
}

resource "rubrik_aws_rcv_archival_location" "cpk_archive" {
  name   = "us-east-1-cpk-archive"
  region = "us-east-1"
  tier   = "BACKUP"

  customer_provided_key {
    key = var.archive_aes_key

    # RSC only accepts the key when the archival location is created. Change the
    # version to replace the archival location with one using a new key.
    version = "1"
  }
}
