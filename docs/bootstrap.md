## rubrik_bootstrap

Bootstrap will complete the bootstrap process for a Rubrik cluster and requires a single node to have it's management interface configured. You will also need to configure the Rubrik provider with the "username" and "password" set to blank strings.

## Example Usage

```hcl
resource "rubrik_bootstrap" "example" {
  cluster_name           = "tf-demo"
  admin_email            = "tf@demo.com"
  admin_password         = "RubrikTFDemo2019"
  management_gateway     = "10.167.8.1"
  management_subnet_mask = "255.255.252.0"
  dns_name_servers       = ["10.167.8.2"]
  ntp_servers            = ["8.8.8.8"]

  node_config = {
    tf-node01 = "10.167.8.180"
  }
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
* `ntp_servers` - (Required) List of FQDN or IPv4 addresses of a network time protocol (NTP) server(s).
* `node_config` - (Required) The Node Name and IP formatted as a map.
* `enable_encryption` - (Optional) Enable software data encryption at rest. When bootstrapping a Cloud Cluster this value needs to be False. Default value is true.
* `wait_for_completion` - (Optional) Flag to determine if the function should wait for the bootstrap process to complete. Default value is true.
* `timeout` - (Optional) The number of seconds to wait to establish a connection the Rubrik cluster before returning a timeout error. Default is 15.
