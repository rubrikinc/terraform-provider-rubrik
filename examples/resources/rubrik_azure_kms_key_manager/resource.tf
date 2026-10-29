# Register an Azure key vault key manager for an Azure subscription onboarded
# with the CLOUD_NATIVE_UEM_KEY_MANAGEMENT feature.
resource "rubrik_azure_kms_key_manager" "production" {
  name             = "Production Azure Key Vault"
  description      = "Keys for the RCV archival locations."
  cloud_account_id = "f8c9c2a0-f02e-494d-bba9-0d4906917b8c"
  region           = "eastus2"
  vault_name       = "production-key-vault"
}

# The key manager ID is the value an RCV archival location refers to it by.
resource "rubrik_azure_rcv_archival_location" "archive" {
  name   = "eastus2-kms-archive"
  region = "eastus2"
  tier   = "BACKUP"

  key_vault_key {
    key_manager_id = rubrik_azure_kms_key_manager.production.id
    key_name       = "archive-key"
    key_version    = "56f46ba5741e4883b5e3ac5cb96a0a71"
  }
}
