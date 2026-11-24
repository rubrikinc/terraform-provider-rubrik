data "azuread_client_config" "current" {}

data "azuread_domains" "current" {
  only_initial = true
}

# Create the application credentials that RSC uses to access Azure.
resource "azuread_application" "rsc" {
  display_name            = "Rubrik Security Cloud - Management Group Example"
  prevent_duplicate_names = true
  owners                  = [data.azuread_client_config.current.object_id]
}

resource "azuread_application_password" "rsc" {
  application_id = azuread_application.rsc.id
}

resource "azuread_service_principal" "rsc" {
  client_id = azuread_application.rsc.client_id
  owners    = [data.azuread_client_config.current.object_id]
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
  principal_id         = azuread_service_principal.rsc.object_id
}

resource "azurerm_role_assignment" "rsc_user_access_administrator" {
  scope                = data.azurerm_management_group.prod.id
  role_definition_name = "User Access Administrator"
  principal_id         = azuread_service_principal.rsc.object_id
}

# Register the app once. The management group resource uses the secret stored
# in RSC. Protect Terraform state because the generated password is stored there.
resource "rubrik_azure_service_principal" "default" {
  app_id        = azuread_application.rsc.client_id
  app_name      = azuread_application.rsc.display_name
  app_secret    = azuread_application_password.rsc.value
  tenant_domain = data.azuread_domains.current.domains[0].domain_name
  tenant_id     = azuread_service_principal.rsc.application_tenant_id
}

resource "rubrik_azure_management_group" "prod" {
  tenant_domain       = rubrik_azure_service_principal.default.tenant_domain
  app_id              = rubrik_azure_service_principal.default.app_id
  management_group_id = data.azurerm_management_group.prod.name
  regions             = ["eastus2", "westus2"]
  subscription_ids    = ["33333333-3333-4333-8333-333333333333"]

  depends_on = [
    azurerm_role_assignment.rsc_contributor,
    azurerm_role_assignment.rsc_user_access_administrator,
  ]

  feature = [
    {
      name              = "CLOUD_DISCOVERY"
      permission_groups = ["BASIC"]
    },
    {
      name                  = "CLOUD_NATIVE_PROTECTION"
      permission_groups     = ["BASIC", "EXPORT_AND_RESTORE", "FILE_LEVEL_RECOVERY"]
      resource_group_name   = "rubrik-protection"
      resource_group_region = "eastus2"
    },
    {
      name                  = "CLOUD_NATIVE_ARCHIVAL"
      permission_groups     = ["BASIC"]
      resource_group_name   = "rubrik-archival"
      resource_group_region = "eastus2"
    },
  ]
}
