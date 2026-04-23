---
page_title: "rubrik_gcp_custom_labels Resource - terraform-provider-rubrik"
subcategory: ""
description: |-
  
The `rubrik_gcp_custom_labels` resource manages RSC custom GCP labels.
Simplify your cloud resource management by assigning custom labels for easy
identification. These custom labels will be used on all existing and future GCP
projects in your RSC account.

-> **Note:** The newly updated custom labels will be applied to all existing and
   new resources, while the previously applied labels will remain unchanged.

~> **Warning:** When using multiple `rubrik_gcp_custom_labels` resources in the
   same RSC account, there is a risk of a race condition when the resources are
   destroyed. This can result in custom labels remaining in RSC even after all
   `rubrik_gcp_custom_labels` resources have been destroyed. The race condition
   can be avoided by either managing all custom labels using a single
   `rubrik_gcp_custom_labels` resource or by using `depends_on` to ensure that
   the resources are destroyed in a serial fashion.

~> **Warning:** The `override_resource_labels` field refers to a single global
   value in RSC. So multiple `rubrik_gcp_custom_labels` resources with
   different values for the `override_resource_labels` field will result in a
   perpetual diff.

---

# rubrik_gcp_custom_labels (Resource)


The `rubrik_gcp_custom_labels` resource manages RSC custom GCP labels.
Simplify your cloud resource management by assigning custom labels for easy
identification. These custom labels will be used on all existing and future GCP
projects in your RSC account.

-> **Note:** The newly updated custom labels will be applied to all existing and
   new resources, while the previously applied labels will remain unchanged.

~> **Warning:** When using multiple `rubrik_gcp_custom_labels` resources in the
   same RSC account, there is a risk of a race condition when the resources are
   destroyed. This can result in custom labels remaining in RSC even after all
   `rubrik_gcp_custom_labels` resources have been destroyed. The race condition
   can be avoided by either managing all custom labels using a single
   `rubrik_gcp_custom_labels` resource or by using `depends_on` to ensure that
   the resources are destroyed in a serial fashion.

~> **Warning:** The `override_resource_labels` field refers to a single global
   value in RSC. So multiple `rubrik_gcp_custom_labels` resources with
   different values for the `override_resource_labels` field will result in a
   perpetual diff.



## Example Usage

```terraform
resource "rubrik_gcp_custom_labels" "labels" {
  custom_labels = {
    "app"    = "RSC"
    "vendor" = "Rubrik"
  }
}
```


## Schema

### Required

- `custom_labels` (Map of String) Custom labels to add to cloud resources.

### Optional

- `override_resource_labels` (Boolean) Should custom labels overwrite existing labels with the same keys. Default value is `true`.

### Read-Only

- `id` (String) SHA-256 hash of the string "GCP".

## Import

To import the resource, you need to provide a dummy ID to the import command. This is because the resource manages an
RSC account-level configuration that don't have a unique identifier.

Import is supported using the following syntax:


In Terraform v1.5.0 and later, the [`import` block](https://developer.hashicorp.com/terraform/language/import) can be used with the `id` attribute, for example:

```terraform
import {
  to = rubrik_gcp_custom_labels.labels
  id = "dummy"
}
```



The [`terraform import` command](https://developer.hashicorp.com/terraform/cli/commands/import) can be used, for example:

```terraform
% terraform import rubrik_gcp_custom_labels.labels dummy
```

