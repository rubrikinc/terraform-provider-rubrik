## Example Usage


```hcl
resource "rubrik_aws_s3_cloudout" "example" {
  aws_bucket        = "rubriktfexample"
  storage_class     = "standard"
  archive_name      = "TF-Demo"
  kms_master_key_id = "1234abcd-12ab-34cd-56ef-1234567890ab"
}
```

## ## Argument Reference
