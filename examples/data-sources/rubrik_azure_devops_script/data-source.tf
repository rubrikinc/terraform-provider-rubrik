# Generate the Azure DevOps onboarding scripts for one or more organizations.
data "rubrik_azure_devops_script" "onboard" {
  tenant_domain  = "mydomain.onmicrosoft.com"
  org_native_ids = ["my-org", "my-other-org"]

  feature {
    name              = "AZURE_DEVOPS_REPOSITORY_PROTECTION"
    permission_groups = ["BASIC", "RECOVERY"]
  }
}

output "onboarding_powershell_script" {
  value     = data.rubrik_azure_devops_script.onboard.powershell_script
  sensitive = true
}
