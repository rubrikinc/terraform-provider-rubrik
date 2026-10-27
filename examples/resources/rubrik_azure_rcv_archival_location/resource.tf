# Archival location in a specific region.
resource "rubrik_azure_rcv_archival_location" "archive" {
  name       = "eastus2-archive"
  region     = "eastus2"
  tier       = "BACKUP"
  redundancy = "MULTI_ZONE"
}

# Azure hosted vault archiving AWS workloads.
resource "rubrik_azure_rcv_archival_location" "aws_workload_archive" {
  name                  = "eastus2-aws-archive"
  region                = "eastus2"
  tier                  = "ARCHIVE"
  source_workload_cloud = "SOURCE_AWS"
}

# Client-side encryption with a customer managed Key Vault key.
resource "rubrik_azure_rcv_archival_location" "key_vault_archive" {
  name   = "eastus2-key-vault-archive"
  region = "eastus2"
  tier   = "BACKUP"

  key_vault_key {
    key_manager_id = "1a7c4f60-3c8d-4f1e-9a2b-6d5e7f8a9b0c"
    key_name       = "rcv-archive-key"
    key_version    = "4f0a1b2c3d4e5f60718293a4b5c6d7e8"
  }
}

# Client-side encryption with a customer provided RSA key. The key is write-only,
# so it never reaches Terraform state and can be sourced from a secret manager or
# an ephemeral variable.
variable "archive_rsa_key" {
  type      = string
  sensitive = true
}

resource "rubrik_azure_rcv_archival_location" "rsa_key_archive" {
  name   = "eastus2-rsa-key-archive"
  region = "eastus2"
  tier   = "BACKUP"

  rsa_key {
    key = var.archive_rsa_key

    # RSC only accepts the key when the archival location is created. Change the
    # version to replace the archival location with one using a new key.
    version = "1"
  }
}
