---
page_title: "rubrik_aws_custom_tags Resource - terraform-provider-rubrik"
subcategory: ""
description: |-
  
The `rubrik_aws_custom_tags` resource manages RSC custom AWS tags. Simplify
your cloud resource management by assigning custom tags for easy identification.
These custom tags will be used on all existing and future AWS accounts in your
RSC account.

-> **Note:** The newly updated custom tags will be applied to all existing and
   new resources, while the previously applied tags will remain unchanged.

~> **Warning:** When using multiple `rubrik_aws_custom_tags` resources in the
   same RSC account, there is a risk of a race condition when the resources are
   destroyed. This can result in custom tags remaining in RSC even after all
   `rubrik_aws_custom_tags` resources have been destroyed. The race condition
   can be avoided by either managing all custom tags using a single
   `rubrik_aws_custom_tags` resource or by using the `depends_on` field to
   ensure that the resources are destroyed in a serial fashion.

~> **Warning:** The `override_resource_tags` field refers to a single global
   value in RSC. So multiple `rubrik_aws_custom_tags` resources with different
   values for the `override_resource_tags` field will result in a perpetual
   diff.

---

# rubrik_aws_custom_tags (Resource)


The `rubrik_aws_custom_tags` resource manages RSC custom AWS tags. Simplify
your cloud resource management by assigning custom tags for easy identification.
These custom tags will be used on all existing and future AWS accounts in your
RSC account.

-> **Note:** The newly updated custom tags will be applied to all existing and
   new resources, while the previously applied tags will remain unchanged.

~> **Warning:** When using multiple `rubrik_aws_custom_tags` resources in the
   same RSC account, there is a risk of a race condition when the resources are
   destroyed. This can result in custom tags remaining in RSC even after all
   `rubrik_aws_custom_tags` resources have been destroyed. The race condition
   can be avoided by either managing all custom tags using a single
   `rubrik_aws_custom_tags` resource or by using the `depends_on` field to
   ensure that the resources are destroyed in a serial fashion.

~> **Warning:** The `override_resource_tags` field refers to a single global
   value in RSC. So multiple `rubrik_aws_custom_tags` resources with different
   values for the `override_resource_tags` field will result in a perpetual
   diff.



## Example Usage

```terraform
resource "rubrik_aws_custom_tags" "tags" {
  custom_tags = {
    "app"    = "RSC"
    "vendor" = "Rubrik"
  }
}
```


## Schema

### Required

- `custom_tags` (Map of String) Custom tags to add to cloud resources.

### Optional

- `override_resource_tags` (Boolean) Should custom tags overwrite existing tags with the same keys. Default value is `true`.

### Read-Only

- `id` (String) SHA-256 hash of the string "AWS".

## Import

To import the resource, you need to provide a dummy ID to the import command. This is because the resource manages an
RSC account-level configuration that don't have a unique identifier.

Import is supported using the following syntax:


In Terraform v1.5.0 and later, the [`import` block](https://developer.hashicorp.com/terraform/language/import) can be used with the `id` attribute, for example:

```terraform
import {
  to = rubrik_aws_custom_tags.tags
  id = "dummy"
}
```



The [`terraform import` command](https://developer.hashicorp.com/terraform/cli/commands/import) can be used, for example:

```terraform
% terraform import rubrik_aws_custom_tags.tags dummy
```

