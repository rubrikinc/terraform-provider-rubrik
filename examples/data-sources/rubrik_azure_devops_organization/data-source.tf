# Look up an onboarded Azure DevOps organization by native ID.
data "rubrik_azure_devops_organization" "org" {
  native_id = "my-org"
}

# Look up by RSC organization ID.
data "rubrik_azure_devops_organization" "by_id" {
  id = "d7f3e5a0-1234-4c5b-9abc-0123456789ab"
}
