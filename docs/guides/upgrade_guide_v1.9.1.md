---
page_title: "Upgrade Guide: v1.9.1"
---

# Upgrade Guide v1.9.1

## Before Upgrading

Review the [changelog](changelog.md) to understand what has changed and what might cause an issue when upgrading the
provider.

~> **Note:** If you are upgrading across multiple minor versions, review the upgrade guide for each intermediate version
as well. Each guide documents breaking changes and migration steps specific to that release.

## How to Upgrade

### If you are already using the `rubrikinc/rubrik` provider

Make sure that the `version` field is configured in a way which allows Terraform to upgrade to the v1.9.1 release. One
way of doing this is by using the pessimistic constraint operator `~>`, which allows Terraform to upgrade to the latest
release within the same minor version:
```terraform
terraform {
  required_providers {
    rubrik = {
      source  = "rubrikinc/rubrik"
      version = "~> 1.9.1"
    }
  }
}
```
Then upgrade the provider by running:
```shell
% terraform init -upgrade
```
Validate the configuration:
```shell
% terraform plan
```
If you get an error or an unwanted diff, please see the _New Features_ section below for additional instructions.
Otherwise, refresh the state to the v1.9.1 version:
```shell
% terraform apply -refresh-only
```
The rest of this section covers users coming from the `rubrikinc/polaris` provider and does not apply to you.

### If you are coming from the `rubrikinc/polaris` provider

There are two realistic upgrade paths. Pick the one that matches what your configuration uses today.

Note that migration is per-module, not per-resource. The local provider name in `required_providers` dictates the
prefix every resource and data source of that provider must use within the module: a module configured with the local
name `polaris` uses the `polaris` prefix throughout, and a module configured with `rubrik` uses the `rubrik` prefix
throughout. Mixing the two prefixes in a single module is not possible.

#### Option 1: Switch source to `rubrikinc/rubrik` but keep the `polaris` local name

This is the lowest-friction way to move to the renamed provider, and the recommended path for any module that contains
a resource which does not yet support the `moved {}` block (see Option 2 for the list of resources that do). Update
only the `source` field in `required_providers`, leaving the local provider name as `polaris`:
```terraform
terraform {
  required_providers {
    polaris = {
      source  = "rubrikinc/rubrik"
      version = "~> 1.9.1"
    }
  }
}
```
The renamed provider knows about both the `polaris` and `rubrik` resource and data source prefixes, so existing
configurations and state continue to work without changes. Terraform will emit a deprecation warning for each
`polaris` resource or data source you reference, but no state surgery is required.

#### Option 2: Switch source to `rubrikinc/rubrik` and change the local name to `rubrik`

This is the cleaner end state. It is realistic for modules that contain only data sources, only resources that support
`moved {}`, or other resources you are willing to remove from state and re-import. Update both the local name and the
source:
```terraform
terraform {
  required_providers {
    rubrik = {
      source  = "rubrikinc/rubrik"
      version = "~> 1.9.1"
    }
  }
}
```
If your configuration contains an explicit `provider "polaris" {}` block, rename it to `provider "rubrik" {}`.

The following resources support state migration via Terraform's `moved {}` block:

* `polaris_aws_cnp_account` → `rubrik_aws_cnp_account`
* `polaris_aws_cnp_account_attachments` → `rubrik_aws_cnp_account_attachments`
* `polaris_azure_devops_organization` → `rubrik_azure_devops_organization`
* `polaris_custom_role` → `rubrik_custom_role`
* `polaris_role_assignment` → `rubrik_role_assignment`
* `polaris_sso_group` → `rubrik_sso_group`
* `polaris_user` → `rubrik_user`

For each of these resources, rename the `resource` block to use the `rubrik` prefix and add a `moved {}` block
referencing the old and new Terraform addresses. For example, a `polaris_aws_cnp_account` resource named `account` would
become:
```terraform
moved {
  from = polaris_aws_cnp_account.account
  to   = rubrik_aws_cnp_account.account
}

resource "rubrik_aws_cnp_account" "account" {
  # ... existing configuration ...
}
```
Data sources do not have state, so they only need their prefix renamed. For example, a `polaris_aws_cnp_artifacts` data
source named `artifacts` would become:
```terraform
data "rubrik_aws_cnp_artifacts" "artifacts" {
  # ... existing configuration ...
}
```
Any other resource in the module must be removed from state and re-imported, or recreated. This is potentially
destructive — if you are not willing to do this for every such resource in the module, use Option 1 instead.

#### Applying the upgrade

Once you have updated the configuration for whichever option you chose, install the renamed provider by running:
```shell
% terraform init -upgrade
```
Then validate the configuration:
```shell
% terraform plan
```
For Option 1, the plan should show no changes (apart from deprecation warnings for each `polaris` resource and data
source). For Option 2, the plan should show the moved resources with no other changes. If you get an error or an
unwanted diff, see the _New Features_ section below for additional context. Otherwise, proceed by running:
```shell
% terraform apply
```
This will record the renames (Option 2) in state and migrate the local Terraform state to the v1.9.1 version.

## New Features

### Azure DevOps Onboarding

A new `rubrik_azure_devops_organization` resource onboards an Azure DevOps organization to RSC using a customer-supplied
application (non-OAuth). Onboarding has three steps that map to three Terraform objects:

1. Register the customer application for the Azure DevOps use case with a `rubrik_azure_service_principal` resource,
   setting the new `use_case = "AZURE_DEVOPS"` field.
2. Generate the onboarding script with the `rubrik_azure_devops_script` data source and run it against the
   organization out of band. The provider does not run the script — run it with the Azure CLI signed in
   (`az login`) as a Project Collection Administrator in the organization; the script mints a short-lived Azure DevOps
   token from that session, so no personal access token is required.
3. Onboard the organization with the `rubrik_azure_devops_organization` resource.

```terraform
resource "rubrik_azure_service_principal" "devops" {
  app_id        = "25c2b42a-c76b-11eb-9767-6ff6b5b7e72b"
  app_name      = "My DevOps App"
  app_secret    = "<my-apps-secret>"
  tenant_domain = "mydomain.onmicrosoft.com"
  tenant_id     = "2bfdaef8-c76b-11eb-8d3d-4706c14a88f0"
  use_case      = "AZURE_DEVOPS"
}

data "rubrik_azure_devops_permissions" "repo" {
  feature           = "AZURE_DEVOPS_REPOSITORY_PROTECTION"
  permission_groups = ["BASIC", "RECOVERY"]
}

data "rubrik_azure_devops_script" "onboard" {
  org_native_ids = ["my-org"]
  tenant_domain  = rubrik_azure_service_principal.devops.tenant_domain

  feature {
    name              = "AZURE_DEVOPS_REPOSITORY_PROTECTION"
    permission_groups = ["BASIC", "RECOVERY"]
  }
}

resource "rubrik_azure_devops_organization" "org" {
  native_id            = "my-org"
  tenant_domain        = rubrik_azure_service_principal.devops.tenant_domain
  exocompute_host_type = "RUBRIK_HOST"
  exocompute_region    = "eastus"
  storage_type         = "RCV"

  feature {
    name              = "AZURE_DEVOPS_REPOSITORY_PROTECTION"
    permission_groups = ["BASIC", "RECOVERY"]
    permissions       = data.rubrik_azure_devops_permissions.repo.id
  }

  depends_on = [rubrik_azure_service_principal.devops]
}
```

The `use_case` field on `rubrik_azure_service_principal` selects whether the application is registered for cloud native
protection (the default) or Azure DevOps. Credentials are stored separately per use case, so a tenant that uses both
declares one service principal per use case. Omitting the field preserves the existing cloud native protection behavior,
so existing service principal configurations are unaffected.

~> **Note:** Running the onboarding script is an out-of-band step by design — the provider does not execute it. To have
Terraform run it for you, a `null_resource` with a `local-exec` provisioner is one option: set its `triggers` to the
`rubrik_azure_devops_permissions` data source `id`s so it re-runs when the required permissions change, and add a
`depends_on` from the `rubrik_azure_devops_organization` resource so the script runs before onboarding. This runs the
script on the machine executing Terraform, which must be signed in to the Azure CLI as a Project Collection
Administrator. See the `azure_devops` example module in the `terraform-provider-polaris-examples` repository for a
complete configuration.

### Updating Permissions

The permissions RSC requires for an Azure DevOps feature can change over time. The `rubrik_azure_devops_permissions`
data source returns the current permissions for a feature and its permission groups, together with a
`permission_group_versions` map and an `id` that is a hash of the feature, permissions and versions. The `id` changes
whenever RSC updates the required permissions.

Wire the data source's `id` into the `permissions` field of the matching `feature` block on the
`rubrik_azure_devops_organization` resource, as shown above. When RSC changes the required permissions, the `id`
changes and Terraform plans an update to the organization. Before applying, re-run the onboarding script against the
organization (see the `rubrik_azure_devops_script` data source) to grant the new permissions; applying then notifies
RSC that they have been granted.

The `permissions` field is optional — omit it to manage the feature's permission groups without tracking permission
version changes.

### Reading Azure DevOps Objects

Three new data sources read onboarded Azure DevOps objects: `rubrik_azure_devops_organization` (by RSC `id` or
`native_id`), `rubrik_azure_devops_project` and `rubrik_azure_devops_repository` (each by RSC `id` or `name`).

Each exposes the object's RSC ID as its `id` attribute, so it can be assigned an SLA Domain with the
`rubrik_sla_domain_assignment` resource:

```terraform
data "rubrik_azure_devops_repository" "repo" {
  name = "my-repo"
}

data "rubrik_sla_domain" "gold" {
  name = "gold"
}

resource "rubrik_sla_domain_assignment" "repo" {
  sla_domain_id = data.rubrik_sla_domain.gold.id
  object_ids    = [data.rubrik_azure_devops_repository.repo.id]
}
```

The `rubrik_object` data source also gains support for the `AzureDevOpsOrganization`, `AzureDevOpsProject` and
`AzureDevOpsRepository` object types, resolving an object to its RSC ID by name for use with the
`rubrik_sla_domain_assignment` resource. Because project and repository names are only unique within their parent, set
the optional `org_id` (for a project) or `org_id` and/or `project_id` (for a repository) to disambiguate a name shared
across parents:

```terraform
data "rubrik_object" "repo" {
  object_type = "AzureDevOpsRepository"
  name        = "my-repo"
  project_id  = data.rubrik_object.project.id
}
```

### Discovery and Import

A new `rubrik_azure_devops_organization` list resource lists onboarded Azure DevOps organizations. Declare it in a
`.tfquery.hcl` file:

```terraform
list "rubrik_azure_devops_organization" "all" {
  provider = rubrik
}
```

Run `terraform query` to discover organizations, or `terraform query -generate-config-out=generated.tf` to also generate
a `resource` block and a matching `import` block for each one. The per-feature `permissions` signal and the
`delete_snapshots_on_destroy` lifecycle setting are not stored in RSC and are left null; before applying, wire each
feature's `permissions` field to a `rubrik_azure_devops_permissions` data source.

### `moved {}` Block Support

The `rubrik_azure_devops_organization` resource supports Terraform's `moved {}` block. This enables in-place migration
from the deprecated `polaris_azure_devops_organization` resource type to the `rubrik_azure_devops_organization` resource
type without offboarding the organization from RSC and re-onboarding it.
