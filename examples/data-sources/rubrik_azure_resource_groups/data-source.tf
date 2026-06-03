# List every Azure resource group visible to RSC across every managed
# subscription.
data "rubrik_azure_resource_groups" "all" {}

# Narrow the list to a specific set of subscriptions. Resource group names are
# only unique within a subscription, so keying on name should always also branch
# on subscription_id.
data "rubrik_azure_resource_groups" "filtered" {
  subscription_ids = [
    "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  ]
}

# Look up a single resource group by exact name within a subscription.
# The provider sends `name` to RSC as a substring filter for server-side
# narrowing, then keeps only entries with an exact name match before returning,
# so this yields zero or one result.
data "rubrik_azure_resource_groups" "by_name" {
  subscription_ids = [
    "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  ]
  name = "terraform-test"
}

output "terraform_test_rg_id" {
  value = one(data.rubrik_azure_resource_groups.by_name.resource_groups[*].id)
}
