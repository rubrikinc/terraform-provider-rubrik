# List the subscriptions under a management group and their eligibility.
data "rubrik_azure_management_group_hierarchy" "prod" {
  tenant_domain       = "mydomain.onmicrosoft.com"
  app_id              = "25c2b42a-c76b-11eb-9767-6ff6b5b7e72b"
  management_group_id = "prod-mg"
  features            = ["CLOUD_NATIVE_PROTECTION"]
}

output "eligible_subscriptions" {
  value = data.rubrik_azure_management_group_hierarchy.prod.eligible_subscription_ids
}

# Limit the hierarchy to the immediate children of the management group.
data "rubrik_azure_management_group_hierarchy" "children" {
  tenant_domain       = "mydomain.onmicrosoft.com"
  app_id              = "25c2b42a-c76b-11eb-9767-6ff6b5b7e72b"
  management_group_id = "prod-mg"
  recurse             = false
}
