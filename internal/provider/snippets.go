// Copyright 2026 Rubrik, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to
// deal in the Software without restriction, including without limitation the
// rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
// sell copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.

package provider

import "text/template"

// docSnippets contains the shared documentation snippets that can be inserted
// into resource and data source descriptions using the template action, e.g.
// {{template "awsPermissionGroups"}}. Snippets are expanded by description.
var docSnippets = template.Must(template.New("snippets").Parse(
	awsPermissionGroupsSnippet +
		azurePermissionGroupsSnippet +
		gcpPermissionGroupsSnippet,
))

const awsPermissionGroupsSnippet = `
{{- define "awsPermissionGroups" -}}
## Permission Groups
Following is a list of features and their applicable permission groups. These
are used when specifying the feature set.

´CLOUD_DISCOVERY´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.

´CLOUD_NATIVE_ARCHIVAL´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.

´CLOUD_NATIVE_PROTECTION´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.
  * ´DOWNLOAD_FILE´ - Represents the set of permissions required to download
    files from snapshots.
  * ´EXPORT_POWER_OFF´ - Represents the set of permissions required to export
    EC2 instances and leave them powered off.
  * ´EXPORT_POWER_ON´ - Represents the set of permissions required to export
    EC2 instances and power them on.
  * ´RESTORE´ - Represents the set of permissions required to restore from
    snapshots.

´CLOUD_NATIVE_DYNAMODB_PROTECTION´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.
  * ´RECOVERY´ - Represents the set of elevated permissions required to perform
    recovery operations.

´CLOUD_NATIVE_S3_PROTECTION´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.
  * ´EXPORT´ - Represents the set of permissions required to export an S3
    recovery to a newly created target bucket.
  * ´RECOVERY´ - Represents the set of elevated permissions required to perform
    recovery operations.

´EXOCOMPUTE´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.
  * ´RSC_MANAGED_CLUSTER´ - Represents the set of permissions required for the
    Rubrik-managed Exocompute cluster.

´KUBERNETES_PROTECTION´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.

´RDS_PROTECTION´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.
  * ´RECOVERY´ - Represents the set of elevated permissions required to perform
    recovery operations.

´ROLE_CHAINING´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.

´SERVERS_AND_APPS´
  * ´CLOUD_CLUSTER_ES´ - Represents the basic set of permissions required to
    onboard the feature.

-> **Note:** When permission groups are specified, the ´BASIC´ permission group
   is always required except for the ´SERVERS_AND_APPS´ feature.

-> **Note:** The ´EXPORT´ and ´RECOVERY´ permission groups of the
   ´CLOUD_NATIVE_S3_PROTECTION´ feature are only available once S3 recovery has
   been enabled for the RSC account. Use the ´rubrik_aws_permission_groups´
   data source to read the permission groups currently available for a feature.
{{- end -}}
`

const azurePermissionGroupsSnippet = `
{{- define "azurePermissionGroups" -}}
## Permission Groups
Following is a list of features and their applicable permission groups. These
are used when specifying the feature.

´AZURE_POSTGRES_FLEXIBLE_SERVER_PROTECTION´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.
  * ´RECOVERY´ - Represents the set of permissions required for all recovery
    operations.

´AZURE_SQL_DB_PROTECTION´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.
  * ´RECOVERY´ - Represents the set of permissions required for all recovery
    operations.
  * ´BACKUP_V2´ - Represents the set of permissions required for immutable
    backup V2 operations.

´AZURE_SQL_MI_PROTECTION´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.
  * ´RECOVERY´ - Represents the set of permissions required for all recovery
    operations.
  * ´BACKUP_V2´ - Represents the set of permissions required for immutable
    backup V2 operations.

´CLOUD_DISCOVERY´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.

´CLOUD_NATIVE_ARCHIVAL´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.
  * ´ENCRYPTION´ - Represents the set of permissions required for encryption
    operation.
  * ´SQL_ARCHIVAL´ - Represents the permissions required to enable Azure AD
    authorization to store Azure SQL and MI snapshots in an archival location.

´CLOUD_NATIVE_ARCHIVAL_ENCRYPTION´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.
  * ´ENCRYPTION´ - Represents the set of permissions required for encryption
    operation.

´CLOUD_NATIVE_BLOB_PROTECTION´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.
  * ´RECOVERY´ - Represents the set of permissions required for all recovery
    operations.

´CLOUD_NATIVE_PROTECTION´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.
  * ´EXPORT_AND_RESTORE´ - Represents the set of permissions required for export
    and restore operations.
  * ´FILE_LEVEL_RECOVERY´ - Represents the set of permissions required for
    file-level recovery operations.
  * ´SNAPSHOT_PRIVATE_ACCESS´ - Represents the set of permissions required for
    private access to disk snapshots.
  * ´EXPORT_AND_RESTORE_POWER_OFF_VM´ - Represents the set of permissions
    required for export and restore operations with VM power off capability.

´SERVERS_AND_APPS´
  * ´CLOUD_CLUSTER_ES´ - Represents the basic set of permissions required to
    onboard the feature.
  * ´SAP_HANA_SS_BASIC´ - Represents the basic set of permissions required for
    SAP HANA snapshot support.
  * ´SAP_HANA_SS_RECOVERY´ - Represents the set of permissions required for SAP
    HANA recovery operations.

´EXOCOMPUTE´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.
  * ´PRIVATE_ENDPOINTS´ - Represents the set of permissions required for usage
    of private endpoints.
  * ´CUSTOMER_MANAGED_BASIC´ - Represents the permissions required to enable
    customer-managed Exocompute feature.
  * ´AKS_CUSTOM_PRIVATE_DNS_ZONE´ - Represents the permissions required for AKS
    custom private DNS zone configuration.
  * ´SERVICE_ENDPOINT_AUTOMATION´ - Represents the permissions required for
    service endpoint automation.
  * ´AUTOMATED_NETWORKING_SETUP´ - Represents the permissions required for
    automated networking setup.

-> **Note:** When permission groups are specified, the ´BASIC´ permission group
   is always required.
{{- end -}}
`

const gcpPermissionGroupsSnippet = `
{{- define "gcpPermissionGroups" -}}
## Permission Groups
Following is a list of features and their applicable permission groups. These
are used when specifying the feature.

´CLOUD_NATIVE_ARCHIVAL´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.
  * ´ENCRYPTION´ - Represents the set of permissions required for encryption
    operation.

´CLOUD_NATIVE_PROTECTION´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.
  * ´EXPORT_AND_RESTORE´ - Represents the set of permissions required for export
    and restore operations.
  * ´FILE_LEVEL_RECOVERY´ - Represents the set of permissions required for
    file-level recovery operations.

´CLOUD_SQL_PROTECTION´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.
  * ´EXPORT_AND_RESTORE´ - Represents the set of permissions required for export
    and restore operations.

-> **Note:** RSC runs Cloud SQL archival and archived recovery on Exocompute,
   which additionally requires the ´CLOUDSQL´ permission group on the
   ´EXOCOMPUTE´ feature. When Exocompute uses a VPC network in a shared VPC host
   project, the ´CLOUDSQL´ permission group is also required on the
   ´GCP_SHARED_VPC_HOST´ feature of the host project.

´GCP_SHARED_VPC_HOST´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.
  * ´CLOUDSQL´ - Represents the set of permissions required to configure
    Private Service Access on the shared VPC host project for Cloud SQL
    protection.

´EXOCOMPUTE´
  * ´BASIC´ - Represents the basic set of permissions required to onboard the
    feature.
  * ´AUTOMATED_NETWORKING_SETUP´ - Represents the set of permissions required
    for automated networking setup. When automated networking setup is enabled,
    RSC is responsible for creating and maintaining the networking resources for
    Exocompute. See the ´rubrik_gcp_exocompute´ resource for more information.
  * ´CLOUDSQL´ - Represents the set of permissions required for Cloud SQL
    archival and archived recovery operations, covering Private Service Access
    networking and the temporary Cloud SQL instances RSC creates. Requires
    Cloud SQL protection to be enabled for the RSC account.

´SERVERS_AND_APPS´
  * ´CLOUD_CLUSTER_ES´ - Represents the set of permissions required to onboard
    the feature.

-> **Note:** When permission groups are specified, the ´BASIC´ permission group
   is always required, except for ´SERVERS_AND_APPS´ which only supports the
   ´CLOUD_CLUSTER_ES´ permission group and does not use ´BASIC´.
{{- end -}}
`
