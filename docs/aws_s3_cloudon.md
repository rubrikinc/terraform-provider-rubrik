## rubrik_aws_s3_cloudon

Provides the ability to convert a vSphere virtual machines snapshot, an archived snapshot, or a replica into an Amazon Machine Image (AMI) and then launch that AMI into an Elastic Compute Cloud (EC2) instance on an Amazon Virtual Private Cloud (VPC).

## Example Usage

```hcl
resource "rubrik_aws_s3_cloudon" "example" {
  archive_name      = "TF-Demo"
  vpc_id            = "vpc-28e32931"
  subnet_id         = "subnet-3ae87e92"
  security_group_id = "sg-9ba32ff8"
}
```

## Argument Reference

The following arguments are supported:

* `archive_name` - (Required) The name of the archive location used in the Rubrik GUI.
* `vpc_id` - (Required) The AWS VPC ID used by Rubrik cluster to launch a temporary Rubrik instance in AWS for instantiation.
* `subnet_id` - (Required) The AWS Subnet ID used by Rubrik cluster to launch a temporary Rubrik instance in AWS for instantiation.
* `security_group_id` - (Required) The AWS Security Group ID used by Rubrik cluster to launch a temporary Rubrik instance in AWS for instantiation.
* `timeout` - (Optional) The number of seconds to wait to establish a connection the Rubrik cluster before returning a timeout error. Default is 15.
