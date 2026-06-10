---
page_title: "rubrik_role_template Data Source - terraform-provider-rubrik"
subcategory: ""
description: |-
  
The `rubrik_role_template` data source is used to access information about an
RSC role template. A role template is looked up using either the ID or the name.

Role templates are most commonly used as input to the `rubrik_custom_role`
resource — iterating over `permission` on the template seeds the custom role
with the template's permission set, which can then be extended or trimmed in
the resource block.

-> **Note:** The available template names can be found in the RSC UI under
   _Settings / Users and Access / Roles / Create Role_.

---

# rubrik_role_template (Data Source)


The `rubrik_role_template` data source is used to access information about an
RSC role template. A role template is looked up using either the ID or the name.

Role templates are most commonly used as input to the `rubrik_custom_role`
resource — iterating over `permission` on the template seeds the custom role
with the template's permission set, which can then be extended or trimmed in
the resource block.

-> **Note:** The available template names can be found in the RSC UI under
   _Settings / Users and Access / Roles / Create Role_.



## Example Usage

```terraform
# Look up role template by name.
data "rubrik_role_template" "compliance_auditor" {
  name = "Compliance Auditor"
}

# Look up role template by ID.
data "rubrik_role_template" "compliance_auditor" {
  role_template_id = "00000000-0000-0000-0000-000000000007"
}
```


## Schema

### Optional

- `name` (String) Role template name.
- `role_template_id` (String) Role template ID (UUID).

### Read-Only

- `description` (String) Role template description.
- `id` (String) Role template ID (UUID).
- `permission` (Set of Object) Role template permission. (see [below for nested schema](#nestedatt--permission))

<a id="nestedatt--permission"></a>
### Nested Schema for `permission`

Read-Only:

- `hierarchy` (Set of Object) Snappable hierarchy. (see [below for nested schema](#nestedobjatt--permission--hierarchy))
- `operation` (String) Operation allowed on object IDs under the snappable hierarchy.

<a id="nestedobjatt--permission--hierarchy"></a>
### Nested Schema for `permission.hierarchy`

Read-Only:

- `object_ids` (Set of String) Object/workload identifiers.
- `snappable_type` (String) Snappable/workload type.
