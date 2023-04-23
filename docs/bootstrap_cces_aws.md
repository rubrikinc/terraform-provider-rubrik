## rubrik_bootstrap_cces_aws

Bootstrap_cces_aws will complete the bootstrap process for a Rubrik Cloud Cluster ES on AWS. You will also need to configure the Rubrik provider with the "username" and "password" set to blank strings.

## Example Usage

```hcl
resource "rubrik_bootstrap_cces_azure" "example" {
  cluster_name           = "tf-demo"
  admin_email            = "tf@demo.com"
  admin_password         = "RubrikTFDemo2019"
  management_gateway     = "192.168.100.1"
  management_subnet_mask = "255.255.255.0"
  dns_search_domain      = "demo.com"
  dns_name_servers       = ["192.168.100.5". "192.168.100.6"]            
  ntp_server1_name            = "8.8.8.8"
  ntp_server2_name            = "8.8.4.4"
  node_config = {
    tf-node01 = "192.168.100.100"
  }
  bucket_name            = "tf_aws_s3_bucket_name"
}
```

## Argument Reference

The following arguments are supported:

* `cluster_name` - (Required) Unique name to assign to the Rubrik cluster.
* `admin_email` - (Required) The Rubrik cluster sends messages for the admin account to this email address.
* `admin_password` - (Required) Password for the admin account
* `management_gateway` - (Required) IP address assigned to the management network gateway.
* `management_subnet_mask` - (Required) Subnet mask assigned to the management network.
* `dns_search_domain` - (Required) List of search domains that the DNS Service will use to resolve hostnames that are not fully qualified.
* `dns_name_servers` - (Required) List of the IPv4 addresses of the DNS servers.
* `ntp_server1_name` - (Required) The FQDN or IPv4 addresses of network time protocol (NTP) server #1.
* `ntp_server1_key_id` - (Optional) The ID number of the symmetric key used with NTP server #1. (Typically this is 0) (Required if `ntp_server1_key` and `ntp_server1_key_type` are also defined.)
* `ntp_server1_key` - (Optional) Symmetric key material for NTP server #1. (Required if `ntp_server1_key_id` and `ntp_server1_key_type` are also defined.) 
* `ntp_server1_key_type` (Optional) Symmetric key type for NTP server #1. (Required if `ntp_server1_key` and `ntp_server1_key_id` are also defined.)
* `ntp_server2_name` - (Required) The FQDN or IPv4 addresses of network time protocol (NTP) server #2.
* `ntp_server2_key_id` - (Optional) The ID number of the symmetric key used with NTP server #2. (Typically this is 0) (Required if `ntp_server2_key` and `ntp_server2_key_type` are also defined.)
* `ntp_server2_key` - (Optional) Symmetric key material for NTP server #2. (Required if `ntp_server2_key_id` and `ntp_server2_key_type` are also defined.) 
* `ntp_server2_key_type` (Optional) Symmetric key type for NTP server #2. (Required if `ntp_server2_key` and `ntp_server2_key_id` are also defined.)
* `node_config` - (Required) The Node Name and IP formatted as a map.
* `enable_encryption` - (Optional) Enable software data encryption at rest. When bootstrapping a Cloud Cluster this value needs to be False. Default value is false.
* `bucket_name` - (Required) AWS S3 bucket where CCES will store its data.
* `enable_immutability` - (Optional) Enable immutability on the S3 objects that CCES uses. Default value is false.
* `wait_for_completion` - (Optional) Flag to determine if the function should wait for the bootstrap process to complete. Default value is true.
* `timeout` - (Optional) The number of seconds to wait to establish a connection the Rubrik cluster before returning a timeout error. Default is 15.
