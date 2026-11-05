data "rubrik_gcp_project" "project" {
  name = "my-gcp-project"
}

# Source region.
resource "rubrik_gcp_archival_location" "archival_location" {
  cloud_account_id = data.rubrik_gcp_project.project.id
  name             = "my-archival-location"
  bucket_prefix    = "my-bucket-prefix"
}

# Specific region.
resource "rubrik_gcp_archival_location" "archival_location" {
  cloud_account_id = data.rubrik_gcp_project.project.id
  name             = "my-archival-location"
  bucket_prefix    = "my-bucket-prefix"
  region           = "nam4"
}

# Client-side encryption with a Cloud KMS key in a key manager registered in RSC.
data "rubrik_gcp_kms_key_manager" "archival" {
  name = "archival-key-ring"
}

resource "rubrik_gcp_archival_location" "archival_location" {
  cloud_account_id = data.rubrik_gcp_project.project.id
  name             = "my-archival-location"
  bucket_prefix    = "my-bucket-prefix"

  kms_key {
    key_manager_id = data.rubrik_gcp_kms_key_manager.archival.id
    key_name       = "my-archival-key"
    key_version    = "1"
  }
}
