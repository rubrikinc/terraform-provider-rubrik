## Example Usage


```hcl
resource "rubrik_aws_s3_cloudon" "example" {
  archive_name      = "TF-Demo"
  vpc_id            = "vpc-28e32931"
  subnet_id         = "subnet-3ae87e92"
  security_group_id = "sg-9ba32ff8"
}
```


## ## Argument Reference
