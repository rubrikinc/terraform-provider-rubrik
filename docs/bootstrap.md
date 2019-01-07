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


## ## Argument Reference
