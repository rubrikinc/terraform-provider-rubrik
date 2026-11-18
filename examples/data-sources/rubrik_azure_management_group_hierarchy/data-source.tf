# List the subscriptions under a management group and their eligibility.
data "rubrik_azure_management_group_hierarchy" "prod" {
  tenant_domain       = "example.onmicrosoft.com"
  app_id              = "11111111-1111-4111-8111-111111111111"
  management_group_id = "prod-mg"
  features            = ["CLOUD_NATIVE_PROTECTION"]
}

output "eligible_subscriptions" {
  value = data.rubrik_azure_management_group_hierarchy.prod.eligible_subscription_ids
}

# Limit the hierarchy to the immediate children of the management group.
data "rubrik_azure_management_group_hierarchy" "children" {
  tenant_domain       = "example.onmicrosoft.com"
  app_id              = "11111111-1111-4111-8111-111111111111"
  management_group_id = "prod-mg"
  recurse             = false
}
