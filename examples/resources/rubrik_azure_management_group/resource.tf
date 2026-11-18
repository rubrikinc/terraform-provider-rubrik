variable "rsc_app_object_id" {
  description = "Object ID of the RSC app's service principal."
  type        = string
}

variable "rsc_app_secret" {
  description = "Client secret for the RSC app registration."
  type        = string
  sensitive   = true
}

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
  app_id        = "11111111-1111-4111-8111-111111111111"
  app_name      = "My App"
  app_secret    = var.rsc_app_secret
  tenant_domain = "example.onmicrosoft.com"
  tenant_id     = "22222222-2222-4222-8222-222222222222"
}

resource "rubrik_azure_management_group" "prod" {
  tenant_domain       = rubrik_azure_service_principal.default.tenant_domain
  app_id              = rubrik_azure_service_principal.default.app_id
  management_group_id = "prod-mg"
  regions             = ["eastus2", "westus2"]
  subscription_ids    = ["33333333-3333-4333-8333-333333333333"]

  depends_on = [
    azurerm_role_assignment.rsc_contributor,
    azurerm_role_assignment.rsc_user_access_administrator,
  ]

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
