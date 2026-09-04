---
page_title: "Changelog"
---

# Changelog

## v1.10.0
* **Breaking Change:** The `timeouts` block in the `rubrik_object` data source is now a nested attribute, so a custom
  read timeout is written as `timeouts = { read = "10m" }` instead of `timeouts { read = "10m" }`. This is a result of
  migrating the data source to the Terraform Plugin Framework. See the [v1.10.0 upgrade guide](upgrade_guide_v1.10.0.md).
  [[docs](../data-sources/object.md)]
* **Breaking Change:** The `rubrik_object` data source now rejects the `subscription_id`, `org_id` and `project_id`
  fields at plan time when they are set for an `object_type` they do not apply to. Previously these fields were
  silently ignored for other object types. See the [v1.10.0 upgrade guide](upgrade_guide_v1.10.0.md).
  [[docs](../data-sources/object.md)]
* **Breaking Change:** The import ID of the `rubrik_aws_custom_tags`, `rubrik_azure_custom_tags` and
  `rubrik_gcp_custom_labels` resources now identifies the scope to import, instead of being ignored. Pass the cloud
  account ID to import a resource scoped to a single cloud account, or `global` for the scope covering all cloud
  accounts of the cloud vendor. Any other import ID, including the dummy ID previously documented, is rejected. See
  the [v1.10.0 upgrade guide](upgrade_guide_v1.10.0.md).
* **Deprecated:** `cluster_security_group_id` and `node_security_group_id` in the `rubrik_aws_exocompute`
  resource. RSC now always creates and manages the Exocompute security groups for RSC managed configurations.
  Existing configurations continue to work, but the fields will be removed in a future release. See the
  [v1.10.0 upgrade guide](upgrade_guide_v1.10.0.md). [[docs](../resources/aws_exocompute.md)]
* New `rubrik_data_security_policy` resource added which creates and manages data security policies in RSC. A
  policy matches on object conditions, in the `object_filter` block, and identity conditions, in the
  `identity_filter` block, with an optional `threshold_filter` block deciding how many matches raise a violation.
  The blocks mirror the RSC data security policy editor, which is the only filter shape RSC accepts.
  [[docs](../resources/data_security_policy.md)]
* New data source added for `rubrik_data_security_policy` which looks up a data security policy in RSC by name or by
  policy ID. [[docs](../data-sources/data_security_policy.md)]
* New resource added for `rubrik_self_serve_rolling_upgrade` which manages the account-wide self-serve rolling upgrade
  setting in RSC. [[docs](../resources/self_serve_rolling_upgrade.md)]
* New resource added for `rubrik_azure_sql_managed_instance_credentials` which configures the credentials RSC uses to
  back up an Azure SQL Managed Instance server. By default RSC connects to the managed instance using the
  `sql_credentials` block and creates the user it backs up as. Set `setup_script_installed` to `true` when the setup
  script has already created that user, in which case RSC only records which credentials to use, and authenticates
  using Microsoft Entra ID when the managed instance supports it. The credentials are write-only, so they never reach
  Terraform state, change `sql_credential_version` to send them again.
  [[docs](../resources/azure_sql_managed_instance_credentials.md)]
* Add support for the `CloudNativeTagRule` object type in the `rubrik_object` data source, resolving a cloud native
  tag rule to its RSC ID by name for use with the `rubrik_sla_domain_assignment` resource.
  [[docs](../data-sources/object.md)]
* Add support for the `AzureSqlManagedInstanceServer` object type in the `rubrik_object` data source, resolving an
  Azure SQL Managed Instance server to its RSC ID by name for use with the `rubrik_sla_domain_assignment` resource.
  Set `subscription_id` to disambiguate a server name shared across subscriptions.
  [[docs](../data-sources/object.md)]
* Add an `auth_type` attribute to the `rubrik_object` data source, reporting the authentication mechanisms an
  `AzureSqlManagedInstanceServer` supports. It decides which credentials the
  `rubrik_azure_sql_managed_instance_credentials` resource requires, and is null for all other object types.
  [[docs](../data-sources/object.md)]
* No longer require `subscription_id` when `object_type` is `AzureNativeResourceGroup` in the `rubrik_object` data
  source. Set it only to disambiguate a resource group name shared across subscriptions.
  [[docs](../data-sources/object.md)]
* Add support for the `AZURE_POSTGRES_FLEXIBLE_SERVER_PROTECTION` feature in the `rubrik_azure_permissions` data
  source, which backs up Azure Database for PostgreSQL flexible servers. The feature has the `BASIC` and `RECOVERY`
  permission groups. [[docs](../data-sources/azure_permissions.md)]
* Add support for the `postgres_flexible_server_protection` feature in the `rubrik_azure_subscription` resource,
  which enables backup and recovery of Azure Database for PostgreSQL flexible servers. Unlike the other features,
  RSC requires both an Azure resource group and a user-assigned managed identity, and the identity must be in the
  feature's resource group, so those fields are mandatory.
  [[docs](../resources/azure_subscription.md#nested-schema-for-postgres_flexible_server_protection)]
* Add support for the `AzurePostgresFlexibleServer` object type in the `rubrik_object` data source, resolving an
  Azure Postgres flexible server to its RSC ID by name for use with the `rubrik_sla_domain_assignment` resource.
  Set `subscription_id` to disambiguate a server name shared across subscriptions.
  [[docs](../data-sources/object.md)]
* Add support for the `AZURE_POSTGRES_FLEXIBLE_SERVER_OBJECT_TYPE` object type and the
  `azure_postgres_flexible_server_config` block in the `rubrik_sla_domain` resource, allowing an SLA Domain to
  protect Azure Postgres flexible servers and to set the point-in-time restore retention RSC enforces on the source
  server. The object type cannot be combined with other object types and requires a `backup_location`.
  [[docs](../resources/sla_domain.md#nested-schema-for-azure_postgres_flexible_server_config)]
* Fix a bug in the `rubrik_sla_domain` resource where an SLA Domain using the
  `AZURE_POSTGRES_FLEXIBLE_SERVER_OBJECT_TYPE` object type could not be created or updated unless the AWS S3 multiple
  backup locations feature was enabled for the RSC account. The `backup_location` was not passed on, failing with an
  error stating that a `backup_location` is required. [[docs](../resources/sla_domain.md)]
* Fix a bug in the `rubrik_sla_domain` resource where a V2 (Rubrik-managed) Azure SQL Database or Azure SQL Managed
  Instance SLA Domain would additionally send its `backup_location` as an AWS S3 configuration when the AWS S3
  multiple backup locations feature was not enabled for the RSC account. [[docs](../resources/sla_domain.md)]
* The `rubrik_sla_domain` resource now rejects `backup_location` when it is set for object types which do not support
  one. Previously the block was sent as an AWS S3 configuration for any object type, where RSC ignored it.
  [[docs](../resources/sla_domain.md)]
* Add support for excluding tags from snapshots in the `rubrik_aws_custom_tags` and `rubrik_azure_custom_tags`
  resources, and labels in the `rubrik_gcp_custom_labels` resource, through the new `excluded_tags` and
  `excluded_labels` fields. A pattern is either an exact key or a prefix wildcard, such as `temp-*`. As with custom
  tags, a resource only manages the patterns listed in its own configuration and leaves any other patterns in RSC
  untouched. See the [v1.10.0 upgrade guide](upgrade_guide_v1.10.0.md).
* The `custom_tags` field in the `rubrik_aws_custom_tags` and `rubrik_azure_custom_tags` resources, and the
  `custom_labels` field in the `rubrik_gcp_custom_labels` resource, are now optional, allowing a resource to manage
  only excluded tags. When specified, the fields must contain at least one tag or label, as must the `excluded_tags`
  and `excluded_labels` fields. At least one of the two fields must be specified.
* Add support for scoping custom tags and excluded tags to a single cloud account in the `rubrik_aws_custom_tags`,
  `rubrik_azure_custom_tags` and `rubrik_gcp_custom_labels` resources, through the new `cloud_account_id` field. When
  omitted, the tags and labels apply to all cloud accounts of the cloud vendor, as before. RSC keeps the two scopes as
  independent configurations. See the [v1.10.0 upgrade guide](upgrade_guide_v1.10.0.md).
* Migrate the `rubrik_aws_custom_tags` resource to the Terraform Plugin Framework.
* Migrate the `rubrik_azure_custom_tags` resource to the Terraform Plugin Framework.
* Migrate the `rubrik_gcp_custom_labels` resource to the Terraform Plugin Framework.
* Add `moved {}` block support to the `rubrik_aws_custom_tags`, `rubrik_azure_custom_tags` and
  `rubrik_gcp_custom_labels` resources. This enables in-place migration from the deprecated `polaris` prefixed
  resource types to the `rubrik` prefixed resource types via a Terraform `moved {}` block, without removing the
  resources from state and re-importing them. See the [v1.10.0 upgrade guide](upgrade_guide_v1.10.0.md) for migration
  instructions.

## v1.9.1
* New resource added for `rubrik_azure_devops_organization` which onboards an Azure DevOps organization to RSC
  using a customer-supplied application (non-OAuth). [[docs](../resources/azure_devops_organization.md)]
* New data source added for `rubrik_azure_devops_script` which generates the Azure DevOps onboarding scripts to run
  against an organization out of band. [[docs](../data-sources/azure_devops_script.md)]
* New data source added for `rubrik_azure_devops_organization` which reads an onboarded Azure DevOps organization.
  [[docs](../data-sources/azure_devops_organization.md)]
* New data source added for `rubrik_azure_devops_project` which reads an Azure DevOps project.
  [[docs](../data-sources/azure_devops_project.md)]
* New data source added for `rubrik_azure_devops_repository` which reads an Azure DevOps repository.
  [[docs](../data-sources/azure_devops_repository.md)]
* New data source added for `rubrik_azure_devops_permissions` which returns the permissions RSC requires for an
  Azure DevOps feature and its permission groups, along with the version of each permission group.
  [[docs](../data-sources/azure_devops_permissions.md)]
* New list resource added for `rubrik_azure_devops_organization` which lists onboarded Azure DevOps organizations
  for discovery and bulk import. [[docs](../list-resources/azure_devops_organization.md)]
* Add `moved {}` block support to the `rubrik_azure_devops_organization` resource. This enables in-place migration
  from the deprecated `polaris` prefixed resource type to the `rubrik` prefixed resource type via a Terraform
  `moved {}` block, without offboarding the organization from RSC and re-onboarding it. See the
  [v1.9.1 upgrade guide](upgrade_guide_v1.9.1.md) for migration instructions.
* Add support for `use_case` in the `rubrik_azure_service_principal` resource, selecting whether the service
  principal is registered for cloud native protection (default) or Azure DevOps. The credentials are stored
  separately per use case. [[docs](../resources/azure_service_principal.md)]
* Add support for the `AzureDevOpsOrganization`, `AzureDevOpsProject` and `AzureDevOpsRepository` object types in
  the `rubrik_object` data source, resolving an Azure DevOps object to its RSC ID by name for use with the
  `rubrik_sla_domain_assignment` resource. [[docs](../data-sources/object.md)]
* New resource added for `rubrik_gcp_cloud_cluster` which creates a Rubrik Cloud Data Management (CDM) cluster
  with elastic storage (CCES) in GCP using RSC, including Multi-AZ resiliency via the `az_resilient` attribute and
  `subnet_az_config` blocks. The target GCP project must be onboarded to RSC with the `SERVERS_AND_APPS` feature
  enabled. The `admin_email` and `admin_password` fields are write-only, which requires Terraform v1.11.0 or later.
  State can be moved from the `polaris_gcp_cloud_cluster` resource with a `moved` block.
  [[docs](../resources/gcp_cloud_cluster.md)]
* New data source added for `rubrik_gcp_regions` which returns the GCP regions and their availability zones
  RSC supports for a cloud account, for use with the `rubrik_gcp_cloud_cluster` resource.
  [[docs](../data-sources/gcp_regions.md)]
* New data source added for `rubrik_github_organization` which reads an onboarded GitHub organization.
  [[docs](../data-sources/github_organization.md)]
* New data source added for `rubrik_github_repository` which reads a GitHub repository.
  [[docs](../data-sources/github_repository.md)]
* Add support for the `GitHubOrganization` and `GitHubRepository` object types in the `rubrik_object` data source,
  resolving a GitHub object to its RSC ID by name for use with the `rubrik_sla_domain_assignment` resource.
  [[docs](../data-sources/object.md)]
* Add support for the `GITHUB_OBJECT_TYPE` object type in the `rubrik_sla_domain` resource, allowing GitHub objects
  to be protected by an SLA Domain. [[docs](../resources/sla_domain.md)]
* Add support for looking up the `rubrik_azure_devops_organization` data source by `name`, in addition to `id` and
  `native_id`. [[docs](../data-sources/azure_devops_organization.md)]
* Refresh the documentation templates for the `rubrik_aws_cloud_cluster`, `rubrik_azure_cloud_cluster`,
  `rubrik_gcp_cloud_cluster`, `rubrik_aws_exocompute`, `rubrik_cdm_bootstrap`,
  `rubrik_cdm_bootstrap_cces_aws`, `rubrik_aws_cnp_account_trust_policy` and `rubrik_sla_domain` resources,
  and the `rubrik_azure_archival_location` and `rubrik_sla_domain` data sources, so that the published
  documentation matches the current schema. These templates hold their schema documentation inline, so it
  does not update automatically. Notably, `admin_email` and `admin_password` on the cloud cluster resources
  are now shown as write-only.

## v1.9.0
* **Breaking Change:** When the `CNP_AZURE_SQL_SLA_REVAMP` feature is enabled for the account, Azure SQL Database and
  Managed Instance SLAs in the `rubrik_sla_domain` resource follow the new V1/V2 model: a V2 (Rubrik-managed) SLA
  specifies a `backup_location` block instead of the top-level `archival` block, and the previous requirement that an
  Azure SQL Database SLA include an instant-archival location no longer applies. Accounts without the feature enabled
  are unaffected and keep the existing behavior. See the [v1.9.0 upgrade guide](upgrade_guide_v1.9.0.md).
  [[docs](../resources/sla_domain.md)]
* New resource added for `rubrik_aws_account_managed` which onboards an RSC-managed AWS account (Rubrik-hosted
  BaaS) up to the point of deploying the CloudFormation stack. It validates and registers the account with RSC and
  exports the CloudFormation `template_url` and `stack_name` needed to deploy the RSC cross-account stack with the
  AWS provider. Features and regions are chosen here and default to the full BaaS-supported set when omitted.
  [[docs](../resources/aws_account_managed.md)]
* New resource added for `rubrik_aws_account_managed_stack` which completes onboarding of an RSC-managed AWS
  account after its CloudFormation stack has been deployed. It waits for the account's features to connect and
  finishes BaaS onboarding, re-completes onboarding when RSC raises a permission version, and disables the
  account's features on destroy. [[docs](../resources/aws_account_managed_stack.md)]
* New data source added for `rubrik_objects` which returns every RSC hierarchy object matching a given
  `object_type`, without filtering by name. Only the `AzureNativeResourceGroup` object type is supported so far,
  optionally scoped to a single subscription via `subscription_id`. [[docs](../data-sources/objects.md)]
* Add support for V1 (Azure-managed, long-term retention) Azure SQL SLAs in the `rubrik_sla_domain` resource via a new
  `ltr_config` block in the `azure_sql_database_config` and `azure_sql_managed_instance_config` blocks, with weekly,
  monthly, and yearly retention. A V1 SLA omits the Rubrik snapshot schedule and backup location. Requires the
  `CNP_AZURE_SQL_SLA_REVAMP` feature. [[docs](../resources/sla_domain.md)]
* Add support for combining the Azure SQL Database and Azure SQL Managed Instance object types in a single
  `rubrik_sla_domain` (they may be combined with each other only, not with other object types), matching RSC.
  [[docs](../resources/sla_domain.md)]
* Add support for `retain_archive_logs_indefinitely` in the `oracle_config` block of the `rubrik_sla_domain` resource.
  [[docs](../resources/sla_domain.md)]
* Add a computed `backup_type` attribute to the `rubrik_sla_domain` resource, reporting whether an Azure SQL SLA's
  backups are Azure-managed (`NATIVE`, V1) or Rubrik-managed (`RUBRIK`, V2).
* Fix the description of `host_log_retention_unit` in the `oracle_config` block to document `MINUTES` and `HOURS` as
  valid values. [[docs](../resources/sla_domain.md)]

## v1.8.2
* **Breaking Change:** The `rubrik_custom_role` resource now requires the `VIEW_CLUSTER_REFERENCE` permission
  operation to be granted alongside `VIEW_CLUSTER`. RSC automatically adds `VIEW_CLUSTER_REFERENCE` whenever
  `VIEW_CLUSTER` is granted, so granting `VIEW_CLUSTER` alone resulted in perpetual drift. `VIEW_CLUSTER_REFERENCE`
  may still be granted on its own. See the [v1.8.2 upgrade guide](upgrade_guide_v1.8.2.md).
* Fix a bug in the `rubrik_data_center_archival_location_amazon_s3` resource where the `cloud_compute_settings`
  block was read from the `archival_proxy_settings` configuration. Specifying an `archival_proxy_settings` block
  caused the provider to crash, and any `cloud_compute_settings` values were silently ignored.

## v1.8.1
* Add support for the `SERVERS_AND_APPS` feature in the `rubrik_gcp_project` resource and the `rubrik_gcp_project`
  and `rubrik_gcp_permissions` data sources. The feature uses the `CLOUD_CLUSTER_ES` permission group and, unlike
  other GCP features, does not use the `BASIC` permission group.
  [[docs](../resources/gcp_project.md)]
* Fix a bug in the `rubrik_aws_cnp_account_attachments` resource where the deprecated `features` field, when omitted
  from the configuration, could be left as an unknown value after apply, causing Terraform to fail with a "Provider
  returned invalid result object after apply" error. The field is now populated from the cloud account during create.
* Fix ROLE_CHAINING handling in the `rubrik_aws_cnp_account` and `rubrik_aws_cnp_account_attachments` resources and the
  `rubrik_aws_cnp_permissions` data source. Role-chaining accounts surface the role under the `ROLE_CHAINING` artifact
  key instead of `CROSSACCOUNT`; see the [v1.8.1 upgrade guide](upgrade_guide_v1.8.1.md) for the expected one-time diff.

## v1.8.0
* Add support for the `AzureNativeResourceGroup` object type in the `polaris_object` data source. Pair with the
  new `subscription_id` field to resolve an Azure resource group to its RSC ID by `(subscription_id, name)`.
  [[docs](../data-sources/object.md)]
* New data source added for `rubrik_aws_permission_groups` which returns the permission groups available for a
  single RSC AWS feature, along with the IAM action statements that each permission group requires. Useful for
  programmatically discovering the available permission groups (for example, the `BASIC` and `RECOVERY` split on
  `RDS_PROTECTION`) at plan time.
  [[docs](../data-sources/aws_permission_groups.md)]
* New data source added for `rubrik_azure_permission_groups` which returns the permission groups available for a
  single RSC Azure feature, along with the Azure RBAC actions and data actions each permission group requires.
  Statements are tagged with their scope (`subscription` or `resource_group`) and kind (`action` or
  `data_action`). Useful for programmatically discovering the available permission groups at plan time.
  [[docs](../data-sources/azure_permission_groups.md)]
* Add support for Multi-AZ resiliency in the `rubrik_aws_cloud_cluster` and `rubrik_azure_cloud_cluster` resources.
  The new `az_resilient` field enables deploying clusters across multiple availability zones, and the new
  `subnet_az_config` block in `vm_config` specifies per-zone subnet mappings.
  [[docs](../resources/aws_cloud_cluster.md)] [[docs](../resources/azure_cloud_cluster.md)]
* Add write-only attributes for `admin_email` and `admin_password` in the `cluster_config` block of the
  `rubrik_aws_cloud_cluster` and `rubrik_azure_cloud_cluster` resources. The credentials are only consumed during
  initial cluster creation and are no longer persisted to state. Requires Terraform v1.11.0 or later.
  [[docs](../resources/aws_cloud_cluster.md)] [[docs](../resources/azure_cloud_cluster.md)]
* **Deprecated:** `features` field in the `rubrik_aws_cnp_account_attachments` resource. The set of features (and
  their permission groups) is now read from the cloud account managed by `rubrik_aws_cnp_account` when artifacts are
  registered, so this field no longer needs to track them. The field is retained for backwards compatibility and
  will be removed in a future major release.
* Add support for the `RECOVERY` permission group in the `RDS_PROTECTION` and `CLOUD_NATIVE_DYNAMODB_PROTECTION`
  features in the `rubrik_aws_account`, `rubrik_aws_cnp_account` and `rubrik_aws_cnp_account_attachments`
  resources. `RECOVERY` grants the elevated AWS permissions required to perform recovery operations.
* Deprecate the `rubrik_aws_cnp_account_trust_policy` resource. Use the `trust_policies` field of the
  `rubrik_aws_cnp_account` resource instead.
* Migrate the `rubrik_aws_account` data source to the Terraform Plugin Framework.
* Migrate the `rubrik_aws_cnp_account` resource to the Terraform Plugin Framework.
* Migrate the `rubrik_aws_cnp_account_attachments` resource to the Terraform Plugin Framework.
* Migrate the `rubrik_aws_cnp_artifacts` data source to the Terraform Plugin Framework.
* Migrate the `rubrik_aws_cnp_permissions` data source to the Terraform Plugin Framework.
* Add `moved {}` block support to the `rubrik_aws_cnp_account` and `rubrik_aws_cnp_account_attachments` resources.
  This enables in-place migration from the deprecated `polaris` prefixed resource types to the `rubrik` prefixed
  resource types via a Terraform `moved {}` block, without removing the resources from state and re-importing them.
  See the [v1.8.0 upgrade guide](upgrade_guide_v1.8.0.md) for migration instructions.
* Add Terraform search support for the `rubrik_aws_cnp_account` resource. Enables `terraform query` to discover AWS
  accounts onboarded via the AWS IAM roles workflow in RSC, including accounts not managed by Terraform. Supports
  filtering by account name and AWS account ID.
* Add Terraform search support for the `rubrik_aws_cnp_account_attachments` resource. Enables `terraform query` to
  discover AWS account attachments onboarded via the AWS IAM roles workflow in RSC, including attachments not managed by
  Terraform.
* New resource added for `rubrik_cluster_settings` which manages the CDM package download and upgrade lifecycle of a
  Rubrik cluster registered with RSC, including automatic multi-hop upgrades through intermediate releases.
  [[docs](../resources/cluster_settings.md)]
* Add Terraform search support for the `rubrik_cluster_settings` resource. Enables `terraform query` to discover the
  upgrade and download state of Rubrik clusters registered with RSC, including clusters not managed by Terraform.
  Supports filtering by cluster name and installed version.
* New data source added for `rubrik_cluster_settings` which returns the upgrade state of a single Rubrik cluster
  registered with RSC. [[docs](../data-sources/cluster_settings.md)]
* New data source added for `rubrik_cluster_versions` which lists the CDM releases available to a Rubrik cluster, for
  driving upgrades of the `rubrik_cluster_settings` resource. [[docs](../data-sources/cluster_versions.md)]

## v1.7.0
* Rename the provider from `rubrikinc/polaris` to `rubrikinc/rubrik`. All resources and data sources are now named with
  the `rubrik` prefix. The `polaris` prefixed names are kept as deprecated aliases for backwards compatibility and
  will be removed in a future release. See the [Upgrade Guide](upgrade_guide_v1.7.0.md) for migration instructions.
* Add `moved {}` block support to the `rubrik_custom_role`, `rubrik_role_assignment`, `rubrik_sso_group` and
  `rubrik_user` resources. This enables migration from the deprecated `polaris` prefixed resource types to the
  `rubrik` prefixed resource types via a Terraform `moved {}` block.
* Rename the provider environment variables from `RUBRIK_POLARIS_*` to `RUBRIK_*`. The `RUBRIK_POLARIS_*` variants
  continue to work via fallback. Likewise, `TF_LOG_PROVIDER_POLARIS` is replaced by `TF_LOG_PROVIDER_RUBRIK` (Terraform
  derives this from the provider name automatically) and `TF_LOG_PROVIDER_POLARIS_API` by `TF_LOG_PROVIDER_RUBRIK_API`.
* Add Terraform search support for the `polaris_custom_role` resource. Enables `terraform query` to discover custom
  roles in RSC, including roles not managed by Terraform.
* Add Terraform search support for the `polaris_user` resource. Enables `terraform query` to discover users in RSC,
  including users not managed by Terraform.
* Add Terraform search support for the `polaris_sso_group` resource. Enables `terraform query` to discover SSO groups
  in RSC, including groups not managed by Terraform. Supports filtering by name and auth domain ID.
* Improve handling of optional retention fields in the object-specific configuration blocks of the
  `polaris_sla_domain` resource (`sap_hana_config`, `db2_config`, `mssql_config`, `oracle_config`, `mongo_config`,
  `managed_volume_config`, `postgres_db_cluster_config`, `mysql_config`, `informix_config`, `gcp_cloud_sql_config`).
  Omitted retention fields are now left out of the API request instead of being sent with empty values.
* Improve state refresh for the `sap_hana_config`, `db2_config`, `oracle_config` and `informix_config` blocks of the
  `polaris_sla_domain` resource. Optional retention unit fields now mirror the schema default when the matching
  duration is unset, and the `storage_snapshot_config` block in `sap_hana_config` is only emitted when it has data.
  This removes spurious diffs after apply.
* Set the default for `log_archival_method` in the `db2_config` block of the `polaris_sla_domain` resource to
  `LOGARCHMETH1`, matching the RSC backend default. Previously, omitting the field produced a drift on subsequent
  plans because the API returned `LOGARCHMETH1` while the schema treated the field as unset.
