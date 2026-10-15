# List every Azure Native Resource Group RSC knows about, across all
# subscriptions it manages.
data "rubrik_objects" "all_resource_groups" {
  object_type = "AzureNativeResourceGroup"
}

output "resource_group_names" {
  value = data.rubrik_objects.all_resource_groups.objects[*].name
}

# Scope the search to a single subscription.
data "rubrik_azure_subscription" "subscription" {
  subscription_id = "31be1bb0-c76c-11eb-9217-afdffe83a002"
}

data "rubrik_objects" "resource_groups_in_subscription" {
  object_type     = "AzureNativeResourceGroup"
  subscription_id = data.rubrik_azure_subscription.subscription.id
}

# List the AWS Glue Iceberg tables in a database.
data "rubrik_object" "glue_database" {
  name        = "sales"
  object_type = "GlueIcebergDatabase"
}

data "rubrik_objects" "glue_tables" {
  object_type = "GlueIcebergTable"
  parent_id   = data.rubrik_object.glue_database.id
}

output "glue_table_names" {
  value = data.rubrik_objects.glue_tables.objects[*].name
}
