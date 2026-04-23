---
page_title: "rubrik_sso_group Data Source - terraform-provider-rubrik"
subcategory: ""
description: |-
  
The `rubrik_sso_group` data source is used to access information about an SSO
group in RSC. An SSO group is looked up using either the ID or the name.

---

# rubrik_sso_group (Data Source)


The `rubrik_sso_group` data source is used to access information about an SSO
group in RSC. An SSO group is looked up using either the ID or the name.



## Example Usage

```terraform
# Look up SSO group by name.
data "rubrik_sso_group" "admins" {
  name = "Administrators"
}

# Look up SSO group by ID.
data "rubrik_sso_group" "admins" {
  sso_group_id = "<id>"
}
```


## Schema

### Optional

- `name` (String) SSO group name.
- `sso_group_id` (String) SSO group ID.

### Read-Only

- `domain_name` (String) The domain name of the SSO group.
- `id` (String) SSO group ID.
- `roles` (Set of Object) Roles assigned to the SSO group. (see [below for nested schema](#nestedatt--roles))
- `users` (Set of Object) Users in the SSO group. (see [below for nested schema](#nestedatt--users))

<a id="nestedatt--roles"></a>
### Nested Schema for `roles`

Read-Only:

- `id` (String) Role ID (UUID).
- `name` (String) Role name.

<a id="nestedatt--users"></a>
### Nested Schema for `users`

Read-Only:

- `email` (String) User email address.
- `id` (String) User ID.
