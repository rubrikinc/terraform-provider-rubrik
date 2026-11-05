data "rubrik_aws_account" "archival" {
  name = "archival-account"
}

# Source region. The archived data is encrypted with the AWS managed S3 key,
# aws/s3, since no key is given.
resource "rubrik_aws_archival_location" "archival_location" {
  account_id    = data.rubrik_aws_account.archival.id
  name          = "my-archival-location"
  bucket_prefix = "e089osn2qn"
}

# Specific region. A specific region requires a customer managed KMS key, AWS
# managed keys such as aws/s3 are rejected.
resource "rubrik_aws_archival_location" "archival_location" {
  account_id     = data.rubrik_aws_account.archival.id
  name           = "my-archival-location"
  bucket_prefix  = "f48wad7flz"
  region         = "us-east-2"
  kms_master_key = "arn:aws:kms:us-east-2:123456789012:key/9c3f1b3e-9f2a-4c05-9d3f-1f2d0a5b8e77"
}

# Client-side encryption with a KMS key in a key manager registered in RSC.
data "rubrik_aws_kms_key_manager" "archival" {
  name = "archival-key-manager"
}

resource "rubrik_aws_archival_location" "archival_location" {
  account_id    = data.rubrik_aws_account.archival.id
  name          = "my-archival-location"
  bucket_prefix = "k2m9xq4rtv"
  region        = "us-east-2"

  kms_key {
    key_manager_id = data.rubrik_aws_kms_key_manager.archival.id
    key_id         = "9c3f1b3e-9f2a-4c05-9d3f-1f2d0a5b8e77"
  }
}
