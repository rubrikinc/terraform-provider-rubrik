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

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/exocompute"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
	gqlexocompute "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/exocompute"
	gqlazure "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/azure"
)

const resourceAzureExocomputeDescription = `
The ´rubrik_azure_exocompute´ resource creates an RSC Exocompute configuration
for Azure workloads.

There are 2 types of Exocompute configurations:
d
 1. *RSC Managed Host* - When a host configuration is created, RSC will
    automatically deploy the necessary resources in the specified Azure region
    to run the Exocompute service. A host configuration can be used by both the
    host cloud account and application cloud accounts mapped to the host
    account.
 2. *Application* - An application configuration is created by mapping the
    application cloud account to a host cloud account. The application cloud
    account will leverage the Exocompute resources deployed for the host
    configuration.

Item 1 above requires that the Azure subscription has been onboarded with the
´EXOCOMPUTE´ feature.

Since there are 2 types of Exocompute configurations, there are 2 ways to create
a ´rubrik_azure_exocompute´ resource:

 1. Using the ´cloud_account_id´, ´region´, ´subnet´ and
   ´pod_overlay_network_cidr´ fields creates an RSC managed host configuration.
 2. Using the ´cloud_account_id´ and ´host_cloud_account_id´ fields creates an
    application configuration.

~> **Note:** A host configuration can be created without specifying the
   ´pod_overlay_network_cidr´ field, this is discouraged and should only be done
   for backwards compatibility reasons.

-> **Note:** Using both host and application Exocompute configurations is
   sometimes referred to as shared Exocompute.

Setting ´trigger_health_check´ to ´true´ makes the provider start an RSC health
check of the Exocompute configuration after the configuration is created, and
wait for the result. Changing ´trigger_health_check´ from ´false´ to ´true´ on
an existing configuration also starts a health check, without replacing the
configuration. Changing it to ´false´ has no effect. Importing a configuration
with ´trigger_health_check´ set to ´true´ starts a health check on the first
apply.

-> **Note:** RSC runs the health check on an RSC managed cluster in the cloud
   account. When there is no running cluster in the region, RSC first launches
   one, so the health check can take around 30 minutes. Use the ´timeouts´ block
   to change how long the provider waits.

~> **Note:** A health check which reports the configuration as anything but
   healthy, a health check which doesn't finish before the timeout, and a health
   check which RSC refuses to start, e.g. because one is already running, are
   reported as warnings. The Exocompute configuration is kept.

To import an application Exocompute configuration, prepend ´app-´ to the ID of
the configuration.
`

// defaultAzureExocomputeTimeout is the default time to wait for the create
// and update operations, which can include a health check.
const defaultAzureExocomputeTimeout = 60 * time.Minute

var (
	_ resource.Resource                     = &azureExocomputeResource{}
	_ resource.ResourceWithConfigure        = &azureExocomputeResource{}
	_ resource.ResourceWithConfigValidators = &azureExocomputeResource{}
	_ resource.ResourceWithIdentity         = &azureExocomputeResource{}
	_ resource.ResourceWithImportState      = &azureExocomputeResource{}
	_ resource.ResourceWithMoveState        = &azureExocomputeResource{}
	_ resource.ResourceWithUpgradeState     = &azureExocomputeResource{}
	_ resource.ResourceWithValidateConfig   = &azureExocomputeResource{}
)

type azureExocomputeResource struct {
	client *client
	prefix string
}

// azureExocomputeModel is the resource state. Field order follows the schema.
type azureExocomputeModel struct {
	ID                    types.String   `tfsdk:"id"`
	CloudAccountID        types.String   `tfsdk:"cloud_account_id"`
	HostCloudAccountID    types.String   `tfsdk:"host_cloud_account_id"`
	OptionalConfig        types.List     `tfsdk:"optional_config"`
	PodOverlayNetworkCIDR types.String   `tfsdk:"pod_overlay_network_cidr"`
	Region                types.String   `tfsdk:"region"`
	Subnet                types.String   `tfsdk:"subnet"`
	SubscriptionID        types.String   `tfsdk:"subscription_id"`
	TriggerHealthCheck    types.Bool     `tfsdk:"trigger_health_check"`
	Timeouts              timeouts.Value `tfsdk:"timeouts"`
}

// azureExocomputeOptionalConfigModel is the element of the optional_config
// block. Field order follows the schema.
type azureExocomputeOptionalConfigModel struct {
	AllowlistAdditionalIPs         types.Set    `tfsdk:"allowlist_additional_ips"`
	AllowlistRubrikIPs             types.Bool   `tfsdk:"allowlist_rubrik_ips"`
	ClusterAccess                  types.String `tfsdk:"cluster_access"`
	ClusterTier                    types.String `tfsdk:"cluster_tier"`
	DiskEncryptionAtHost           types.Bool   `tfsdk:"disk_encryption_at_host"`
	DiskEncryptionSetID            types.String `tfsdk:"disk_encryption_set_id"`
	HealthCheckVMNamePrefix        types.String `tfsdk:"health_check_vm_name_prefix"`
	MaxNodeCount                   types.String `tfsdk:"max_node_count"`
	PostgresPrivateDNSZoneID       types.String `tfsdk:"postgres_private_dns_zone_id"`
	PostgresSubnetID               types.String `tfsdk:"postgres_subnet_id"`
	PrivateExocomputeDNSZoneID     types.String `tfsdk:"private_exocompute_dns_zone_id"`
	ResourceGroupPrefix            types.String `tfsdk:"resource_group_prefix"`
	SnapshotPrivateAccessDNSZoneID types.String `tfsdk:"snapshot_private_access_dns_zone_id"`
	SQLPrivateDNSZoneID            types.String `tfsdk:"sql_private_dns_zone_id"`
	UserDefinedRouting             types.Bool   `tfsdk:"user_defined_routing"`
}

// azureExocomputeOptionalConfigAttrTypes holds the attribute types of the
// optional_config block element.
var azureExocomputeOptionalConfigAttrTypes = map[string]attr.Type{
	keyAllowlistAdditionalIPs:         types.SetType{ElemType: types.StringType},
	keyAllowlistRubrikIPs:             types.BoolType,
	keyClusterAccess:                  types.StringType,
	keyClusterTier:                    types.StringType,
	keyDiskEncryptionAtHost:           types.BoolType,
	keyDiskEncryptionSetID:            types.StringType,
	keyHealthCheckVMNamePrefix:        types.StringType,
	keyMaxNodeCount:                   types.StringType,
	keyPostgresPrivateDNSZoneID:       types.StringType,
	keyPostgresSubnetID:               types.StringType,
	keyPrivateExocomputeDNSZoneID:     types.StringType,
	keyResourceGroupPrefix:            types.StringType,
	keySnapshotPrivateAccessDNSZoneID: types.StringType,
	keySQLPrivateDNSZoneID:            types.StringType,
	keyUserDefinedRouting:             types.BoolType,
}

type azureExocomputeIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

func newAzureExocomputeResource() resource.Resource {
	return &azureExocomputeResource{prefix: keyRubrik}
}

func newPolarisAzureExocomputeResource() resource.Resource {
	return &azureExocomputeResource{prefix: keyPolaris}
}

func (r *azureExocomputeResource) Metadata(ctx context.Context, _ resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "azureExocomputeResource.Metadata")

	res.TypeName = r.prefix + "_" + keyAzureExocompute
}

func (r *azureExocomputeResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "azureExocomputeResource.Schema")

	res.Schema = schema.Schema{
		Version:     1,
		Description: description(resourceAzureExocomputeDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "Exocompute configuration ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyCloudAccountID: schema.StringAttribute{
				Optional: true,
				Description: "RSC cloud account ID. This is the ID of the `rubrik_azure_subscription` resource for " +
					"which the Exocompute service runs. Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyHostCloudAccountID: schema.StringAttribute{
				Optional: true,
				Description: "RSC cloud account ID of the shared exocompute host account. Changing this forces a new " +
					"resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isUUID(),
					stringvalidator.ConflictsWith(
						path.MatchRoot(keyPodOverlayNetworkCIDR),
						path.MatchRoot(keySubnet),
					),
				},
			},
			keyPodOverlayNetworkCIDR: schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(""),
				Description: "The CIDR range assigned to pods when launching Exocompute with the CNI overlay network " +
					"plugin mode. Rubrik recommends a size of /18 or larger. The pod CIDR must not overlap with the " +
					"cluster subnet or any IP ranges used in on-premises networks and other peered VNets. The " +
					"default space assigned by Azure is 10.244.0.0/16. Changing this forces a new resource to be " +
					"created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyRegion: schema.StringAttribute{
				Optional: true,
				Description: "Azure region to run the exocompute service in. Should be specified in the standard " +
					"Azure style, e.g. `eastus`. Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(gqlazure.AllRegionNames()...),
				},
			},
			keySubnet: schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(""),
				Description: "Azure subnet ID of the cluster subnet corresponding to the Exocompute configuration. " +
					"This subnet will be used to allocate IP addresses to the nodes of the cluster. Changing this " +
					"forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keySubscriptionID: schema.StringAttribute{
				Optional: true,
				Description: "RSC cloud account ID. This is the ID of the `rubrik_azure_subscription` resource for " +
					"which the Exocompute service runs. Changing this forces a new resource to be created. " +
					"**Deprecated:** use `cloud_account_id` instead.",
				DeprecationMessage: "use `cloud_account_id` instead.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyTriggerHealthCheck: schema.BoolAttribute{
				Optional: true,
				Description: "Trigger a health check of the Exocompute configuration when it's created, or when the " +
					"field is changed to `true`, and wait for the result. A health check which fails or doesn't " +
					"finish is reported as a warning. Defaults to `false`.",
			},
		},
		Blocks: map[string]schema.Block{
			keyOptionalConfig: schema.ListNestedBlock{
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
				PlanModifiers: []planmodifier.List{
					requiresReplaceOnAddOrRemove(),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyAllowlistAdditionalIPs: schema.SetAttribute{
							ElementType: types.StringType,
							Optional:    true,
							Description: "Allowlist additional IP addresses for the API server on the Kubernetes " +
								"cluster. Requires that the `allowlist_rubrik_ips` field is set to `true`. Changing " +
								"this forces a new resource to be created.",
							PlanModifiers: []planmodifier.Set{
								setplanmodifier.RequiresReplace(),
							},
							Validators: []validator.Set{
								setvalidator.SizeAtLeast(1),
								setvalidator.ValueStringsAre(isIPAddress()),
								setvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName(keyAllowlistRubrikIPs)),
							},
						},
						keyAllowlistRubrikIPs: schema.BoolAttribute{
							Optional: true,
							Computed: true,
							Default:  booldefault.StaticBool(false),
							Description: "Allowlist Rubrik IPs for the API server on the Kubernetes cluster. " +
								"Defaults to `false`. Changing this forces a new resource to be created.",
							PlanModifiers: []planmodifier.Bool{
								boolplanmodifier.RequiresReplace(),
							},
						},
						keyClusterAccess: schema.StringAttribute{
							Optional: true,
							Computed: true,
							Default:  stringdefault.StaticString(string(gqlexocompute.AzureClusterAccessPrivate)),
							Description: "Azure cluster access type. Possible values are " +
								"`AKS_CLUSTER_ACCESS_TYPE_PUBLIC` and `AKS_CLUSTER_ACCESS_TYPE_PRIVATE`. Defaults to " +
								"`AKS_CLUSTER_ACCESS_TYPE_PRIVATE`. Changing this forces a new resource to be created.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
							Validators: []validator.String{
								stringvalidator.OneOf(
									string(gqlexocompute.AzureClusterAccessPrivate),
									string(gqlexocompute.AzureClusterAccessPublic),
								),
							},
						},
						keyClusterTier: schema.StringAttribute{
							Optional: true,
							Computed: true,
							Default:  stringdefault.StaticString(string(gqlexocompute.AzureClusterTierFree)),
							Description: "Azure cluster tier. Possible values are `AKS_CLUSTER_TIER_FREE` and " +
								"`AKS_CLUSTER_TIER_STANDARD`. Defaults to `AKS_CLUSTER_TIER_FREE`. Changing this " +
								"forces a new resource to be created.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
							Validators: []validator.String{
								stringvalidator.OneOf(
									string(gqlexocompute.AzureClusterTierFree),
									string(gqlexocompute.AzureClusterTierStandard),
								),
							},
						},
						keyDiskEncryptionAtHost: schema.BoolAttribute{
							Optional: true,
							Computed: true,
							Default:  booldefault.StaticBool(false),
							Description: "Enable disk encryption at host. Defaults to `false`. Changing this forces " +
								"a new resource to be created.",
							PlanModifiers: []planmodifier.Bool{
								boolplanmodifier.RequiresReplace(),
							},
						},
						// The fields added after v1.11.2 are computed, so a value configured
						// in RSC, e.g. using the RSC UI, does not replace the resource when
						// the field is not set.
						keyDiskEncryptionSetID: schema.StringAttribute{
							Optional: true,
							Computed: true,
							Description: "Azure resource ID of the disk encryption set used to encrypt the disks of " +
								"the cluster nodes with customer managed keys. The disk encryption set must be in the " +
								"same region as the Exocompute configuration. RSC ignores the disk encryption set " +
								"unless the `CNP_AZURE_AKS_CMK_ENABLED` feature flag is enabled for the RSC account, " +
								"and RSC does not validate the disk encryption set when the Exocompute configuration " +
								"is created. To let RSC list the disk encryption sets, add the `ENCRYPTION` " +
								"permission group to the Exocompute feature of the `rubrik_azure_subscription` " +
								"resource. If not set, the value in RSC is kept. Changing this forces a new resource " +
								"to be created.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
								stringplanmodifier.RequiresReplaceIfConfigured(),
							},
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
						// RSC can change the prefix in place, but only with a mutation which
						// replaces all Exocompute configurations of the cloud account, which
						// races with changes to the other configurations in the same apply.
						keyHealthCheckVMNamePrefix: schema.StringAttribute{
							Optional: true,
							Computed: true,
							Description: "Name prefix of the virtual machine RSC launches for the Exocompute health " +
								"check. RSC appends a marker and a UUID to the prefix. The prefix can be at most 22 " +
								"characters long, can only contain letters, digits and hyphens, and must start and " +
								"end with a letter or a digit. When RSC has no prefix, it uses " +
								"`test-aks-connectivity-rk-`. If not set, the value in RSC is kept. Changing this " +
								"forces a new resource to be created.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
								stringplanmodifier.RequiresReplaceIfConfigured(),
							},
							Validators: []validator.String{
								stringvalidator.LengthBetween(1, 22),
								stringvalidator.RegexMatches(regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?$`),
									"must only contain letters, digits and hyphens, and must start and end with a "+
										"letter or a digit"),
							},
						},
						keyMaxNodeCount: schema.StringAttribute{
							Optional: true,
							Computed: true,
							Default:  stringdefault.StaticString(string(gqlexocompute.AzureClusterNodeCountMedium)),
							Description: "The maximum number of nodes each cluster can use. Make sure you have " +
								"enough IP addresses in the subnet or a node pool large enough to handle the number " +
								"of nodes to avoid launch failure. Possible values are `AKS_NODE_COUNT_BUCKET_SMALL` " +
								"(32 nodes), `AKS_NODE_COUNT_BUCKET_MEDIUM` (64 nodes), `AKS_NODE_COUNT_BUCKET_LARGE` " +
								"(128 nodes) and `AKS_NODE_COUNT_BUCKET_XLARGE` (256 nodes). Defaults to " +
								"`AKS_NODE_COUNT_BUCKET_MEDIUM`. Changing this forces a new resource to be created.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
							Validators: []validator.String{
								stringvalidator.OneOf(
									string(gqlexocompute.AzureClusterNodeCountSmall),
									string(gqlexocompute.AzureClusterNodeCountMedium),
									string(gqlexocompute.AzureClusterNodeCountLarge),
									string(gqlexocompute.AzureClusterNodeCountXLarge),
								),
							},
						},
						keyPostgresPrivateDNSZoneID: schema.StringAttribute{
							Optional: true,
							Computed: true,
							Description: "Azure resource ID of the private DNS zone used to resolve the FQDNs of " +
								"RSC managed Azure Postgres Flexible Servers from the Exocompute subnet. The name of " +
								"the zone must end with `.postgres.database.azure.com` and the zone must be linked " +
								"to the Exocompute VNet. Must be specified together with `postgres_subnet_id`. " +
								"Requires the `REL_ENABLE_AZURE_POSTGRES_FLEXIBLE_SERVER` feature flag to be enabled for " +
								"the RSC account. If not set, the value in RSC is kept. Changing this forces a new " +
								"resource to be created.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
								stringplanmodifier.RequiresReplaceIfConfigured(),
							},
							Validators: []validator.String{
								isNotWhiteSpace(),
								stringvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName(keyPostgresSubnetID)),
							},
						},
						keyPostgresSubnetID: schema.StringAttribute{
							Optional: true,
							Computed: true,
							Description: "Azure resource ID of the subnet used for VNet integration of RSC managed " +
								"Azure Postgres Flexible Servers. RSC uses these servers to back up Azure Postgres " +
								"Flexible Servers with private access. The subnet must be in the Exocompute VNet, " +
								"delegated to `Microsoft.DBforPostgreSQL/flexibleServers`, at least `/28` in size, " +
								"and different from the Exocompute subnet. Must be specified together with " +
								"`postgres_private_dns_zone_id`. Requires the `REL_ENABLE_AZURE_POSTGRES_FLEXIBLE_SERVER` " +
								"feature flag to be enabled for the RSC account. If not set, the value in RSC is kept. " +
								"Changing this forces a new resource to be created.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
								stringplanmodifier.RequiresReplaceIfConfigured(),
							},
							Validators: []validator.String{
								isNotWhiteSpace(),
								stringvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName(keyPostgresPrivateDNSZoneID)),
							},
						},
						keyPrivateExocomputeDNSZoneID: schema.StringAttribute{
							Optional: true,
							Computed: true,
							Default:  stringdefault.StaticString(""),
							Description: "Azure resource ID of the private DNS zone which will resolve the API " +
								"server URL for a private cluster. If empty, Azure will automatically create a " +
								"private DNS zone in the node resource group, and will delete it when the AKS " +
								"cluster is deleted. Changing this forces a new resource to be created.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
						keyResourceGroupPrefix: schema.StringAttribute{
							Optional: true,
							Computed: true,
							Default:  stringdefault.StaticString(""),
							Description: "Prefix of resource groups associated with the cluster, such as cluster " +
								"nodes. Changing this forces a new resource to be created.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
						keySnapshotPrivateAccessDNSZoneID: schema.StringAttribute{
							Optional: true,
							Computed: true,
							Default:  stringdefault.StaticString(""),
							Description: "Azure resource ID of the private DNS zone linked to the exocompute VNet, " +
								"which will resolve private endpoints linked to snapshots. If empty, a new private " +
								"DNS zone will be created in the Exocompute resource group. Changing this forces a " +
								"new resource to be created.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
						keySQLPrivateDNSZoneID: schema.StringAttribute{
							Optional: true,
							Computed: true,
							Description: "Azure resource ID of the private DNS zone used to resolve the private " +
								"endpoints of the Azure SQL servers RSC uses to back up Azure SQL Databases. The name " +
								"of the zone must be `privatelink.database.windows.net` and the zone must be linked to " +
								"the Exocompute VNet. RSC only uses the zone when the " +
								"`REL_ENABLE_AZURE_SQL_DB_PRIVATE_ENDPOINT_AUTOMATION` feature flag is enabled for the " +
								"RSC account and the Exocompute feature has the `PRIVATE_ENDPOINTS` permission group. " +
								"If not set, the value in RSC is kept. Changing this forces a new resource to be " +
								"created.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
								stringplanmodifier.RequiresReplaceIfConfigured(),
							},
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
						keyUserDefinedRouting: schema.BoolAttribute{
							Optional: true,
							Computed: true,
							Default:  booldefault.StaticBool(false),
							Description: "Enable user defined routing. This allows the route for the Exocompute " +
								"egress traffic to be configured. Defaults to `false`. Changing this forces a new " +
								"resource to be created.",
							PlanModifiers: []planmodifier.Bool{
								boolplanmodifier.RequiresReplace(),
							},
						},
					},
				},
			},
			keyTimeouts: timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Update: true,
			}),
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use `rubrik_azure_exocompute` instead."
	}
}

func (r *azureExocomputeResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "azureExocomputeResource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Exocompute configuration ID (UUID), prefixed with `app-` for an application configuration.",
			},
		},
	}
}

func (r *azureExocomputeResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	tflog.Trace(ctx, "azureExocomputeResource.ConfigValidators")

	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot(keyCloudAccountID),
			path.MatchRoot(keySubscriptionID),
		),
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot(keyHostCloudAccountID),
			path.MatchRoot(keyRegion),
		),
		resourcevalidator.Conflicting(
			path.MatchRoot(keyHostCloudAccountID),
			path.MatchRoot(keyTriggerHealthCheck),
		),
	}
}

func (r *azureExocomputeResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, res *resource.ValidateConfigResponse) {
	tflog.Trace(ctx, "azureExocomputeResource.ValidateConfig")

	var config azureExocomputeModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	// An unset block is an empty list, not null, so the optional_config
	// conflict can't use a path based validator.
	if !config.HostCloudAccountID.IsNull() && !config.OptionalConfig.IsNull() &&
		(config.OptionalConfig.IsUnknown() || len(config.OptionalConfig.Elements()) > 0) {
		res.Diagnostics.AddAttributeError(path.Root(keyOptionalConfig), "Invalid Attribute Combination",
			"The optional_config block can't be used together with host_cloud_account_id.")
	}

	// A configuration with a region but no subnet is a customer managed
	// configuration, which RSC no longer supports.
	if config.HostCloudAccountID.IsNull() && !config.Region.IsNull() && !config.Region.IsUnknown() &&
		config.Subnet.IsNull() {
		res.Diagnostics.AddAttributeWarning(path.Root(keyRegion), "Customer managed Exocompute is deprecated",
			"Customer managed Exocompute is no longer supported by RSC. Set the subnet field to create an RSC "+
				"managed Exocompute configuration.")
	}
}

func (r *azureExocomputeResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "azureExocomputeResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *azureExocomputeResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "azureExocomputeResource.Create")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	var plan azureExocomputeModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	id := plan.CloudAccountID.ValueString()
	if id == "" {
		id = plan.SubscriptionID.ValueString()
	}
	accountID, err := uuid.Parse(id)
	if err != nil {
		res.Diagnostics.AddError("Invalid cloud account ID", err.Error())
		return
	}

	var optionalConfig []azureExocomputeOptionalConfigModel
	res.Diagnostics.Append(plan.OptionalConfig.ElementsAs(ctx, &optionalConfig, false)...)
	if res.Diagnostics.HasError() {
		return
	}

	timeout, diags := plan.Timeouts.Create(ctx, defaultAzureExocomputeTimeout)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if !plan.HostCloudAccountID.IsNull() {
		hostCloudAccountID, err := uuid.Parse(plan.HostCloudAccountID.ValueString())
		if err != nil {
			res.Diagnostics.AddAttributeError(path.Root(keyHostCloudAccountID), "Invalid host cloud account ID",
				err.Error())
			return
		}

		err = exocompute.Wrap(polarisClient).MapAzureCloudAccount(ctx, accountID, hostCloudAccountID)
		if err != nil {
			res.Diagnostics.AddError("Failed to map application cloud account", err.Error())
			return
		}

		plan.ID = types.StringValue(appCloudAccountPrefix + accountID.String())
	} else {
		var exoConfig exocompute.AzureConfigurationFunc
		region := gqlazure.RegionFromName(plan.Region.ValueString())

		var optional *gqlexocompute.AzureOptionalConfig
		if len(optionalConfig) > 0 {
			config := optionalConfig[0]

			var additionalIPs []string
			res.Diagnostics.Append(config.AllowlistAdditionalIPs.ElementsAs(ctx, &additionalIPs, false)...)
			if res.Diagnostics.HasError() {
				return
			}

			optional = &gqlexocompute.AzureOptionalConfig{
				AdditionalWhitelistIPs:     additionalIPs,
				ClusterAccess:              gqlexocompute.AzureClusterAccess(config.ClusterAccess.ValueString()),
				ClusterTier:                gqlexocompute.AzureClusterTier(config.ClusterTier.ValueString()),
				DiskEncryptionAtHost:       config.DiskEncryptionAtHost.ValueBool(),
				DiskEncryptionSetID:        config.DiskEncryptionSetID.ValueString(),
				ExocomputePrivateDnsZoneID: config.PrivateExocomputeDNSZoneID.ValueString(),
				HealthCheckVMNamePrefix:    config.HealthCheckVMNamePrefix.ValueString(),
				NodeCount:                  gqlexocompute.AzureClusterNodeCount(config.MaxNodeCount.ValueString()),
				NodeRGPrefix:               config.ResourceGroupPrefix.ValueString(),
				PostgresPrivateDnsZoneID:   config.PostgresPrivateDNSZoneID.ValueString(),
				PostgresSubnetID:           config.PostgresSubnetID.ValueString(),
				SnapshotPrivateDnsZoneId:   config.SnapshotPrivateAccessDNSZoneID.ValueString(),
				SQLPrivateDnsZoneID:        config.SQLPrivateDNSZoneID.ValueString(),
				UserDefinedRouting:         config.UserDefinedRouting.ValueBool(),
				WhitelistRubrikIPs:         config.AllowlistRubrikIPs.ValueBool(),
			}
		}

		if plan.PodOverlayNetworkCIDR.ValueString() != "" {
			// RSC managed host configuration with overlay network.
			exoConfig = func(ctx context.Context, cloudAccountID uuid.UUID) (gqlexocompute.CreateAzureConfigurationParams, error) {
				return gqlexocompute.CreateAzureConfigurationParams{
					CloudAccountID:        cloudAccountID,
					IsManagedByRubrik:     true,
					Region:                region.ToCloudAccountRegionEnum(),
					SubnetID:              plan.Subnet.ValueString(),
					PodOverlayNetworkCIDR: plan.PodOverlayNetworkCIDR.ValueString(),
					OptionalConfig:        optional,
				}, nil
			}
		} else if plan.Subnet.ValueString() != "" {
			// RSC managed host configuration.
			exoConfig = func(ctx context.Context, cloudAccountID uuid.UUID) (gqlexocompute.CreateAzureConfigurationParams, error) {
				return gqlexocompute.CreateAzureConfigurationParams{
					CloudAccountID:    cloudAccountID,
					IsManagedByRubrik: true,
					Region:            region.ToCloudAccountRegionEnum(),
					SubnetID:          plan.Subnet.ValueString(),
					OptionalConfig:    optional,
				}, nil
			}
		} else {
			exoConfig = exocompute.AzureBYOKCluster(region)
		}

		exoConfigID, err := exocompute.Wrap(polarisClient).AddAzureConfiguration(ctx, accountID, exoConfig)
		if err != nil {
			res.Diagnostics.AddError("Failed to create Azure Exocompute configuration", err.Error())
			return
		}

		plan.ID = types.StringValue(exoConfigID.String())
	}

	// Set the ID in the state and identity right away, so a failure after
	// this point doesn't orphan the configuration. The optional config fields
	// RSC fills in are unknown in the plan and are set to null, the next read
	// fills them in.
	if len(optionalConfig) > 0 {
		if optionalConfig[0].DiskEncryptionSetID.IsUnknown() {
			optionalConfig[0].DiskEncryptionSetID = types.StringNull()
		}
		if optionalConfig[0].HealthCheckVMNamePrefix.IsUnknown() {
			optionalConfig[0].HealthCheckVMNamePrefix = types.StringNull()
		}
		if optionalConfig[0].PostgresPrivateDNSZoneID.IsUnknown() {
			optionalConfig[0].PostgresPrivateDNSZoneID = types.StringNull()
		}
		if optionalConfig[0].PostgresSubnetID.IsUnknown() {
			optionalConfig[0].PostgresSubnetID = types.StringNull()
		}
		if optionalConfig[0].SQLPrivateDNSZoneID.IsUnknown() {
			optionalConfig[0].SQLPrivateDNSZoneID = types.StringNull()
		}

		list, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: azureExocomputeOptionalConfigAttrTypes}, optionalConfig)
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return
		}

		plan.OptionalConfig = list
	}

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	res.Diagnostics.Append(res.Identity.Set(ctx, azureExocomputeIdentityModel{ID: plan.ID})...)
	if res.Diagnostics.HasError() {
		return
	}

	if plan.TriggerHealthCheck.ValueBool() {
		res.Diagnostics.Append(exocomputeRunHealthCheck(ctx, polarisClient, core.CloudVendorAzure, plan.ID.ValueString(),
			plan.Subnet.ValueString() == "")...)
	}
}

func (r *azureExocomputeResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "azureExocomputeResource.Read")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	var state azureExocomputeModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if strings.HasPrefix(id, appCloudAccountPrefix) {
		appID, err := uuid.Parse(strings.TrimPrefix(id, appCloudAccountPrefix))
		if err != nil {
			res.Diagnostics.AddError("Invalid Exocompute configuration ID", err.Error())
			return
		}

		hostID, err := exocompute.Wrap(polarisClient).AzureHostCloudAccount(ctx, appID)
		if errors.Is(err, graphql.ErrNotFound) {
			res.State.RemoveResource(ctx)
			return
		}
		if err != nil {
			res.Diagnostics.AddError("Failed to read Azure Exocompute configuration", err.Error())
			return
		}

		if state.SubscriptionID.IsNull() {
			state.CloudAccountID = types.StringValue(appID.String())
		} else {
			state.SubscriptionID = types.StringValue(appID.String())
		}
		state.HostCloudAccountID = types.StringValue(hostID.String())
		state.PodOverlayNetworkCIDR = types.StringValue("")
		state.Subnet = types.StringValue("")
	} else {
		exoConfigID, err := uuid.Parse(id)
		if err != nil {
			res.Diagnostics.AddError("Invalid Exocompute configuration ID", err.Error())
			return
		}

		exoConfig, err := exocompute.Wrap(polarisClient).AzureConfigurationByID(ctx, exoConfigID)
		if errors.Is(err, graphql.ErrNotFound) {
			res.State.RemoveResource(ctx)
			return
		}
		if err != nil {
			res.Diagnostics.AddError("Failed to read Azure Exocompute configuration", err.Error())
			return
		}

		if state.SubscriptionID.IsNull() {
			state.CloudAccountID = types.StringValue(exoConfig.CloudAccountID.String())
		} else {
			state.SubscriptionID = types.StringValue(exoConfig.CloudAccountID.String())
		}
		state.Region = types.StringValue(exoConfig.Region.Name())

		// RSC returns an unset string as an empty string, which is kept as an
		// empty string. These attributes default to an empty string, which
		// matches the state written by the SDKv2 resource. The validators
		// reject a configured empty string.
		state.Subnet = types.StringValue(exoConfig.SubnetID)
		state.PodOverlayNetworkCIDR = types.StringValue(exoConfig.PodOverlayNetworkCIDR)

		// An unset block is an empty list, not null.
		state.OptionalConfig = types.ListValueMust(types.ObjectType{AttrTypes: azureExocomputeOptionalConfigAttrTypes}, []attr.Value{})
		if c := exoConfig.OptionalConfig; c != nil {
			config := azureExocomputeOptionalConfigModel{
				AllowlistAdditionalIPs:         types.SetNull(types.StringType),
				AllowlistRubrikIPs:             types.BoolValue(c.WhitelistRubrikIPs),
				ClusterAccess:                  types.StringValue(string(c.ClusterAccess)),
				ClusterTier:                    types.StringValue(string(c.ClusterTier)),
				DiskEncryptionAtHost:           types.BoolValue(c.DiskEncryptionAtHost),
				DiskEncryptionSetID:            types.StringNull(),
				HealthCheckVMNamePrefix:        types.StringNull(),
				MaxNodeCount:                   types.StringValue(string(c.NodeCount)),
				PostgresPrivateDNSZoneID:       types.StringNull(),
				PostgresSubnetID:               types.StringNull(),
				PrivateExocomputeDNSZoneID:     types.StringValue(c.ExocomputePrivateDnsZoneID),
				ResourceGroupPrefix:            types.StringValue(c.NodeRGPrefix),
				SnapshotPrivateAccessDNSZoneID: types.StringValue(c.SnapshotPrivateDnsZoneId),
				SQLPrivateDNSZoneID:            types.StringNull(),
				UserDefinedRouting:             types.BoolValue(c.UserDefinedRouting),
			}
			if len(c.AdditionalWhitelistIPs) > 0 {
				ips, diags := types.SetValueFrom(ctx, types.StringType, c.AdditionalWhitelistIPs)
				res.Diagnostics.Append(diags...)
				if res.Diagnostics.HasError() {
					return
				}
				config.AllowlistAdditionalIPs = ips
			}
			if v := c.DiskEncryptionSetID; v != "" {
				config.DiskEncryptionSetID = types.StringValue(v)
			}
			if v := c.HealthCheckVMNamePrefix; v != "" {
				config.HealthCheckVMNamePrefix = types.StringValue(v)
			}
			if v := c.PostgresPrivateDnsZoneID; v != "" {
				config.PostgresPrivateDNSZoneID = types.StringValue(v)
			}
			if v := c.PostgresSubnetID; v != "" {
				config.PostgresSubnetID = types.StringValue(v)
			}
			if v := c.SQLPrivateDnsZoneID; v != "" {
				config.SQLPrivateDNSZoneID = types.StringValue(v)
			}

			list, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: azureExocomputeOptionalConfigAttrTypes},
				[]azureExocomputeOptionalConfigModel{config})
			res.Diagnostics.Append(diags...)
			if res.Diagnostics.HasError() {
				return
			}

			state.OptionalConfig = list
		}
	}

	// RSC doesn't return trigger_health_check, it is kept as it is.
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.Identity.Set(ctx, azureExocomputeIdentityModel{ID: state.ID})...)
}

func (r *azureExocomputeResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "azureExocomputeResource.Update")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	var plan azureExocomputeModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	var state azureExocomputeModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	// Only trigger_health_check, and the timeouts, can change in place. All
	// other values come from the state, so the plan values of the computed
	// optional config fields are replaced by the state values.
	plan.ID = state.ID
	plan.OptionalConfig = state.OptionalConfig

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	res.Diagnostics.Append(res.Identity.Set(ctx, azureExocomputeIdentityModel{ID: plan.ID})...)
	if res.Diagnostics.HasError() {
		return
	}

	timeout, diags := plan.Timeouts.Update(ctx, defaultAzureExocomputeTimeout)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if plan.TriggerHealthCheck.ValueBool() && !state.TriggerHealthCheck.ValueBool() {
		res.Diagnostics.Append(exocomputeRunHealthCheck(ctx, polarisClient, core.CloudVendorAzure, plan.ID.ValueString(),
			plan.Subnet.ValueString() == "")...)
	}
}

func (r *azureExocomputeResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "azureExocomputeResource.Delete")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	var state azureExocomputeModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if strings.HasPrefix(id, appCloudAccountPrefix) {
		appID, err := uuid.Parse(strings.TrimPrefix(id, appCloudAccountPrefix))
		if err != nil {
			res.Diagnostics.AddError("Invalid Exocompute configuration ID", err.Error())
			return
		}
		err = exocompute.Wrap(polarisClient).UnmapAzureCloudAccount(ctx, appID)
		if err != nil && !errors.Is(err, graphql.ErrNotFound) {
			res.Diagnostics.AddError("Failed to unmap application cloud account", err.Error())
			return
		}
		return
	}

	exoConfigID, err := uuid.Parse(id)
	if err != nil {
		res.Diagnostics.AddError("Invalid Exocompute configuration ID", err.Error())
		return
	}
	err = exocompute.Wrap(polarisClient).RemoveAzureConfiguration(ctx, exoConfigID)
	if err != nil && !errors.Is(err, graphql.ErrNotFound) {
		res.Diagnostics.AddError("Failed to delete Azure Exocompute configuration", err.Error())
		return
	}
}

func (r *azureExocomputeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "azureExocomputeResource.ImportState")

	resource.ImportStatePassthroughWithIdentity(ctx, path.Root(keyID), path.Root(keyID), req, res)
}
