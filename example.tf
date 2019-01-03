// Assume the default rubrik_cdm_username, rubrik_cdm_node_ip, rubrik_cdm_password env
// vars are in place -- will need to manually populate node_ip, username, and password
// if not 
provider "rubrik" {}

resource "rubrik_configure_timezone" "timezone" {
  timezone = ""
}

resource "tls_private_key" "aws-cloud-out-rsa" {
  algorithm = "RSA"
}

resource "rubrik_azure_cloudout" "azure_archive_config" {
  container            = ""
  azure_access_key     = ""
  storage_account_name = ""
  archive_name         = ""
  rsa_key              = "${tls_private_key.aws-cloud-out-rsa.private_key_pem}"
}

resource "rubrik_azure_cloudout" "azure_archive_config" {
  archive_name         = "${rubrik_azure_cloudout.azure_archive_config.archive_name}"
  container            = "${rubrik_azure_cloudout.azure_archive_config.container}"
  storage_account_name = "${rubrik_azure_cloudout.azure_archive_config.storage_account_name}"
  application_id       = ""
  application_key      = ""
  directory_id         = ""
  region               = ""
  virtual_network_id   = ""
  subnet_name          = ""
  security_group_id    = ""
}

resource "aws_s3_bucket" "b" {
  bucket        = ""
  force_destroy = true
}

resource "rubrik_aws_s3_cloudout" "s3_archive_config" {
  aws_bucket     = "${aws_s3_bucket.b.bucket}"
  storage_class  = ""
  archive_name   = ""
  aws_region     = ""
  aws_access_key = ""
  aws_secret_key = ""
  rsa_key        = "${tls_private_key.aws-cloud-out-rsa.private_key_pem}"
}

resource "rubrik_aws_s3_cloudon" "aws_cloud_on" {
  # archive_name = "TF-AWS-S3"
  archive_name      = "${rubrik_aws_s3_cloudout.s3_archive_config.archive_name}"
  vpc_id            = ""
  subnet_id         = ""
  security_group_id = ""
}

resource "rubrik_aws_native_account" "rubrik_aws_native" {
  aws_account_name = ""
  aws_access_key   = ""
  aws_secret_key   = ""

  aws_regions = [""]

  bolt_config = [
    {
      region          = ""
      vNetId          = ""
      subnetId        = ""
      securityGroupId = ""
    },
  ]
}

# output "cluster_timezone" {
#   value = "${tls_private_key.aws-cloud-out-rsa.private_key_pem }"
# }

