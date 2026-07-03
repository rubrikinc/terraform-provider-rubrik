# Look up an Azure subscription by name.
data "rubrik_object" "subscription" {
  name        = "my-subscription"
  object_type = "AzureNativeSubscription"
}

# Look up an Azure resource group. Resource group names are only unique within
# a subscription, so subscription_id is required.
data "rubrik_object" "resource_group" {
  name            = "my-resource-group"
  object_type     = "AzureNativeResourceGroup"
  subscription_id = data.rubrik_object.subscription.id
}

# Look up an Azure DevOps project. Project names are only unique within an
# organization, so set org_id to disambiguate a name shared across organizations.
data "rubrik_object" "project" {
  name        = "my-project"
  object_type = "AzureDevOpsProject"
  org_id      = "a1b2c3d4-1234-4c5b-9abc-0123456789ab"
}
