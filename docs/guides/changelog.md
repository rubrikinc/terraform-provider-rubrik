---
page_title: "Changelog"
---

# Changelog

## v1.9.1
* New resource added for `rubrik_gcp_cloud_cluster` which creates a Rubrik Cloud Data Management (CDM) cluster
  with elastic storage (CCES) in GCP using RSC, including Multi-AZ resiliency via the `az_resilient` attribute and
  `subnet_az_config` blocks. The target GCP project must be onboarded to RSC with the `SERVERS_AND_APPS` feature
  enabled. The `admin_email` and `admin_password` fields are write-only, which requires Terraform v1.11.0 or later.
  State can be moved from the `polaris_gcp_cloud_cluster` resource with a `moved` block.
  [[docs](../resources/gcp_cloud_cluster.md)]
* New data source added for `rubrik_gcp_service_accounts` which returns the GCP service accounts RSC has
  discovered for a cloud account, for use with the `rubrik_gcp_cloud_cluster` resource.
  [[docs](../data-sources/gcp_service_accounts.md)]

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
* Add support for `retain_archive_logs_indefinitely` in the `oracle_config` block of the `rubrik_sla_domain` resource. [[docs](../resources/sla_domain.md)]
* Add a computed `backup_type` attribute to the `rubrik_sla_domain` resource, reporting whether an Azure SQL SLA's
  backups are Azure-managed (`NATIVE`, V1) or Rubrik-managed (`RUBRIK`, V2).
* Fix the description of `host_log_retention_unit` in the `oracle_config` block to document `MINUTES` and `HOURS` as valid values. [[docs](../resources/sla_domain.md)]

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
