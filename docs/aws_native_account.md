## Example Usage


```hcl
resource "rubrik_aws_native_account" "example" {
  aws_account_name = "TF-Demo"
  aws_regions = ["us-east-1"]
  bolt_config = [
    {
      region          = "us-east-1"
      vNetId          = "vpc-11a44968"
      subnetId        = "subnet-3ac58e06"
      securityGroupId = "sg-9ba90ee5"
    },
  ]
}

```


## ## Argument Reference
