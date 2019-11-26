---
layout: "rubrik"
page_title: "Rubrik: cluster_version"
sidebar_current: "docs-rubrik-data-source-cluster_version"
description: |-
  Returns the current running version of the Rubrik CDM cluster software.
---

# cluster\_version

Returns the current running version of the Rubrik CDM cluster software.

## Example Usage

```hcl
data "rubrik_cluster_version" "version" {}
```

## Argument Reference

None

## Attributes Reference

The following computed attributes are exported:

* `cluster_version` - The CDM version of the Rubrik cluster.