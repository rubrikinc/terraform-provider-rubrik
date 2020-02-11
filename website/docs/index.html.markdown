---
layout: "github"
page_title: "Provider: rubrikcdm"
description: |-
  The Rubrik CDM provider is used to interact with resources on the Rubrik CDM platform.
---

# Rubrik CDM Provider

The Rubrik CDM provider is used to interact with resources on the Rubrik CDM platform.

Use the navigation to the left to read about the available resources.

## Example Usage

```hcl
# Configure the Rubrik CDM Provider
provider "rubrik" {
  node_ip     = "${var.rubrikcdm_node_ip}"
  username    = "${var.rubrikcdm_username}"
  password    = "${var.rubrikcdm_password}"
}

# Set the Rubrik cluster timezone
resource "rubrik_configure_timezone" "LA-Timezone" {
  timezone = "America/Los_Angeles"
}
```

## Argument Reference

The following arguments are supported in the `provider` block:

* `node_ip` - (Optional) The Node IP address of the Rubrik cluster 
  you wish to connect to. The value may also be sourced from the `rubrik_cdm_node_ip` 
  environment variable.

* `username` - (Optional) The username of the Rubrik cluster
  you wish to connect to. The value may also be sourced from the `rubrik_cdm_username` 
  environment variable.

* `password` - (Optional) The password of the Rubrik cluster 
  you wish to connect to. The value may also be sourced from the `rubrik_cdm_password` 
  environment variable