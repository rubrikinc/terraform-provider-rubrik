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

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/exocompute"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	gqlgcp "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/gcp"
)

const resourceGCPExocomputeDescription = `
The ´rubrik_gcp_exocompute´ resource creates an RSC Exocompute configuration
for GCP workloads. This resource should only be used with customer managed
networking. Customer managed networking is used when the ´EXOCOMPUTE´ feature
of the GCP project was onboarded without the ´AUTOMATED_NETWORKING_SETUP´
permission group. If the GCP project was onboarded with the
´AUTOMATED_NETWORKING_SETUP´ permission group, RSC will automatically create
and manage the networking resources for Exocompute.

~> **Note:** Only a single ´rubrik_gcp_exocompute´ resource should be created
per cloud account. RSC maintains a single Exocompute configuration per cloud
account, so multiple resources referring to the same ´cloud_account_id´
overwrite each other's regional configurations. To run Exocompute in more than
one region, add a ´regional_config´ block per region.
`

// gcpDefaultSecondaryRangeName is the name RSC gives the GKE pods secondary
// IP range when the regional configuration doesn't specify one.
const gcpDefaultSecondaryRangeName = "pods-cidr-range"

var (
	_ resource.Resource                = &gcpExocomputeResource{}
	_ resource.ResourceWithIdentity    = &gcpExocomputeResource{}
	_ resource.ResourceWithImportState = &gcpExocomputeResource{}
	_ resource.ResourceWithMoveState   = &gcpExocomputeResource{}
)

type gcpExocomputeResource struct {
	client *client
	prefix string
}

type gcpExocomputeModel struct {
	ID                 types.String `tfsdk:"id"`
	CloudAccountID     types.String `tfsdk:"cloud_account_id"`
	RegionalConfig     types.Set    `tfsdk:"regional_config"`
	TriggerHealthCheck types.Bool   `tfsdk:"trigger_health_check"`
}

type gcpExocomputeRegionalConfigModel struct {
	HostProjectID      types.String `tfsdk:"host_project_id"`
	Region             types.String `tfsdk:"region"`
	SecondaryRangeName types.String `tfsdk:"secondary_range_name"`
	SubnetName         types.String `tfsdk:"subnet_name"`
	VPCName            types.String `tfsdk:"vpc_name"`
}

// gcpExocomputeRegionalConfigType is the element type of the regional_config
// block.
var gcpExocomputeRegionalConfigType = types.ObjectType{AttrTypes: map[string]attr.Type{
	keyHostProjectID:      types.StringType,
	keyRegion:             types.StringType,
	keySecondaryRangeName: types.StringType,
	keySubnetName:         types.StringType,
	keyVPCName:            types.StringType,
}}

type gcpExocomputeIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

func newGcpExocomputeResource() resource.Resource {
	return &gcpExocomputeResource{prefix: keyRubrik}
}

func newPolarisGcpExocomputeResource() resource.Resource {
	return &gcpExocomputeResource{prefix: keyPolaris}
}

func (r *gcpExocomputeResource) Metadata(ctx context.Context, _ resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "gcpExocomputeResource.Metadata")

	res.TypeName = r.prefix + "_" + keyGCPExocompute
}

func (r *gcpExocomputeResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "gcpExocomputeResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceGCPExocomputeDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "RSC Cloud Account ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyCloudAccountID: schema.StringAttribute{
				Required: true,
				Description: "RSC cloud account ID. This is the ID of the `rubrik_gcp_project` resource for " +
					"which the Exocompute service runs. Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyTriggerHealthCheck: schema.BoolAttribute{
				Optional:    true,
				Description: "Trigger a health check for the Exocompute configuration. Defaults to `false`.",
			},
		},
		Blocks: map[string]schema.Block{
			keyRegionalConfig: schema.SetNestedBlock{
				Description: "Regional configuration for the Exocompute service. At least one block must be " +
					"specified.",
				Validators: []validator.Set{
					setvalidator.IsRequired(),
					setvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyHostProjectID: schema.StringAttribute{
							Optional: true,
							Description: "GCP project ID of the project hosting the VPC network. Only needed " +
								"when the VPC network is a Shared VPC. When omitted, the VPC network is assumed " +
								"to belong to the same GCP project as the Exocompute cluster. Note, this is the " +
								"GCP project ID, e.g. `my-host-project`, and not the RSC cloud account ID.",
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
						keyRegion: schema.StringAttribute{
							Optional: true,
							Description: "GCP region to run the exocompute service in. Should be specified in " +
								"the standard GCP style, e.g. `us-east1`.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlgcp.AllRegionNames()...),
							},
						},
						keySecondaryRangeName: schema.StringAttribute{
							Optional: true,
							Computed: true,
							Default:  stringdefault.StaticString(gcpDefaultSecondaryRangeName),
							Description: "Name of the GKE pods secondary IP range on the subnet. Defaults to " +
								"`pods-cidr-range`.",
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
						keySubnetName: schema.StringAttribute{
							Optional:    true,
							Description: "Name of the GCP subnet to run the exocompute service in.",
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
						keyVPCName: schema.StringAttribute{
							Optional:    true,
							Description: "Name of the GCP VPC to run the exocompute service in.",
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
					},
				},
			},
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use `rubrik_gcp_exocompute` instead."
	}
}

func (r *gcpExocomputeResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "gcpExocomputeResource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "RSC Cloud Account ID (UUID).",
			},
		},
	}
}

func (r *gcpExocomputeResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "gcpExocomputeResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *gcpExocomputeResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "gcpExocomputeResource.Create")

	var plan gcpExocomputeModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	cloudAccountID, err := uuid.Parse(plan.CloudAccountID.ValueString())
	if err != nil {
		res.Diagnostics.AddAttributeError(path.Root(keyCloudAccountID), "Invalid cloud account ID", err.Error())
		return
	}

	var configs []gcpExocomputeRegionalConfigModel
	res.Diagnostics.Append(plan.RegionalConfig.ElementsAs(ctx, &configs, false)...)
	if res.Diagnostics.HasError() {
		return
	}
	regionalConfigs := make([]exocompute.RegionalConfig, 0, len(configs))
	for _, config := range configs {
		regionalConfigs = append(regionalConfigs, exocompute.RegionalConfig{
			Region:             gqlgcp.RegionFromName(config.Region.ValueString()),
			SubnetName:         config.SubnetName.ValueString(),
			VPCNetworkName:     config.VPCName.ValueString(),
			HostProjectID:      config.HostProjectID.ValueString(),
			SecondaryRangeName: config.SecondaryRangeName.ValueString(),
		})
	}

	err = exocompute.Wrap(polarisClient).UpdateGCPConfiguration(ctx, cloudAccountID, regionalConfigs,
		plan.TriggerHealthCheck.ValueBool())
	if err != nil {
		res.Diagnostics.AddError("Failed to create GCP Exocompute configuration", err.Error())
		return
	}

	// The ID is the only computed value and all configured values must equal
	// the plan, so there is nothing to read back.
	plan.ID = types.StringValue(cloudAccountID.String())
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.Identity.Set(ctx, gcpExocomputeIdentityModel{ID: plan.ID})...)
}

func (r *gcpExocomputeResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "gcpExocomputeResource.Read")

	var state gcpExocomputeModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	cloudAccountID, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid Exocompute configuration ID", err.Error())
		return
	}

	exoConfigs, err := exocompute.Wrap(polarisClient).GCPConfigurationsByCloudAccountID(ctx, cloudAccountID, false)
	if errors.Is(err, graphql.ErrNotFound) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read GCP Exocompute configuration", err.Error())
		return
	}

	// RSC returns an unset string as an empty string, which is stored as null
	// to match a configuration leaving the attribute out. The validators reject
	// a configured empty string. The secondary range name has a default, an
	// empty value is stored as the default.
	configs := make([]gcpExocomputeRegionalConfigModel, 0, len(exoConfigs))
	for _, exoConfig := range exoConfigs {
		config := gcpExocomputeRegionalConfigModel{
			HostProjectID:      types.StringNull(),
			Region:             types.StringNull(),
			SecondaryRangeName: types.StringValue(gcpDefaultSecondaryRangeName),
			SubnetName:         types.StringNull(),
			VPCName:            types.StringNull(),
		}
		if v := exoConfig.Config.HostProjectID; v != "" {
			config.HostProjectID = types.StringValue(v)
		}
		if v := exoConfig.Config.Region.Name(); v != "" {
			config.Region = types.StringValue(v)
		}
		if v := exoConfig.Config.SecondaryRangeName; v != "" {
			config.SecondaryRangeName = types.StringValue(v)
		}
		if v := exoConfig.Config.SubnetName; v != "" {
			config.SubnetName = types.StringValue(v)
		}
		if v := exoConfig.Config.VPCNetworkName; v != "" {
			config.VPCName = types.StringValue(v)
		}
		configs = append(configs, config)
	}
	set, diags := types.SetValueFrom(ctx, gcpExocomputeRegionalConfigType, configs)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	state.CloudAccountID = types.StringValue(cloudAccountID.String())
	state.RegionalConfig = set

	// RSC doesn't return trigger_health_check, it is kept as it is.
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.Identity.Set(ctx, gcpExocomputeIdentityModel{ID: state.ID})...)
}

func (r *gcpExocomputeResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "gcpExocomputeResource.Update")

	var plan gcpExocomputeModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	var state gcpExocomputeModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	cloudAccountID, err := uuid.Parse(plan.CloudAccountID.ValueString())
	if err != nil {
		res.Diagnostics.AddAttributeError(path.Root(keyCloudAccountID), "Invalid cloud account ID", err.Error())
		return
	}

	var configs []gcpExocomputeRegionalConfigModel
	res.Diagnostics.Append(plan.RegionalConfig.ElementsAs(ctx, &configs, false)...)
	if res.Diagnostics.HasError() {
		return
	}
	regionalConfigs := make([]exocompute.RegionalConfig, 0, len(configs))
	for _, config := range configs {
		regionalConfigs = append(regionalConfigs, exocompute.RegionalConfig{
			Region:             gqlgcp.RegionFromName(config.Region.ValueString()),
			SubnetName:         config.SubnetName.ValueString(),
			VPCNetworkName:     config.VPCName.ValueString(),
			HostProjectID:      config.HostProjectID.ValueString(),
			SecondaryRangeName: config.SecondaryRangeName.ValueString(),
		})
	}

	// RSC replaces the whole configuration, the full set is always sent.
	err = exocompute.Wrap(polarisClient).UpdateGCPConfiguration(ctx, cloudAccountID, regionalConfigs,
		plan.TriggerHealthCheck.ValueBool())
	if err != nil {
		res.Diagnostics.AddError("Failed to update GCP Exocompute configuration", err.Error())
		return
	}

	// The ID is the only computed value and all configured values must equal
	// the plan, so there is nothing to read back.
	plan.ID = state.ID
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.Identity.Set(ctx, gcpExocomputeIdentityModel{ID: plan.ID})...)
}

func (r *gcpExocomputeResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "gcpExocomputeResource.Delete")

	var state gcpExocomputeModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	cloudAccountID, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid Exocompute configuration ID", err.Error())
		return
	}

	err = exocompute.Wrap(polarisClient).RemoveGCPConfiguration(ctx, cloudAccountID)
	if err != nil && !errors.Is(err, graphql.ErrNotFound) {
		res.Diagnostics.AddError("Failed to delete GCP Exocompute configuration", err.Error())
		return
	}
}

func (r *gcpExocomputeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "gcpExocomputeResource.ImportState")

	resource.ImportStatePassthroughWithIdentity(ctx, path.Root(keyID), path.Root(keyID), req, res)
}
