---
page_title: "rubrik_azure_cloud_cluster Resource - terraform-provider-rubrik"
subcategory: ""
description: |-
  
The `rubrik_azure_cloud_cluster` resource creates an Azure cloud cluster using RSC.

This resource creates a Rubrik Cloud Data Management (CDM) cluster with elastic storage
in Azure using the specified configuration. The cluster will be deployed with the specified
number of nodes, instance types, and network configuration.

~> **Note:** This resource creates actual Azure infrastructure. Destroying the
   resource removes the cluster from RSC, the cloud VMs are left running and must be
   cleaned up manually.

~> **Note:** The Azure subscription must be onboarded to RSC with the Server and Apps
   feature enabled before creating a cloud cluster.

~> **Note:** This resource requires **Terraform v1.11.0 or later** due to the use of write-only attributes for
   `admin_email` and `admin_password`.

~> **Note:** When destroying this resource, the cluster is removed from RSC. If the cluster has
   blocking conditions (active SLAs, global SLAs, or RCV locations), the removal fails and the
   conditions must be resolved first. `force_cluster_delete_on_destroy` only applies to
   disconnected clusters with blocking conditions, and must be applied before the resource is
   destroyed.

---

# rubrik_azure_cloud_cluster (Resource)

The `rubrik_azure_cloud_cluster` resource creates an Azure cloud cluster using RSC.

This resource creates a Rubrik Cloud Data Management (CDM) cluster with elastic storage
in Azure using the specified configuration. The cluster will be deployed with the specified
number of nodes, instance types, and network configuration.

~> **Note:** This resource creates actual Azure infrastructure. Destroying the
   resource removes the cluster from RSC, the cloud VMs are left running and must be
   cleaned up manually.

~> **Note:** The Azure subscription must be onboarded to RSC with the Server and Apps
   feature enabled before creating a cloud cluster.

~> **Note:** This resource requires **Terraform v1.11.0 or later** due to the use of write-only attributes for
   `admin_email` and `admin_password`.

~> **Note:** When destroying this resource, the cluster is removed from RSC. If the cluster has
   blocking conditions (active SLAs, global SLAs, or RCV locations), the removal fails and the
   conditions must be resolved first. `force_cluster_delete_on_destroy` only applies to
   disconnected clusters with blocking conditions, and must be applied before the resource is
   destroyed.

## Example Usage

```terraform
# Create an Azure cloud cluster using RSC
resource "rubrik_azure_cloud_cluster" "example" {
  cloud_account_id = "12345678-1234-1234-1234-123456789012"

  cluster_config {
    cluster_name            = "my-cloud-cluster"
    admin_email             = "admin@example.com"
    admin_password          = "RubrikGoForward!"
    dns_name_servers        = ["8.8.8.8", "8.8.4.4"]
    dns_search_domains      = ["example.com"]
    ntp_servers             = ["pool.ntp.org"]
    num_nodes               = 3
    keep_cluster_on_failure = false
    timezone                = "America/Los_Angeles"
    location                = "Palo Alto, CA"
  }

  vm_config {
    cdm_version                         = "9.2.3-p7-29713"
    instance_type                       = "STANDARD_D8S_V5"
    region                              = "westus"
    resource_group_name                 = "my-resource-group"
    storage_account_name                = "mystorageaccount"
    container_name                      = "my-container"
    enable_immutability                 = true
    user_assigned_managed_identity_name = "my-managed-identity"
    network_resource_group              = "my-network-resource-group"
    vnet_resource_group                 = "my-vnet-resource-group"
    vnet                                = "my-vnet"
    subnet                              = "my-subnet"
    network_security_group              = "my-network-security-group"
    network_security_resource_group     = "my-network-security-resource-group"
    vm_type                             = "EXTRA_DENSE"
    availability_zone                   = "1"
  }
}

# Create an Azure cloud cluster with Multi-AZ resiliency
resource "rubrik_azure_cloud_cluster" "multi_az" {
  cloud_account_id = "12345678-1234-1234-1234-123456789012"
  az_resilient     = true

  cluster_config {
    cluster_name            = "my-multi-az-cluster"
    admin_email             = "admin@example.com"
    admin_password          = "RubrikGoForward!"
    dns_name_servers        = ["8.8.8.8", "8.8.4.4"]
    ntp_servers             = ["pool.ntp.org"]
    num_nodes               = 3
    keep_cluster_on_failure = false
  }

  vm_config {
    cdm_version                         = "9.2.3-p7-29713"
    instance_type                       = "STANDARD_D8S_V5"
    region                              = "westus"
    resource_group_name                 = "my-resource-group"
    storage_account_name                = "mystorageaccount"
    container_name                      = "my-container"
    enable_immutability                 = true
    user_assigned_managed_identity_name = "my-managed-identity"
    network_resource_group              = "my-network-resource-group"
    vnet_resource_group                 = "my-vnet-resource-group"
    vnet                                = "my-vnet"
    network_security_group              = "my-network-security-group"
    network_security_resource_group     = "my-network-security-resource-group"
    vm_type                             = "EXTRA_DENSE"

    subnet_az_config {
      availability_zone = "1"
      subnet            = "my-subnet-1"
    }

    subnet_az_config {
      availability_zone = "2"
      subnet            = "my-subnet-2"
    }

    subnet_az_config {
      availability_zone = "3"
      subnet            = "my-subnet-3"
    }
  }
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `cloud_account_id` (String) RSC cloud account ID (UUID). Changing this forces a new resource to be created.

### Optional

> **NOTE**: [Write-only arguments](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) are supported in Terraform 1.11 and later.

- `az_resilient` (Boolean) Whether to deploy the cluster across multiple availability zones for AZ resiliency. When enabled, `subnet_az_config` blocks must be specified in `vm_config` instead of a single `subnet` and `availability_zone`. Changing this forces a new resource to be created.
- `cluster_config` (Block List) Configuration for the cloud cluster. (see [below for nested schema](#nestedblock--cluster_config))
- `timeouts` (Block, Optional) (see [below for nested schema](#nestedblock--timeouts))
- `vm_config` (Block List) VM configuration for the cluster nodes. (see [below for nested schema](#nestedblock--vm_config))

### Read-Only

- `id` (String) Cloud cluster ID (UUID).

<a id="nestedblock--cluster_config"></a>
### Nested Schema for `cluster_config`

Required:

> **NOTE**: [Write-only arguments](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) are supported in Terraform 1.11 and later.

- `admin_email` (String, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) Email address for the cluster admin user. Changing this value will have no effect on the cluster.
- `admin_password` (String, Sensitive, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) Password for the cluster admin user. Changing this value will have no effect on the cluster.
- `cluster_name` (String) Unique name to assign to the cloud cluster.
- `dns_name_servers` (Set of String) DNS name servers for the cluster.
- `keep_cluster_on_failure` (Boolean) Whether to keep the cluster on failure (can be useful for troubleshooting). Changing this forces a new resource to be created.
- `ntp_servers` (Set of String) NTP servers for the cluster.
- `num_nodes` (Number) Number of nodes in the cluster. Must be 1 or at least 3, two nodes are not supported. Changing this forces a new resource to be created.

Optional:

- `dns_search_domains` (Set of String) DNS search domains for the cluster.
- `force_cluster_delete_on_destroy` (Boolean) Whether to force delete the cluster on destroy. Only applies to disconnected clusters with blocking conditions. The value must be applied before the resource is destroyed.
- `location` (String) Location for the cluster. This is free text, RSC will map it to the closest possible location e.g. Palo Alto, CA.
- `timezone` (String) Timezone for the cluster using IANA standard format e.g. America/Los_Angeles, Europe/Paris, etc.


<a id="nestedblock--timeouts"></a>
### Nested Schema for `timeouts`

Optional:

- `create` (String) A string that can be [parsed as a duration](https://pkg.go.dev/time#ParseDuration) consisting of numbers and unit suffixes, such as "30s" or "2h45m". Valid time units are "s" (seconds), "m" (minutes), "h" (hours).
- `read` (String) A string that can be [parsed as a duration](https://pkg.go.dev/time#ParseDuration) consisting of numbers and unit suffixes, such as "30s" or "2h45m". Valid time units are "s" (seconds), "m" (minutes), "h" (hours). Read operations occur during any refresh or planning operation when refresh is enabled.


<a id="nestedblock--vm_config"></a>
### Nested Schema for `vm_config`

Required:

- `cdm_version` (String) CDM version to use. This is a create-time input, it is not updated when the cluster is upgraded outside of Terraform. Changing this forces a new resource to be created.
- `container_name` (String) Azure storage container name for the cluster. Changing this forces a new resource to be created.
- `enable_immutability` (Boolean) Whether to enable immutability for the storage account. Changing this forces a new resource to be created.
- `instance_type` (String) Azure instance type for the cluster nodes. Changing this forces a new resource to be created. Possible values are `STANDARD_DS5_V2`, `STANDARD_D16S_V5`, `STANDARD_D8S_V5`, `STANDARD_D32S_V5`, `STANDARD_E16S_V5`, `STANDARD_D8AS_V5`, `STANDARD_D16AS_V5`, `STANDARD_D32AS_V5` and `STANDARD_E16AS_V5`.
- `network_resource_group` (String) Azure resource group name for network resources. Changing this forces a new resource to be created.
- `network_security_group` (String) Azure network security group name. Changing this forces a new resource to be created.
- `network_security_resource_group` (String) Azure resource group name for the network security group. Changing this forces a new resource to be created.
- `region` (String) Azure region to deploy the cluster in. The format should be the native Azure format, e.g. `eastus`, `westus`, etc. Changing this forces a new resource to be created.
- `resource_group_name` (String) Azure resource group name where the cluster will be deployed. Changing this forces a new resource to be created.
- `storage_account_name` (String) Azure storage account name for the cluster. Changing this forces a new resource to be created.
- `user_assigned_managed_identity_name` (String) Name of the user-assigned managed identity. Changing this forces a new resource to be created.
- `vnet` (String) Azure virtual network name. Changing this forces a new resource to be created.
- `vnet_resource_group` (String) Azure resource group name for the virtual network. Changing this forces a new resource to be created.

Optional:

- `availability_zone` (String) Availability zone for the cluster, if this is not specified, the cluster will be deployed in availability zone 1. Used for single-AZ deployments. Changing this forces a new resource to be created.
- `subnet` (String) Azure subnet name for the cluster nodes. Required when `az_resilient` is false. Changing this forces a new resource to be created.
- `subnet_az_config` (Block List) Subnet and availability zone pairs for Multi-AZ deployments. Required when `az_resilient` is true. Each block specifies a subnet and its availability zone. Changing this forces a new resource to be created. (see [below for nested schema](#nestedblock--vm_config--subnet_az_config))
- `vm_type` (String) VM type for the cluster. Changing this forces a new resource to be created. Possible values are `STANDARD`, `DENSE` and `EXTRA_DENSE`. `EXTRA_DENSE` is recommended for CCES.

Read-Only:

- `cdm_product` (String) CDM Product Code. This is a read-only field and computed based on the CDM version when the cluster is created.

<a id="nestedblock--vm_config--subnet_az_config"></a>
### Nested Schema for `vm_config.subnet_az_config`

Required:

- `availability_zone` (String) Availability zone name.
- `subnet` (String) Subnet name for this availability zone.

## Import

The RSC cloud cluster ID (UUID) is used to import the cluster. RSC cannot return most of the values used when a cloud
cluster is created, such as the number of nodes, the virtual network, subnet and network security group and the VM configuration. An
imported cluster therefore holds only the values RSC can return, and the first apply after the import takes the remaining values from
the configuration without changing or replacing the cluster. The provider trusts the configuration to match the
cluster, so the `cluster_config` and `vm_config` blocks must describe the imported cluster. Changing one of these values
after the first apply replaces the cluster, as it does for any other cluster.

Import is supported using the following syntax:

In Terraform v1.12.0 and later, the [`import` block](https://developer.hashicorp.com/terraform/language/import) can be used with the `identity` attribute, for example:

```terraform
import {
  to = rubrik_azure_cloud_cluster.cluster
  identity = {
    id = "00000000-0000-0000-0000-000000000000"
  }
}
```

<!-- schema generated by tfplugindocs -->
### Identity Schema

#### Required

- `id` (String) Cloud cluster ID (UUID).

In Terraform v1.5.0 and later, the [`import` block](https://developer.hashicorp.com/terraform/language/import) can be used with the `id` attribute, for example:

```terraform
import {
  to = rubrik_azure_cloud_cluster.cluster
  id = "00000000-0000-0000-0000-000000000000"
}
```

The [`terraform import` command](https://developer.hashicorp.com/terraform/cli/commands/import) can be used, for example:

```shell
% terraform import rubrik_azure_cloud_cluster.cluster 00000000-0000-0000-0000-000000000000
```
