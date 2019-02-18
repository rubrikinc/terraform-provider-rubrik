## rubrik_aws_s3_cloudout

Manages an AWS S3 archive target using a either an RSA Key or AWS KMS Master Key ID for encryption.

## Example Usage

```hcl
resource "rubrik_aws_s3_cloudout" "example" {
  aws_bucket        = "rubriktfexample"
  storage_class     = "standard"
  archive_name      = "TF-Demo"
  kms_master_key_id = "1234abcd-12ab-34cd-56ef-1234567890ab"
}
```

## Argument Reference

The following arguments are supported:

* `aws_bucket` - (Required) The name of the AWS S3 bucket you wish to use as an archive target.
* `storage_class` - (Optional) The AWS storage class you wish to use. Valid choices are `standard`, `standard_ia`, and `reduced_redundancy` with `standard` being the default choice.
* `archive_name` - (Required) The name of the archive location used in the Rubrik GUI..
* `aws_region` - (Optional) The name of the AWS region where the bucket is located. Valid choices are ap-south-1,ap-northeast-3, ap-northeast-2, ap-southeast-1, ap-southeast-2, ap-northeast-1, ca-central-1, cn-north-1, cn-northwest-1, eu-central-1, eu-west-1, eu-west-2, eu-west-3, us-west-1, us-east-1, us-east-2, and us-west-2. Default is the `AWS_DEFAULT_REGION` environment variable.
* `aws_access_key` - (Optional) The access key of a AWS account with the required permissions. Default is the `AWS_ACCESS_KEY_ID` environment variable.
* `aws_secret_key` - (Optional) The secret key of a AWS account with the required permissions. Default is the `AWS_SECRET_ACCESS_KEY` environment variable.
* `rsa_key` - (Optional) The RSA key that will be used to encrypt the archive data. If `kms_master_key_id` has not be provided this value is required.
* `kms_master_key_id` - (Optional) The AWS KMS master key ID that will be used to encrypt the archive data. If `rsa_key` has not be provided this value is required.
* `timeout` - (Optional) The number of seconds to wait to establish a connection the Rubrik cluster before returning a timeout error. Default is 15.
