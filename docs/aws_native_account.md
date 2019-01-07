## rubrik_aws_native_account

Enables the management and protection of Amazon Elastic Compute Cloud (Amazon EC2) instances from the Rubrik cluster.

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

## Argument Reference

The following arguments are supported:

* `aws_account_name` - (Required) The name of the AWS account you wish to protect. This is the name that will be displayed in the Rubrik UI
* `aws_access_key` - (Optional) The access key of a AWS account with the required permissions. Default is the `AWS_ACCESS_KEY_ID` environment variable.
* `aws_secret_key` - (Optional) The secret key of a AWS account with the required permissions. Default is the `AWS_SECRET_ACCESS_KEY` environment variable.
* `aws_regions` - (Required) List of AWS regions to protect in this AWS account.
* `bolt_config` - (Required) List of dicts containing per region bolt network configs.
* `delete_snapshots` - (Optional) On destory, this will delete all EBS snapshots driven by Rubrik from your AWS account
* `timeout` - (Optional) The number of seconds to wait to establish a connection the Rubrik cluster before returning a timeout error. Default is 15.