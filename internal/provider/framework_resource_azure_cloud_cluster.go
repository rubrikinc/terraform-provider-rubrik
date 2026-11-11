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
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/cloudcluster"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/cluster"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	gqlcloudcluster "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/cloudcluster"
	gqlcluster "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/cluster"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core/secret"
	azureRegion "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/azure"
)

const resourceAzureCloudClusterDescription = `
The ´rubrik_azure_cloud_cluster´ resource creates an Azure cloud cluster using RSC.

This resource creates a Rubrik Cloud Data Management (CDM) cluster with elastic storage
in Azure using the specified configuration. The cluster will be deployed with the specified
number of nodes, instance types, and network configuration.

~> **Note:** This resource creates actual Azure infrastructure. Destroying the
   resource removes the cluster from RSC, the cloud VMs are left running and must be
   cleaned up manually.

~> **Note:** The Azure subscription must be onboarded to RSC with the Server and Apps
   feature enabled before creating a cloud cluster.

~> **Note:** This resource requires **Terraform v1.11.0 or later** due to the use of write-only attributes for
   ´admin_email´ and ´admin_password´.

~> **Note:** When destroying this resource, the cluster is removed from RSC. If the cluster has
   blocking conditions (active SLAs, global SLAs, or RCV locations), the removal fails and the
   conditions must be resolved first. ´force_cluster_delete_on_destroy´ only applies to
   disconnected clusters with blocking conditions, and must be applied before the resource is
   destroyed.
`

// azureCloudClusterInstanceTypes are the instance type names RSC accepts for
// Azure cloud clusters.
var azureCloudClusterInstanceTypes = []gqlcloudcluster.AzureCCESSupportedInstanceType{
	gqlcloudcluster.AzureInstanceTypeStandardDS5V2,
	gqlcloudcluster.AzureInstanceTypeStandardD16SV5,
	gqlcloudcluster.AzureInstanceTypeStandardD8SV5,
	gqlcloudcluster.AzureInstanceTypeStandardD32SV5,
	gqlcloudcluster.AzureInstanceTypeStandardE16SV5,
	gqlcloudcluster.AzureInstanceTypeStandardD8ASV5,
	gqlcloudcluster.AzureInstanceTypeStandardD16ASV5,
	gqlcloudcluster.AzureInstanceTypeStandardD32ASV5,
	gqlcloudcluster.AzureInstanceTypeStandardE16ASV5,
}

var (
	_ resource.Resource                   = &azureCloudClusterResource{}
	_ resource.ResourceWithConfigure      = &azureCloudClusterResource{}
	_ resource.ResourceWithIdentity       = &azureCloudClusterResource{}
	_ resource.ResourceWithImportState    = &azureCloudClusterResource{}
	_ resource.ResourceWithMoveState      = &azureCloudClusterResource{}
	_ resource.ResourceWithValidateConfig = &azureCloudClusterResource{}
)

type azureCloudClusterResource struct {
	client *client
	prefix string
}

type azureCloudClusterModel struct {
	ID             types.String              `tfsdk:"id"`
	CloudAccountID types.String              `tfsdk:"cloud_account_id"`
	AZResilient    types.Bool                `tfsdk:"az_resilient"`
	ClusterConfig  []azureClusterConfigModel `tfsdk:"cluster_config"`
	VMConfig       []azureVMConfigModel      `tfsdk:"vm_config"`
	Timeouts       timeouts.Value            `tfsdk:"timeouts"`
}

type azureClusterConfigModel struct {
	cloudClusterCommonConfigModel
}

type azureVMConfigModel struct {
	CDMVersion                   types.String `tfsdk:"cdm_version"`
	CDMProduct                   types.String `tfsdk:"cdm_product"`
	InstanceType                 types.String `tfsdk:"instance_type"`
	ResourceGroupName            types.String `tfsdk:"resource_group_name"`
	StorageAccountName           types.String `tfsdk:"storage_account_name"`
	ContainerName                types.String `tfsdk:"container_name"`
	EnableImmutability           types.Bool   `tfsdk:"enable_immutability"`
	UserAssignedManagedIdentity  types.String `tfsdk:"user_assigned_managed_identity_name"`
	Region                       types.String `tfsdk:"region"`
	NetworkResourceGroup         types.String `tfsdk:"network_resource_group"`
	VnetResourceGroup            types.String `tfsdk:"vnet_resource_group"`
	Subnet                       types.String `tfsdk:"subnet"`
	Vnet                         types.String `tfsdk:"vnet"`
	NetworkSecurityGroup         types.String `tfsdk:"network_security_group"`
	NetworkSecurityResourceGroup types.String `tfsdk:"network_security_resource_group"`
	VMType                       types.String `tfsdk:"vm_type"`
	AvailabilityZone             types.String `tfsdk:"availability_zone"`
	// SubnetAzConfig is a types.List so it can hold an unknown value, see the
	// GCP cloud cluster resource.
	SubnetAzConfig types.List `tfsdk:"subnet_az_config"`
}

func newAzureCloudClusterResource() resource.Resource {
	return &azureCloudClusterResource{prefix: keyRubrik}
}

func newPolarisAzureCloudClusterResource() resource.Resource {
	return &azureCloudClusterResource{prefix: keyPolaris}
}

func (r *azureCloudClusterResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "azureCloudClusterResource.Metadata")

	res.TypeName = r.prefix + "_" + keyAzureCloudCluster
}

func (r *azureCloudClusterResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "azureCloudClusterResource.Schema")

	instanceTypes := make([]string, len(azureCloudClusterInstanceTypes))
	for i, t := range azureCloudClusterInstanceTypes {
		instanceTypes[i] = string(t)
	}
	vmTypes := make([]string, len(cloudClusterVMTypes))
	for i, t := range cloudClusterVMTypes {
		vmTypes[i] = string(t)
	}

	// requiredString returns a required, create-time string attribute which
	// can't be blank.
	requiredString := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{
			Required:      true,
			Description:   desc + " Changing this forces a new resource to be created.",
			PlanModifiers: []planmodifier.String{cloudClusterRequiresReplaceString()},
			Validators:    []validator.String{isNotWhiteSpace()},
		}
	}

	res.Schema = schema.Schema{
		Description: description(resourceAzureCloudClusterDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:      true,
				Description:   "Cloud cluster ID (UUID).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			keyCloudAccountID: schema.StringAttribute{
				Required:      true,
				Description:   "RSC cloud account ID (UUID). Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{cloudClusterRequiresReplaceString()},
				Validators:    []validator.String{isUUID()},
			},
			// No default: SDKv2 state holds null when the attribute is not
			// configured, a default of false would plan a change from null.
			keyAzResilient: schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether to deploy the cluster across multiple availability zones for AZ resiliency. When enabled, `subnet_az_config` blocks must be specified in `vm_config` instead of a single `subnet` and `availability_zone`. Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown(), cloudClusterRequiresReplaceBool()},
			},
		},
		Blocks: map[string]schema.Block{
			keyClusterConfig: schema.ListNestedBlock{
				Description: "Configuration for the cloud cluster.",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
					listvalidator.SizeAtMost(1),
					listvalidator.IsRequired(),
				},
				NestedObject: schema.NestedBlockObject{Attributes: cloudClusterCommonConfigAttributes()},
			},
			keyVMConfig: schema.ListNestedBlock{
				Description: "VM configuration for the cluster nodes.",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
					listvalidator.SizeAtMost(1),
					listvalidator.IsRequired(),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyCDMVersion: requiredString("CDM version to use. This is a create-time input, it is not updated when the " +
							"cluster is upgraded outside of Terraform."),
						keyCDMProduct: schema.StringAttribute{
							Computed:      true,
							Description:   "CDM Product Code. This is a read-only field and computed based on the CDM version when the cluster is created.",
							PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
						},
						keyInstanceType: schema.StringAttribute{
							Required: true,
							Description: fmt.Sprintf("Azure instance type for the cluster nodes. Changing this forces a new resource to be created. %s.",
								possibleValues(azureCloudClusterInstanceTypes)),
							PlanModifiers: []planmodifier.String{cloudClusterRequiresReplaceString()},
							Validators:    []validator.String{stringvalidator.OneOf(instanceTypes...)},
						},
						keyResourceGroupName:  requiredString("Azure resource group name where the cluster will be deployed."),
						keyStorageAccountName: requiredString("Azure storage account name for the cluster."),
						keyContainerName:      requiredString("Azure storage container name for the cluster."),
						keyEnableImmutability: schema.BoolAttribute{
							Required:      true,
							Description:   "Whether to enable immutability for the storage account. Changing this forces a new resource to be created.",
							PlanModifiers: []planmodifier.Bool{cloudClusterRequiresReplaceBool()},
						},
						keyUserAssignedManagedIdentityName: requiredString("Name of the user-assigned managed identity."),
						keyRegion: schema.StringAttribute{
							Required:      true,
							Description:   "Azure region to deploy the cluster in. The format should be the native Azure format, e.g. `eastus`, `westus`, etc. Changing this forces a new resource to be created.",
							PlanModifiers: []planmodifier.String{cloudClusterRequiresReplaceString()},
							Validators:    []validator.String{stringvalidator.OneOf(azureRegion.AllRegionNames()...)},
						},
						keyNetworkResourceGroup: requiredString("Azure resource group name for network resources."),
						keyVnetResourceGroup:    requiredString("Azure resource group name for the virtual network."),
						keySubnet: schema.StringAttribute{
							Optional:      true,
							Description:   "Azure subnet name for the cluster nodes. Required when `az_resilient` is false. Changing this forces a new resource to be created.",
							PlanModifiers: []planmodifier.String{cloudClusterRequiresReplaceString()},
							Validators:    []validator.String{isNotWhiteSpace()},
						},
						keyVnet:                         requiredString("Azure virtual network name."),
						keyNetworkSecurityGroup:         requiredString("Azure network security group name."),
						keyNetworkSecurityResourceGroup: requiredString("Azure resource group name for the network security group."),
						keyVMType: schema.StringAttribute{
							Optional: true,
							Computed: true,
							Default:  stringdefault.StaticString(string(gqlcloudcluster.CCVmConfigDense)),
							Description: fmt.Sprintf("VM type for the cluster. Changing this forces a new resource to be created. %s. "+
								"`EXTRA_DENSE` is recommended for CCES.", possibleValues(cloudClusterVMTypes)),
							PlanModifiers: []planmodifier.String{cloudClusterRequiresReplaceString()},
							Validators:    []validator.String{stringvalidator.OneOf(vmTypes...)},
						},
						keyAvailabilityZone: schema.StringAttribute{
							Optional:      true,
							Description:   "Availability zone for the cluster, if this is not specified, the cluster will be deployed in availability zone 1. Used for single-AZ deployments. Changing this forces a new resource to be created.",
							PlanModifiers: []planmodifier.String{cloudClusterRequiresReplaceString()},
						},
					},
					Blocks: map[string]schema.Block{
						keySubnetAzConfigs: cloudClusterSubnetAzConfigBlock("Subnet name for this availability zone."),
					},
				},
			},
			keyTimeouts: timeouts.Block(ctx, timeouts.Opts{Create: true, Read: true}),
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use `rubrik_azure_cloud_cluster` instead."
	}
}

func (r *azureCloudClusterResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "azureCloudClusterResource.IdentitySchema")

	res.IdentitySchema = cloudClusterIdentitySchema()
}

func (r *azureCloudClusterResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "azureCloudClusterResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *azureCloudClusterResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, res *resource.ValidateConfigResponse) {
	tflog.Trace(ctx, "azureCloudClusterResource.ValidateConfig")

	var config azureCloudClusterModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(validateAzureCloudClusterConfig(config)...)
}

// validateAzureCloudClusterConfig holds the plan-time, client-free validation
// rules: the az_resilient / subnet / availability_zone / subnet_az_config
// either-or. A null az_resilient is false.
//
// Checks are skipped for any value that is unknown at plan time, because
// reading it would yield a zero value and reject an otherwise-valid config.
func validateAzureCloudClusterConfig(config azureCloudClusterModel) diag.Diagnostics {
	var diags diag.Diagnostics

	if len(config.VMConfig) == 0 || config.AZResilient.IsUnknown() {
		return diags
	}
	vm := config.VMConfig[0]
	azResilient := config.AZResilient.ValueBool()

	subnetAzKnown := !vm.SubnetAzConfig.IsUnknown()
	hasSubnetAzConfigs := subnetAzKnown && !vm.SubnetAzConfig.IsNull() && len(vm.SubnetAzConfig.Elements()) > 0
	subnetKnown := !vm.Subnet.IsUnknown()
	hasSubnet := subnetKnown && vm.Subnet.ValueString() != ""
	hasAvailabilityZone := !vm.AvailabilityZone.IsUnknown() && vm.AvailabilityZone.ValueString() != ""

	if azResilient {
		if subnetAzKnown && !hasSubnetAzConfigs {
			diags.AddAttributeError(path.Root(keyVMConfig), "subnet_az_config required",
				"`subnet_az_config` is required in `vm_config` when `az_resilient` is true.")
		}
		if hasSubnet {
			diags.AddAttributeError(path.Root(keyVMConfig), "subnet not allowed",
				"`subnet` cannot be specified in `vm_config` when `az_resilient` is true, use `subnet_az_config` instead.")
		}
		if hasAvailabilityZone {
			diags.AddAttributeError(path.Root(keyVMConfig), "availability_zone not allowed",
				"`availability_zone` cannot be specified in `vm_config` when `az_resilient` is true, use `subnet_az_config` instead.")
		}
	} else {
		if hasSubnetAzConfigs {
			diags.AddAttributeError(path.Root(keyVMConfig), "subnet_az_config not allowed",
				"`subnet_az_config` cannot be specified in `vm_config` when `az_resilient` is false.")
		}
		if subnetKnown && !hasSubnet {
			diags.AddAttributeError(path.Root(keyVMConfig), "subnet required",
				"`subnet` is required in `vm_config` when `az_resilient` is false.")
		}
	}

	return diags
}

func (r *azureCloudClusterResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "azureCloudClusterResource.Create")

	var plan azureCloudClusterModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	// admin_email and admin_password are write-only, so they are null in the
	// plan; read them from the config instead.
	var config azureCloudClusterModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(cloudClusterCheckBlocks(len(plan.ClusterConfig), len(plan.VMConfig))...)
	res.Diagnostics.Append(cloudClusterCheckBlocks(len(config.ClusterConfig), len(config.VMConfig))...)
	if res.Diagnostics.HasError() {
		return
	}

	timeout, diags := plan.Timeouts.Create(ctx, defaultCloudClusterCreateTimeout)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	input, diags := r.buildCreateInput(ctx, plan, config)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	azureCluster, err := cloudcluster.Wrap(polarisClient).CreateAzureCloudCluster(ctx, input)
	if err != nil {
		res.Diagnostics.AddError("Failed to create Azure cloud cluster", err.Error())
		return
	}

	// The cluster exists in RSC from here on. Saving its ID before reading it
	// back makes Terraform keep track of it if the read fails.
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyID), azureCluster.ID.String())...)
	res.Diagnostics.Append(res.Identity.SetAttribute(ctx, path.Root(keyID), azureCluster.ID.String())...)
	if res.Diagnostics.HasError() {
		return
	}

	plan.ID = types.StringValue(azureCluster.ID.String())
	if plan.AZResilient.IsUnknown() {
		plan.AZResilient = types.BoolValue(input.IsAzResilient)
	}

	// The CDM product is computed once, here. Read never looks it up, that
	// would need the subscription and marketplace calls on every refresh. The
	// value is the product version of the configured CDM version, which is what
	// the state of existing clusters holds.
	vm := &plan.VMConfig[0]
	vm.CDMProduct = types.StringNull()
	cdmVersions, err := gqlcloudcluster.Wrap(polarisClient.GQL).AllAzureCdmVersions(ctx, input.CloudAccountID, input.VMConfig.Location)
	if err != nil {
		res.Diagnostics.AddWarning("Failed to look up the CDM product", err.Error())
	} else {
		for _, version := range cdmVersions {
			if version.CDMVersion == vm.CDMVersion.ValueString() {
				vm.CDMProduct = types.StringValue(version.Version)
				break
			}
		}
		if vm.CDMProduct.IsNull() {
			res.Diagnostics.AddWarning("Failed to look up the CDM product",
				fmt.Sprintf("CDM version %q was not found in the versions available for the region.", vm.CDMVersion.ValueString()))
		}
	}

	// Read back the cluster and apply the timezone and location, the create API
	// can't set them. Failures are warnings, the cluster exists.
	res.Diagnostics.Append(cloudClusterFinishCreate(ctx, gqlcluster.Wrap(polarisClient.GQL), azureCluster.ID,
		&plan.ClusterConfig[0].cloudClusterCommonConfigModel)...)

	// Write-only attributes must be null in state.
	plan.ClusterConfig[0].AdminEmail = types.StringNull()
	plan.ClusterConfig[0].AdminPassword = types.StringNull()

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
}

func (r *azureCloudClusterResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "azureCloudClusterResource.Read")

	var state azureCloudClusterModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid cloud cluster ID", err.Error())
		return
	}

	// The blocks are missing from the state of an imported cluster, RSC can't
	// return most of what they hold. The values RSC can return are read below,
	// and the rest is adopted from the configuration by the plan modifiers.
	imported := cloudClusterIsImported(ctx, req.Private)
	if len(state.ClusterConfig) == 0 || len(state.VMConfig) == 0 {
		if !imported {
			res.Diagnostics.Append(cloudClusterCheckBlocks(len(state.ClusterConfig), len(state.VMConfig))...)
			return
		}
		state.ClusterConfig = []azureClusterConfigModel{{
			cloudClusterCommonConfigModel: cloudClusterCommonConfigModel{
				ClusterName:                 types.StringNull(),
				AdminEmail:                  types.StringNull(),
				AdminPassword:               types.StringNull(),
				NumNodes:                    types.Int64Null(),
				DNSNameServers:              types.SetNull(types.StringType),
				DNSSearchDomains:            types.SetNull(types.StringType),
				NTPServers:                  types.SetNull(types.StringType),
				KeepClusterOnFailure:        types.BoolNull(),
				ForceClusterDeleteOnDestroy: types.BoolValue(false),
				Timezone:                    types.StringNull(),
				Location:                    types.StringNull(),
			},
		}}
		state.VMConfig = []azureVMConfigModel{{
			CDMVersion:                   types.StringNull(),
			CDMProduct:                   types.StringNull(),
			InstanceType:                 types.StringNull(),
			ResourceGroupName:            types.StringNull(),
			StorageAccountName:           types.StringNull(),
			ContainerName:                types.StringNull(),
			EnableImmutability:           types.BoolNull(),
			UserAssignedManagedIdentity:  types.StringNull(),
			Region:                       types.StringNull(),
			NetworkResourceGroup:         types.StringNull(),
			VnetResourceGroup:            types.StringNull(),
			Subnet:                       types.StringNull(),
			Vnet:                         types.StringNull(),
			NetworkSecurityGroup:         types.StringNull(),
			NetworkSecurityResourceGroup: types.StringNull(),
			VMType:                       types.StringNull(),
			AvailabilityZone:             types.StringNull(),
			SubnetAzConfig:               types.ListNull(cloudClusterSubnetAzObjectType),
		}}
	}

	timeout, diags := state.Timeouts.Read(ctx, defaultCloudClusterReadTimeout)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	_, err = cluster.Wrap(polarisClient).Cluster(ctx, id)
	if errors.Is(err, graphql.ErrNotFound) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read Azure cloud cluster", err.Error())
		return
	}

	// vm_config.cdm_version and vm_config.cdm_product are create-time values and
	// are deliberately not refreshed, that would plan a replacement of the
	// cluster after an upgrade outside of Terraform. After an import the
	// cdm_version is left null and adopted from the configuration, the installed
	// version may differ from the version the cluster was created with. No
	// subscription or marketplace calls are made here.
	vm := &state.VMConfig[0]
	// SDKv2 stored an empty string for an unset optional string.
	if vm.Subnet.ValueString() == "" {
		vm.Subnet = types.StringNull()
	}
	if vm.AvailabilityZone.ValueString() == "" {
		vm.AvailabilityZone = types.StringNull()
	}

	res.Diagnostics.Append(cloudClusterReadNetwork(ctx, gqlcluster.Wrap(polarisClient.GQL), id,
		&state.ClusterConfig[0].cloudClusterCommonConfigModel)...)
	if res.Diagnostics.HasError() {
		return
	}

	// Write-only attributes must be null in state.
	state.ClusterConfig[0].AdminEmail = types.StringNull()
	state.ClusterConfig[0].AdminPassword = types.StringNull()

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.Identity.SetAttribute(ctx, path.Root(keyID), state.ID)...)
}

func (r *azureCloudClusterResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "azureCloudClusterResource.Update")

	var plan, state azureCloudClusterModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(cloudClusterCheckBlocks(len(plan.ClusterConfig), len(plan.VMConfig))...)
	res.Diagnostics.Append(cloudClusterCheckBlocks(len(state.ClusterConfig), len(state.VMConfig))...)
	if res.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid cloud cluster ID", err.Error())
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}
	gqlClusterAPI := gqlcluster.Wrap(polarisClient.GQL)

	// Only the network settings and the cluster settings can be changed in
	// place. The other attributes only change in place when they are adopted
	// from the configuration after an import, which needs no API call.
	planCC := &plan.ClusterConfig[0].cloudClusterCommonConfigModel
	_, diags := cloudClusterUpdateNetwork(ctx, gqlClusterAPI, id, planCC, &state.ClusterConfig[0].cloudClusterCommonConfigModel)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	// Refresh the computed values. The values just applied are kept, RSC can
	// normalize them, e.g. the location, which would fail the apply with an
	// inconsistent result error. Drift is picked up by Read.
	planned := *planCC
	res.Diagnostics.Append(cloudClusterDowngrade(ctx, cloudClusterReadNetwork(ctx, gqlClusterAPI, id, planCC))...)
	cloudClusterKeepPlanned(planned, planCC)
	if planCC.Timezone.IsUnknown() {
		planCC.Timezone = types.StringNull()
	}
	if planCC.Location.IsUnknown() {
		planCC.Location = types.StringNull()
	}
	if planCC.DNSSearchDomains.IsUnknown() {
		planCC.DNSSearchDomains = types.SetNull(types.StringType)
	}
	if plan.VMConfig[0].CDMProduct.IsUnknown() {
		plan.VMConfig[0].CDMProduct = types.StringNull()
	}
	if plan.AZResilient.IsUnknown() {
		plan.AZResilient = types.BoolNull()
	}

	// The configured values are adopted into the state by this update, the
	// cluster is no longer treated as imported.
	res.Diagnostics.Append(res.Private.SetKey(ctx, cloudClusterImportedKey, nil)...)

	// Write-only attributes must be null in state.
	plan.ClusterConfig[0].AdminEmail = types.StringNull()
	plan.ClusterConfig[0].AdminPassword = types.StringNull()

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.Identity.SetAttribute(ctx, path.Root(keyID), plan.ID)...)
}

func (r *azureCloudClusterResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "azureCloudClusterResource.Delete")

	var state azureCloudClusterModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid cloud cluster ID", err.Error())
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	force := false
	if len(state.ClusterConfig) > 0 {
		force = state.ClusterConfig[0].ForceClusterDeleteOnDestroy.ValueBool()
	}

	res.Diagnostics.Append(cloudClusterDelete(ctx, polarisClient, id, force)...)
}

func (r *azureCloudClusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "azureCloudClusterResource.ImportState")

	cloudClusterImportState(ctx, req, res)
}

// buildCreateInput assembles the SDK CreateAzureClusterInput from the plan (for
// non-write-only fields) and the config (for the write-only admin fields).
func (r *azureCloudClusterResource) buildCreateInput(ctx context.Context, plan, config azureCloudClusterModel) (gqlcloudcluster.CreateAzureClusterInput, diag.Diagnostics) {
	var diags diag.Diagnostics

	cloudAccountID, err := uuid.Parse(plan.CloudAccountID.ValueString())
	if err != nil {
		diags.AddError("Invalid cloud account ID", err.Error())
		return gqlcloudcluster.CreateAzureClusterInput{}, diags
	}

	cc := plan.ClusterConfig[0]
	vm := plan.VMConfig[0]

	region := azureRegion.RegionFromName(vm.Region.ValueString())
	if region == azureRegion.RegionUnknown {
		diags.AddAttributeError(path.Root(keyVMConfig), "Invalid region", fmt.Sprintf("Unknown Azure region %q.", vm.Region.ValueString()))
		return gqlcloudcluster.CreateAzureClusterInput{}, diags
	}

	var subnetAzModels []cloudClusterSubnetAzConfigModel
	if !vm.SubnetAzConfig.IsNull() && !vm.SubnetAzConfig.IsUnknown() {
		diags.Append(vm.SubnetAzConfig.ElementsAs(ctx, &subnetAzModels, false)...)
	}
	var subnetAzConfigs []gqlcloudcluster.SubnetAzConfig
	for _, az := range subnetAzModels {
		subnetAzConfigs = append(subnetAzConfigs, gqlcloudcluster.SubnetAzConfig{
			AvailabilityZone: az.AvailabilityZone.ValueString(),
			Subnet:           az.Subnet.ValueString(),
		})
	}

	dnsNameServers := decodeStringSetOrNil(ctx, cc.DNSNameServers, &diags)
	dnsSearchDomains := decodeStringSetOrNil(ctx, cc.DNSSearchDomains, &diags)
	ntpServers := decodeStringSetOrNil(ctx, cc.NTPServers, &diags)
	if diags.HasError() {
		return gqlcloudcluster.CreateAzureClusterInput{}, diags
	}
	if dnsNameServers == nil {
		dnsNameServers = []string{}
	}
	if dnsSearchDomains == nil {
		dnsSearchDomains = []string{}
	}
	if ntpServers == nil {
		ntpServers = []string{}
	}

	// admin_email and admin_password are write-only, so read them from the config.
	configCC := config.ClusterConfig[0]

	input := gqlcloudcluster.CreateAzureClusterInput{
		CloudAccountID: cloudAccountID,
		ClusterConfig: gqlcloudcluster.AzureClusterConfig{
			ClusterName:      cc.ClusterName.ValueString(),
			UserEmail:        configCC.AdminEmail.ValueString(),
			AdminPassword:    secret.String(configCC.AdminPassword.ValueString()),
			DNSNameServers:   dnsNameServers,
			DNSSearchDomains: dnsSearchDomains,
			NTPServers:       ntpServers,
			NumNodes:         int(cc.NumNodes.ValueInt64()),
			AzureESConfig: gqlcloudcluster.AzureEsConfigInput{
				ResourceGroup:         vm.ResourceGroupName.ValueString(),
				StorageAccount:        vm.StorageAccountName.ValueString(),
				ContainerName:         vm.ContainerName.ValueString(),
				ShouldCreateContainer: false,
				EnableImmutability:    vm.EnableImmutability.ValueBool(),
				ManagedIdentity: gqlcloudcluster.AzureManagedIdentityName{
					Name: vm.UserAssignedManagedIdentity.ValueString(),
				},
			},
		},
		IsAzResilient:        plan.AZResilient.ValueBool(),
		IsESType:             true,
		KeepClusterOnFailure: cc.KeepClusterOnFailure.ValueBool(),
		Validations:          []gqlcloudcluster.ClusterCreateValidations{gqlcloudcluster.AllChecks},
		VMConfig: gqlcloudcluster.AzureVMConfig{
			CDMVersion:                   vm.CDMVersion.ValueString(),
			InstanceType:                 gqlcloudcluster.AzureCCESSupportedInstanceType(vm.InstanceType.ValueString()),
			Location:                     region,
			ResourceGroup:                vm.ResourceGroupName.ValueString(),
			NetworkResourceGroup:         vm.NetworkResourceGroup.ValueString(),
			VnetResourceGroup:            vm.VnetResourceGroup.ValueString(),
			Subnet:                       vm.Subnet.ValueString(),
			SubnetAzConfigs:              subnetAzConfigs,
			Vnet:                         vm.Vnet.ValueString(),
			NetworkSecurityGroup:         vm.NetworkSecurityGroup.ValueString(),
			NetworkSecurityResourceGroup: vm.NetworkSecurityResourceGroup.ValueString(),
			VMType:                       gqlcloudcluster.VmConfigType(vm.VMType.ValueString()),
			AvailabilityZone:             vm.AvailabilityZone.ValueString(),
		},
	}

	return input, diags
}
