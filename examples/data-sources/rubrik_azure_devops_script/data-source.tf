# Generate the Azure DevOps onboarding scripts for one or more organizations.
data "rubrik_azure_devops_script" "onboard" {
  tenant_domain  = "mydomain.onmicrosoft.com"
  org_native_ids = ["my-org", "my-other-org"]

  feature {
    name = "AZURE_DEVOPS_PROTECTION"
  }
  feature {
    name = "AZURE_DEVOPS_REPOSITORY_PROTECTION"
  }
}

output "onboarding_powershell_script" {
  value     = data.rubrik_azure_devops_script.onboard.powershell_script
  sensitive = true
}
