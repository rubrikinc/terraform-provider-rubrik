
## rubrik_aws_export_ec2

Export the latest snapshot of the specified EC2 instance.


## Example Usage

```hcl
resource "rubrik_aws_export_ec2" "export_ec2_instance" {
  instance_id          = "i-0398476613c9404dc"
  export_instance_name = "TF Export Instance"
  instance_type        = "m4.large"
  aws_region           = "us-east-2"
  subnet_id            = "subnet-873a313a"
  security_group_id    = "sg-082f484931cd7q3d1"
  date_time = "04-09-2019 05:56 PM"
  wait_for_completion  = true
}
```

## Argument Reference

The following arguments are supported:

* `instance_id` - (Required) The Instance ID of the AWS EC2 instance you wish to export.
* `export_instance_name` - (Required) The name to assign to instance being launched.
* `instance_type` - (Optional) The EC2 Instance Type to use for the instance being launched.
* `aws_region` - (Optional) The name of the AWS region where the bucket is located. Default is the `AWS_DEFAULT_REGION` environment variable.
* `subnet_id` - (Required) The ID of the subnet to assign to the instance being launched.
* `security_group_id` - (Required) The ID of the security group to assign to the instance being launched.
* `date_time` - (Required) The date and time of the EC2 Snapshot you wish to export formated as `Month:Day:Year Hour:Minute AM/PM`. Ex. `04-09-2019 05:56 PM`. You may also use `latest` to export the last snapshot taken.
* `wait_for_completion` - (Optional) Boolean flag to determine if the resource should wait for the export job to complete before continuing. Default is `False`.
* `timeout` - (Optional) The number of seconds to wait to establish a connection the Rubrik cluster before returning a timeout error. Default is `15`.

Valid `instance_type` choices:

*	a1.medium
* a1.large
* a1.xlarge
* a1.2xlarge
* a1.4xlarge
* m4.large
* m4.xlarge
* m4.2xlarge
* m4.4xlarge
* m4.10xlarge
* m4.16xlarge
* m5.large
* m5.xlarge
* m5.2xlarge
* m5.4xlarge
* m5.12xlarge
* m5.24xlarge
* m5a.large
* m5a.xlarge
* m5a.2xlarge
* m5a.4xlarge
* m5a.12xlarge
* m5a.24xlarge
* m5d.large
* m5d.xlarge
* m5d.2xlarge
* m5d.4xlarge
* m5d.12xlarge
* m5d.24xlarge
* t2.nano
* t2.micro
* t2.small
* t2.medium
* t2.large
* t2.xlarge
* t2.2xlarge
* t3.nano
* t3.micro
* t3.small
* t3.medium
* t3.large
* t3.xlarge
* t3.2xlarge
* c4.large
* c4.xlarge
* c4.2xlarge
* c4.4xlarge
* c4.8xlarge
* c5.large
* c5.xlarge
* c5.2xlarge
* c5.4xlarge
* c5.9xlarge
* c5.18xlarge
* c5d.xlarge
* c5d.2xlarge
* c5d.4xlarge
* c5d.9xlarge
* c5d.18xlarge
* c5n.large
* c5n.xlarge
* c5n.2xlarge
* c5n.4xlarge
* c5n.9xlarge
* c5n.18xlarge
* r4.large
* r4.xlarge
* r4.2xlarge
* r4.4xlarge
* r4.8xlarge
* r4.16xlarge
* r5.large
* r5.xlarge
* r5.2xlarge
* r5.4xlarge
* r5.12xlarge
* r5.24xlarge
* r5a.large
* r5a.xlarge
* r5a.2xlarge
* r5a.4xlarge
* r5a.12xlarge
* r5a.24xlarge
* r5d.large
* r5d.xlarge
* r5d.2xlarge
* r5d.4xlarge
* r5d.12xlarge
* r5d.24xlarge
* x1.16xlarge
* x1.32xlarge
* x1e.xlarge
* x1e.2xlarge
* x1e.4xlarge
* x1e.8xlarge
* x1e.16xlarge
* x1e.32xlarge
* z1d.large
* z1d.xlarge
* z1d.2xlarge
* z1d.3xlarge
* z1d.6xlarge
* z1d.12xlarge
* d2.xlarge
* d2.2xlarge
* d2.4xlarge
* d2.8xlarge
* h1.2xlarge
* h1.4xlarge
* h1.8xlarge
* h1.16xlarge
* i3.large
* i3.xlarge
* i3.2xlarge
* i3.4xlarge
* i3.8xlarge
* i3.16xlarge
* f1.2xlarge
* f1.4xlarge
* f1.16xlarge
* g3s.xlarge
* g3.4xlarge
* g3.8xlarge
* g3.16xlarge
* p2.xlarge
* p2.8xlarge
* p2.16xlarge
* p3.2xlarge
* p3.8xlarge
* p3.16xlarge
* p3dn.24xlarge

Valid `aws_region` choices:

* ap-south-1
* ap-northeast-3
* ap-northeast-2
* ap-southeast-1
* ap-southeast-2
* ap-northeast-1
* ca-central-1
* cn-north-1
* cn-northwest-1
* eu-central-1
* eu-west-1
* eu-west-2
* eu-west-3
* us-west-1
* us-east-1
* us-east-2
* us-west-2