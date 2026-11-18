# The app's service principal needs the Contributor and User Access
# Administrator roles at the management group scope. Keep them in place until
# the resource has been destroyed, RSC needs them to remove its role
# assignments.
data "azurerm_management_group" "prod" {
  name = "prod-mg"
}

resource "azurerm_role_assignment" "rsc_contributor" {
  scope                = data.azurerm_management_group.prod.id
  role_definition_name = "Contributor"
  principal_id         = var.rsc_app_object_id
}

resource "azurerm_role_assignment" "rsc_user_access_administrator" {
  scope                = data.azurerm_management_group.prod.id
  role_definition_name = "User Access Administrator"
  principal_id         = var.rsc_app_object_id
}

resource "rubrik_azure_service_principal" "default" {
  app_id        = "25c2b42a-c76b-11eb-9767-6ff6b5b7e72b"
  app_name      = "My App"
  app_secret    = "<my-apps-secret>"
  tenant_domain = "mydomain.onmicrosoft.com"
  tenant_id     = "2bfdaef8-c76b-11eb-8d3d-4706c14a88f0"
}

# Onboard the subscriptions of the management group. Subscriptions added to the
# management group later are not onboarded automatically.
data "rubrik_azure_management_group_hierarchy" "prod" {
  tenant_domain       = rubrik_azure_service_principal.default.tenant_domain
  app_id              = rubrik_azure_service_principal.default.app_id
  management_group_id = "prod-mg"
  features            = ["CLOUD_NATIVE_PROTECTION"]

  depends_on = [
    azurerm_role_assignment.rsc_contributor,
    azurerm_role_assignment.rsc_user_access_administrator,
  ]
}

resource "rubrik_azure_management_group" "prod" {
  tenant_domain       = rubrik_azure_service_principal.default.tenant_domain
  app_id              = rubrik_azure_service_principal.default.app_id
  management_group_id = "prod-mg"
  regions             = ["eastus2", "westus2"]
  subscription_ids    = data.rubrik_azure_management_group_hierarchy.prod.all_subscription_ids

  feature {
    name              = "CLOUD_NATIVE_PROTECTION"
    permission_groups = ["BASIC", "EXPORT_AND_RESTORE", "FILE_LEVEL_RECOVERY"]
  }

  feature {
    name                  = "CLOUD_NATIVE_ARCHIVAL"
    permission_groups     = ["BASIC"]
    resource_group_name   = "rubrik-archival"
    resource_group_region = "eastus2"
  }
}

# With an explicit list of subscriptions.
resource "rubrik_azure_management_group" "dev" {
  tenant_domain       = rubrik_azure_service_principal.default.tenant_domain
  app_id              = rubrik_azure_service_principal.default.app_id
  management_group_id = "dev-mg"
  regions             = ["eastus2"]
  subscription_ids = [
    "7f3b4c19-5a0d-4c4a-9d0e-2b6f3b8c1a11",
    "0c8d2f6e-91a4-4d7b-b2c5-6a1e9f4d3b22",
  ]

  feature {
    name              = "CLOUD_NATIVE_PROTECTION"
    permission_groups = ["BASIC"]
  }

  delete_snapshots_on_destroy = false
}
