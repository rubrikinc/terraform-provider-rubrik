data "rubrik_azure_subscription" "archival" {
    name = "archival-subscription"
}

# Source region.
resource "rubrik_azure_archival_location" "archival_location" {
  cloud_account_id            = data.rubrik_azure_subscription.archival.id
  name                        = "my-archival-location"
  storage_account_name_prefix = "archival"
}

# Source region with a customer managed key.
resource "rubrik_azure_archival_location" "archival_location" {
  cloud_account_id            = data.rubrik_azure_subscription.archival.id
  name                        = "my-archival-location"
  storage_account_name_prefix = "archival"

  customer_managed_key {
    name       = "my-archival-key"
    region     = "eastus"
    vault_name = "my-archival-key-vault"
  }
}

# Client-side encryption with a key in a key vault registered as a key manager
# in RSC.
data "rubrik_azure_kms_key_manager" "archival" {
  name = "archival-key-vault"
}

resource "rubrik_azure_archival_location" "archival_location" {
  cloud_account_id            = data.rubrik_azure_subscription.archival.id
  name                        = "my-archival-location"
  storage_account_name_prefix = "archival"

  key_vault_key {
    key_manager_id = data.rubrik_azure_kms_key_manager.archival.id
    key_name       = "my-archival-key"
    key_version    = "4f0a1b2c3d4e5f60718293a4b5c6d7e8"
  }
}

# Specific region.
resource "rubrik_azure_archival_location" "archival_location" {
  cloud_account_id            = data.rubrik_azure_subscription.archival.id
  name                        = "my-archival-location"
  storage_account_name_prefix = "archival"
  storage_account_region      = "eastus2"
}
