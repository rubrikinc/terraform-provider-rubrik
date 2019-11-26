---
layout: "rubrik"
page_title: "Provider: rubrik"
sidebar_current: "docs-rubrik-index"
description: |-
  The Rubrik provider is used to interact with Rubrik CDM.
---

# Rubrik Provider

The Rubrik provider is used to interact with Rubrik CDM.

## Authentication

The Rubrik provider offers a flexible means of providing credentials for
authentication. The following methods are supported, in this order, and
explained below:

- Environment variables
- Static credentials

### Environment variables

* **rubrik_cdm_node_ip** (Contains the IP/FQDN of a Rubrik node)
* **rubrik_cdm_username** (Contains a username with configured access to the Rubrik cluster)
* **rubrik_cdm_password** (Contains the password for the above user).

### Static credentials 

Static credentials can be provided by adding a `node_ip`, `username` and `password` in-line in the
Rubrik provider block:

Usage:

```hcl
provider "rubrik" {
  node_ip     = "10.255.41.201"
  username    = "admin"
  password    = "A$ecur3P@ssw0rd!"
}
```

## Example Usage

```hcl
provider "rubrik" {}

resource "rubrik_configure_timezone" "LA-Timezone" {
  timezone = "America/Los_Angeles"
  timeout = 15
}
```

Use the navigation to the left to read about the available resources.
