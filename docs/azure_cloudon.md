## Example Usage


```hcl
resource "rubrik_azure_cloudon" "example" {
  archive_name         = "TF-Demo"
  container            = "rubriktfdemo"
  storage_account_name = "rubriktf"
  application_id       = "e3152f67-232-3d5e-9fa4-d93f28ef8e9c4"
  application_key      = "gworr/wGI39/YJwBlv3+5vKMGxxjT56qdH6QHOUKraI="
  directory_id         = "5378b592-1987-40cb-9675-8a85797675cb"
  region               = "westus2"
  virtual_network_id   = "/subscriptions/32e23faw-a3d0-4b8c-ba74-93010aa3efa4/resourceGroups/tf-demo-rg/providers/Microsoft.Network/virtualNetworks/tf-demo-vnet"
  subnet_name          = "tf-demo-subnet-LAN-us-west-2a"
  security_group_id    = "/subscriptions/32e23faw-a3d0-4b8c-ba74-93010aa3efa4/resourceGroups/tf-demo-rg/providers/Microsoft.Network/networkSecurityGroups/gaia-amer1-cloudon-nsg"
}

```


## ## Argument Reference
