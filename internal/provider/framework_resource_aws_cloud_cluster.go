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
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
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
	gqlaws "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/aws"
)

const resourceAwsCloudClusterDescription = `
The ´rubrik_aws_cloud_cluster´ resource creates an AWS cloud cluster using RSC.

This resource creates a Rubrik Cloud Data Management (CDM) cluster with elastic storage
in AWS using the specified configuration. The cluster will be deployed with the specified
number of nodes, instance types, and network configuration.

~> **Note:** This resource creates actual AWS infrastructure. Destroying the
   resource removes the cluster from RSC, the cloud VMs are left running and must be
   cleaned up manually.

~> **Note:** The AWS account must be onboarded to RSC with the Server and Apps
   feature enabled before creating a cloud cluster.

~> **Note:** This resource requires **Terraform v1.11.0 or later** due to the use of write-only attributes for
   ´admin_email´ and ´admin_password´.

~> **Note:** When destroying this resource, the cluster is removed from RSC. If the cluster has
   blocking conditions (active SLAs, global SLAs, or RCV locations), the removal fails and the
   conditions must be resolved first. ´force_cluster_delete_on_destroy´ only applies to
   disconnected clusters with blocking conditions, and must be applied before the resource is
   destroyed.
`

// awsCloudClusterInstanceTypes are the instance type names RSC accepts for AWS
// cloud clusters.
var awsCloudClusterInstanceTypes = []gqlcloudcluster.AwsCCInstanceType{
	gqlcloudcluster.AwsInstanceTypeM5_4XLarge,
	gqlcloudcluster.AwsInstanceTypeM6I_2XLarge,
	gqlcloudcluster.AwsInstanceTypeM6I_4XLarge,
	gqlcloudcluster.AwsInstanceTypeM6I_8XLarge,
	gqlcloudcluster.AwsInstanceTypeR6I_4XLarge,
	gqlcloudcluster.AwsInstanceTypeM6A_2XLarge,
	gqlcloudcluster.AwsInstanceTypeM6A_4XLarge,
	gqlcloudcluster.AwsInstanceTypeM6A_8XLarge,
	gqlcloudcluster.AwsInstanceTypeR6A_4XLarge,
}

var (
	_ resource.Resource                   = &awsCloudClusterResource{}
	_ resource.ResourceWithConfigure      = &awsCloudClusterResource{}
	_ resource.ResourceWithIdentity       = &awsCloudClusterResource{}
	_ resource.ResourceWithImportState    = &awsCloudClusterResource{}
	_ resource.ResourceWithMoveState      = &awsCloudClusterResource{}
	_ resource.ResourceWithValidateConfig = &awsCloudClusterResource{}
)

type awsCloudClusterResource struct {
	client *client
	prefix string
}

type awsCloudClusterModel struct {
	ID                 types.String            `tfsdk:"id"`
	CloudAccountID     types.String            `tfsdk:"cloud_account_id"`
	Region             types.String            `tfsdk:"region"`
	UsePlacementGroups types.Bool              `tfsdk:"use_placement_groups"`
	AZResilient        types.Bool              `tfsdk:"az_resilient"`
	ClusterConfig      []awsClusterConfigModel `tfsdk:"cluster_config"`
	VMConfig           []awsVMConfigModel      `tfsdk:"vm_config"`
	Timeouts           timeouts.Value          `tfsdk:"timeouts"`
}

type awsClusterConfigModel struct {
	cloudClusterCommonConfigModel
	BucketName            types.String `tfsdk:"bucket_name"`
	EnableImmutability    types.Bool   `tfsdk:"enable_immutability"`
	DynamicScalingEnabled types.Bool   `tfsdk:"dynamic_scaling_enabled"`
}

type awsVMConfigModel struct {
	CDMVersion          types.String `tfsdk:"cdm_version"`
	CDMProduct          types.String `tfsdk:"cdm_product"`
	InstanceType        types.String `tfsdk:"instance_type"`
	InstanceProfileName types.String `tfsdk:"instance_profile_name"`
	VPCID               types.String `tfsdk:"vpc_id"`
	SubnetID            types.String `tfsdk:"subnet_id"`
	SecurityGroupIDs    types.Set    `tfsdk:"security_group_ids"`
	VMType              types.String `tfsdk:"vm_type"`
	// SubnetAzConfig is a types.List so it can hold an unknown value, see the
	// GCP cloud cluster resource.
	SubnetAzConfig types.List `tfsdk:"subnet_az_config"`
}

func newAwsCloudClusterResource() resource.Resource {
	return &awsCloudClusterResource{prefix: keyRubrik}
}

func newPolarisAwsCloudClusterResource() resource.Resource {
	return &awsCloudClusterResource{prefix: keyPolaris}
}

func (r *awsCloudClusterResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "awsCloudClusterResource.Metadata")

	res.TypeName = r.prefix + "_" + keyAWSCloudCluster
}

func (r *awsCloudClusterResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "awsCloudClusterResource.Schema")

	instanceTypes := make([]string, len(awsCloudClusterInstanceTypes))
	for i, t := range awsCloudClusterInstanceTypes {
		instanceTypes[i] = string(t)
	}
	vmTypes := make([]string, len(cloudClusterVMTypes))
	for i, t := range cloudClusterVMTypes {
		vmTypes[i] = string(t)
	}

	clusterConfigAttributes := cloudClusterCommonConfigAttributes()
	clusterConfigAttributes[keyBucketName] = schema.StringAttribute{
		Required:      true,
		Description:   "Name of the S3 bucket to use for the cluster. Changing this forces a new resource to be created.",
		PlanModifiers: []planmodifier.String{cloudClusterRequiresReplaceString()},
		Validators:    []validator.String{isNotWhiteSpace()},
	}
	clusterConfigAttributes[keyEnableImmutability] = schema.BoolAttribute{
		Required:      true,
		Description:   "Whether to enable immutability and object lock for the S3 bucket. Changing this forces a new resource to be created.",
		PlanModifiers: []planmodifier.Bool{cloudClusterRequiresReplaceBool()},
	}
	clusterConfigAttributes[keyDynamicScalingEnabled] = schema.BoolAttribute{
		Optional:      true,
		Computed:      true,
		Default:       booldefault.StaticBool(false),
		Description:   "Whether to enable dynamic scaling for the cluster. Requires CDM Version 9.5+. Changing this forces a new resource to be created.",
		PlanModifiers: []planmodifier.Bool{cloudClusterRequiresReplaceBool()},
	}

	res.Schema = schema.Schema{
		Description: description(resourceAwsCloudClusterDescription),
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
			keyRegion: schema.StringAttribute{
				Required:      true,
				Description:   "AWS region to deploy the cluster in. Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{cloudClusterRequiresReplaceString()},
				Validators:    []validator.String{stringvalidator.OneOf(gqlaws.AllRegionNames()...)},
			},
			keyUsePlacementGroups: schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Default:       booldefault.StaticBool(false),
				Description:   "Whether to use placement groups for the cluster. Cannot be used with `az_resilient`. Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.Bool{cloudClusterRequiresReplaceBool()},
			},
			// No default: SDKv2 state holds null when the attribute is not
			// configured, a default of false would plan a change from null.
			keyAzResilient: schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether to deploy the cluster across multiple availability zones for AZ resiliency. When enabled, `subnet_az_config` blocks must be specified in `vm_config` and `use_placement_groups` must be false. Changing this forces a new resource to be created.",
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
				NestedObject: schema.NestedBlockObject{Attributes: clusterConfigAttributes},
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
						keyCDMVersion: schema.StringAttribute{
							Required: true,
							Description: "CDM version to use. This is a create-time input, it is not updated when the cluster is " +
								"upgraded outside of Terraform. Changing this forces a new resource to be created.",
							PlanModifiers: []planmodifier.String{cloudClusterRequiresReplaceString()},
							Validators:    []validator.String{isNotWhiteSpace()},
						},
						keyCDMProduct: schema.StringAttribute{
							Computed:      true,
							Description:   "CDM Product Code. This is a read-only field and computed based on the CDM version.",
							PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
						},
						keyInstanceType: schema.StringAttribute{
							Required: true,
							Description: fmt.Sprintf("AWS instance type for the cluster nodes. Changing this forces a new resource to be created. %s.",
								possibleValues(awsCloudClusterInstanceTypes)),
							PlanModifiers: []planmodifier.String{cloudClusterRequiresReplaceString()},
							Validators:    []validator.String{stringvalidator.OneOf(instanceTypes...)},
						},
						keyInstanceProfileName: schema.StringAttribute{
							Required:      true,
							Description:   "AWS instance profile name for the cluster nodes. Changing this forces a new resource to be created.",
							PlanModifiers: []planmodifier.String{cloudClusterRequiresReplaceString()},
							Validators:    []validator.String{isNotWhiteSpace()},
						},
						keyVPCID: schema.StringAttribute{
							Required:      true,
							Description:   "AWS VPC ID where the cluster will be deployed. Changing this forces a new resource to be created.",
							PlanModifiers: []planmodifier.String{cloudClusterRequiresReplaceString()},
							Validators:    []validator.String{isNotWhiteSpace()},
						},
						keySubnetID: schema.StringAttribute{
							Optional:      true,
							Description:   "AWS subnet ID where the cluster nodes will be deployed. Required when `az_resilient` is false. Changing this forces a new resource to be created.",
							PlanModifiers: []planmodifier.String{cloudClusterRequiresReplaceString()},
							Validators:    []validator.String{isNotWhiteSpace()},
						},
						keySecurityGroupIDs: schema.SetAttribute{
							ElementType:   types.StringType,
							Required:      true,
							Description:   "AWS security group IDs for the cluster nodes. Changing this forces a new resource to be created.",
							PlanModifiers: []planmodifier.Set{cloudClusterRequiresReplaceSet()},
							Validators:    []validator.Set{setvalidator.SizeAtLeast(1)},
						},
						keyVMType: schema.StringAttribute{
							Optional: true,
							Computed: true,
							Default:  stringdefault.StaticString(string(gqlcloudcluster.CCVmConfigDense)),
							Description: fmt.Sprintf("VM type for the cluster. Changing this forces a new resource to be created. %s. "+
								"`DENSE` is recommended for CCES.", possibleValues(cloudClusterVMTypes)),
							PlanModifiers: []planmodifier.String{cloudClusterRequiresReplaceString()},
							Validators:    []validator.String{stringvalidator.OneOf(vmTypes...)},
						},
					},
					Blocks: map[string]schema.Block{
						keySubnetAzConfigs: cloudClusterSubnetAzConfigBlock("Subnet ID for this availability zone."),
					},
				},
			},
			keyTimeouts: timeouts.Block(ctx, timeouts.Opts{Create: true, Read: true}),
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use `rubrik_aws_cloud_cluster` instead."
	}
}

func (r *awsCloudClusterResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "awsCloudClusterResource.IdentitySchema")

	res.IdentitySchema = cloudClusterIdentitySchema()
}

func (r *awsCloudClusterResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "awsCloudClusterResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *awsCloudClusterResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, res *resource.ValidateConfigResponse) {
	tflog.Trace(ctx, "awsCloudClusterResource.ValidateConfig")

	var config awsCloudClusterModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(validateAwsCloudClusterConfig(config)...)
}

// validateAwsCloudClusterConfig holds the plan-time, client-free validation
// rules: the az_resilient / subnet_id / subnet_az_config either-or, and that
// placement groups can't be used with AZ resiliency. A null az_resilient is
// false.
//
// Checks are skipped for any value that is unknown at plan time, because
// reading it would yield a zero value and reject an otherwise-valid config.
func validateAwsCloudClusterConfig(config awsCloudClusterModel) diag.Diagnostics {
	var diags diag.Diagnostics

	if len(config.VMConfig) == 0 || config.AZResilient.IsUnknown() {
		return diags
	}
	vm := config.VMConfig[0]
	azResilient := config.AZResilient.ValueBool()

	subnetAzKnown := !vm.SubnetAzConfig.IsUnknown()
	hasSubnetAzConfigs := subnetAzKnown && !vm.SubnetAzConfig.IsNull() && len(vm.SubnetAzConfig.Elements()) > 0
	subnetKnown := !vm.SubnetID.IsUnknown()
	hasSubnet := subnetKnown && vm.SubnetID.ValueString() != ""

	if azResilient {
		if subnetAzKnown && !hasSubnetAzConfigs {
			diags.AddAttributeError(path.Root(keyVMConfig), "subnet_az_config required",
				"`subnet_az_config` is required in `vm_config` when `az_resilient` is true.")
		}
		if hasSubnet {
			diags.AddAttributeError(path.Root(keyVMConfig), "subnet_id not allowed",
				"`subnet_id` cannot be specified in `vm_config` when `az_resilient` is true, use `subnet_az_config` instead.")
		}
		if !config.UsePlacementGroups.IsUnknown() && config.UsePlacementGroups.ValueBool() {
			diags.AddAttributeError(path.Root(keyUsePlacementGroups), "use_placement_groups not allowed",
				"`use_placement_groups` must be false when `az_resilient` is true.")
		}
	} else {
		if hasSubnetAzConfigs {
			diags.AddAttributeError(path.Root(keyVMConfig), "subnet_az_config not allowed",
				"`subnet_az_config` cannot be specified in `vm_config` when `az_resilient` is false.")
		}
		if subnetKnown && !hasSubnet {
			diags.AddAttributeError(path.Root(keyVMConfig), "subnet_id required",
				"`subnet_id` is required in `vm_config` when `az_resilient` is false.")
		}
	}

	return diags
}

func (r *awsCloudClusterResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "awsCloudClusterResource.Create")

	var plan awsCloudClusterModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	// admin_email and admin_password are write-only, so they are null in the
	// plan; read them from the config instead.
	var config awsCloudClusterModel
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

	// useLatestCdmVersion is false so the configured cdm_version is honoured
	// exactly.
	awsCluster, err := cloudcluster.Wrap(polarisClient).CreateCloudCluster(ctx, input, false)
	if err != nil {
		res.Diagnostics.AddError("Failed to create AWS cloud cluster", err.Error())
		return
	}

	// The cluster exists in RSC from here on. Saving its ID before reading it
	// back makes Terraform keep track of it if the read fails.
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyID), awsCluster.ID.String())...)
	res.Diagnostics.Append(res.Identity.SetAttribute(ctx, path.Root(keyID), awsCluster.ID.String())...)
	if res.Diagnostics.HasError() {
		return
	}

	plan.ID = types.StringValue(awsCluster.ID.String())
	plan.VMConfig[0].CDMProduct = types.StringValue(awsCluster.CdmProduct)
	if plan.AZResilient.IsUnknown() {
		plan.AZResilient = types.BoolValue(input.IsAzResilient)
	}

	// Read back the cluster and apply the timezone and location, the create API
	// can't set them. Failures are warnings, the cluster exists.
	res.Diagnostics.Append(cloudClusterFinishCreate(ctx, gqlcluster.Wrap(polarisClient.GQL), awsCluster.ID,
		&plan.ClusterConfig[0].cloudClusterCommonConfigModel)...)

	// Write-only attributes must be null in state.
	plan.ClusterConfig[0].AdminEmail = types.StringNull()
	plan.ClusterConfig[0].AdminPassword = types.StringNull()

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
}

func (r *awsCloudClusterResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "awsCloudClusterResource.Read")

	var state awsCloudClusterModel
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
		state.ClusterConfig = []awsClusterConfigModel{{
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
			BucketName:            types.StringNull(),
			EnableImmutability:    types.BoolNull(),
			DynamicScalingEnabled: types.BoolValue(false),
		}}
		state.VMConfig = []awsVMConfigModel{{
			CDMVersion:          types.StringNull(),
			CDMProduct:          types.StringNull(),
			InstanceType:        types.StringNull(),
			InstanceProfileName: types.StringNull(),
			VPCID:               types.StringNull(),
			SubnetID:            types.StringNull(),
			SecurityGroupIDs:    types.SetNull(types.StringType),
			VMType:              types.StringNull(),
			SubnetAzConfig:      types.ListNull(cloudClusterSubnetAzObjectType),
		}}
		if state.UsePlacementGroups.IsNull() {
			state.UsePlacementGroups = types.BoolValue(false)
		}
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

	rscCluster, err := cluster.Wrap(polarisClient).Cluster(ctx, id)
	if errors.Is(err, graphql.ErrNotFound) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read AWS cloud cluster", err.Error())
		return
	}

	// vm_config.cdm_version is a create-time input and is deliberately not
	// refreshed from the installed version, that would plan a replacement of
	// the cluster after an upgrade outside of Terraform. It's only set when
	// missing, i.e. after an import.
	vm := &state.VMConfig[0]
	if vm.CDMVersion.IsNull() {
		vm.CDMVersion = types.StringValue(rscCluster.Version)
	}
	// SDKv2 stored an empty string for an unset optional string.
	if vm.SubnetID.ValueString() == "" {
		vm.SubnetID = types.StringNull()
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

func (r *awsCloudClusterResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "awsCloudClusterResource.Update")

	var plan, state awsCloudClusterModel
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

func (r *awsCloudClusterResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "awsCloudClusterResource.Delete")

	var state awsCloudClusterModel
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

func (r *awsCloudClusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "awsCloudClusterResource.ImportState")

	cloudClusterImportState(ctx, req, res)
}

// buildCreateInput assembles the SDK CreateAwsClusterInput from the plan (for
// non-write-only fields) and the config (for the write-only admin fields).
func (r *awsCloudClusterResource) buildCreateInput(ctx context.Context, plan, config awsCloudClusterModel) (gqlcloudcluster.CreateAwsClusterInput, diag.Diagnostics) {
	var diags diag.Diagnostics

	cloudAccountID, err := uuid.Parse(plan.CloudAccountID.ValueString())
	if err != nil {
		diags.AddError("Invalid cloud account ID", err.Error())
		return gqlcloudcluster.CreateAwsClusterInput{}, diags
	}

	cc := plan.ClusterConfig[0]
	vm := plan.VMConfig[0]

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

	securityGroups := decodeStringSetOrNil(ctx, vm.SecurityGroupIDs, &diags)
	dnsNameServers := decodeStringSetOrNil(ctx, cc.DNSNameServers, &diags)
	dnsSearchDomains := decodeStringSetOrNil(ctx, cc.DNSSearchDomains, &diags)
	ntpServers := decodeStringSetOrNil(ctx, cc.NTPServers, &diags)
	if diags.HasError() {
		return gqlcloudcluster.CreateAwsClusterInput{}, diags
	}
	if securityGroups == nil {
		securityGroups = []string{}
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

	input := gqlcloudcluster.CreateAwsClusterInput{
		CloudAccountID: cloudAccountID,
		ClusterConfig: gqlcloudcluster.AwsClusterConfig{
			ClusterName:      cc.ClusterName.ValueString(),
			UserEmail:        configCC.AdminEmail.ValueString(),
			AdminPassword:    secret.String(configCC.AdminPassword.ValueString()),
			DNSNameServers:   dnsNameServers,
			DNSSearchDomains: dnsSearchDomains,
			NTPServers:       ntpServers,
			NumNodes:         int(cc.NumNodes.ValueInt64()),
			AwsEsConfig: gqlcloudcluster.AwsEsConfigInput{
				BucketName:         cc.BucketName.ValueString(),
				ShouldCreateBucket: false,
				EnableImmutability: cc.EnableImmutability.ValueBool(),
				EnableObjectLock:   cc.EnableImmutability.ValueBool(),
			},
			DynamicScalingEnabled: cc.DynamicScalingEnabled.ValueBool(),
		},
		IsAzResilient:        plan.AZResilient.ValueBool(),
		IsEsType:             true,
		KeepClusterOnFailure: cc.KeepClusterOnFailure.ValueBool(),
		Region:               plan.Region.ValueString(),
		UsePlacementGroups:   plan.UsePlacementGroups.ValueBool(),
		Validations:          []gqlcloudcluster.ClusterCreateValidations{gqlcloudcluster.AllChecks},
		VMConfig: gqlcloudcluster.AwsVmConfig{
			CDMVersion:          vm.CDMVersion.ValueString(),
			InstanceProfileName: vm.InstanceProfileName.ValueString(),
			InstanceType:        gqlcloudcluster.AwsCCInstanceType(vm.InstanceType.ValueString()),
			SecurityGroups:      securityGroups,
			Subnet:              vm.SubnetID.ValueString(),
			SubnetAzConfigs:     subnetAzConfigs,
			VMType:              gqlcloudcluster.VmConfigType(vm.VMType.ValueString()),
			VPC:                 vm.VPCID.ValueString(),
		},
	}

	return input, diags
}
