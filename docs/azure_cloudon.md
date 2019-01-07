## rubrik_azure_cloudon

Provides the ability to convert a snapshot, archived snapshot, or replica into a Virtual Hard Disk (VHD). This enables the instantiation of the associated virtual machine on the Microsoft Azure cloud platform.

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

## Argument Reference

The following arguments are supported:

* `archive_name` - (Required) The name of the archive location used in the Rubrik GUI.
* `container` - (Required) The name of the Azure storage container being used as the archive target.
* `storage_account_name` - (Required) The name of the Storage Account that the container belongs to.
* `application_id` - (Required) The ID of the application registered in Azure Active Directory.
* `directory_id` - (Required) The directory ID, also known as the tenant ID, found under the Azure Active Directory properties.
* `region` - (Required) The name of the Azure region where the container is located. Valid choices are westus, westus2, centralus, eastus, eastus2, northcentralus, southcentralus, westcentralus, canadacentral, canadaeast, brazilsouth, northeurope, westeurope, uksouth, ukwest, eastasia, southeastasia, japaneast, japanwest, australiaeast,australiasoutheast, centralindia, southindia, westindia, koreacentral, koreasouth
* `virtual_network_id` - (Required) The Azure virtual network ID used by Rubrik cluster to launch a temporary Rubrik instance in Azure for instantiation.
* `subnet_name` - (Required) The Azure subnet name used by Rubrik cluster to launch a temporary Rubrik instance in Azure for instantiation.
* `security_group_id` - (Required) The Azure Security Group ID used by Rubrik cluster to launch a temporary Rubrik instance in Azure for instantiation.
* `timeout` - (Optional) The number of seconds to wait to establish a connection the Rubrik cluster before returning a timeout error. Default is 15.
