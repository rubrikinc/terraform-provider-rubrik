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
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/gcp"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

const resourceGCPProjectDescription = `
The ´rubrik_gcp_project´ resource adds a GCP project to RSC.

The ´permissions´ field of each feature can be used with the
´rubrik_gcp_permissions´ data source to notify RSC about permission updates
when the Terraform configuration is applied.

{{template "gcpPermissionGroups"}}
`

var (
	_ resource.Resource                   = &gcpProjectResource{}
	_ resource.ResourceWithIdentity       = &gcpProjectResource{}
	_ resource.ResourceWithImportState    = &gcpProjectResource{}
	_ resource.ResourceWithModifyPlan     = &gcpProjectResource{}
	_ resource.ResourceWithMoveState      = &gcpProjectResource{}
	_ resource.ResourceWithUpgradeState   = &gcpProjectResource{}
	_ resource.ResourceWithValidateConfig = &gcpProjectResource{}
)

type gcpProjectResource struct {
	client *client
	prefix string
}

type gcpProjectResourceModel struct {
	ID                       types.String `tfsdk:"id"`
	CloudNativeProtection    types.List   `tfsdk:"cloud_native_protection"`
	Credentials              types.String `tfsdk:"credentials"`
	DeleteSnapshotsOnDestroy types.Bool   `tfsdk:"delete_snapshots_on_destroy"`
	Feature                  types.Set    `tfsdk:"feature"`
	OrganizationName         types.String `tfsdk:"organization_name"`
	PermissionsHash          types.String `tfsdk:"permissions_hash"`
	Project                  types.String `tfsdk:"project"`
	ProjectName              types.String `tfsdk:"project_name"`
	ProjectNumber            types.String `tfsdk:"project_number"`
}

type gcpProjectFeatureModel struct {
	Name             types.String `tfsdk:"name"`
	PermissionGroups types.Set    `tfsdk:"permission_groups"`
	Permissions      types.String `tfsdk:"permissions"`
	Status           types.String `tfsdk:"status"`
}

type gcpProjectCloudNativeProtectionModel struct {
	Status types.String `tfsdk:"status"`
}

// gcpProjectFeatureType is the element type of the feature block.
var gcpProjectFeatureType = types.ObjectType{AttrTypes: map[string]attr.Type{
	keyName:             types.StringType,
	keyPermissionGroups: types.SetType{ElemType: types.StringType},
	keyPermissions:      types.StringType,
	keyStatus:           types.StringType,
}}

// gcpProjectCloudNativeProtectionType is the element type of the
// cloud_native_protection block.
var gcpProjectCloudNativeProtectionType = types.ObjectType{AttrTypes: map[string]attr.Type{
	keyStatus: types.StringType,
}}

type gcpProjectIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

func newGcpProjectResource() resource.Resource {
	return &gcpProjectResource{prefix: keyRubrik}
}

func newPolarisGcpProjectResource() resource.Resource {
	return &gcpProjectResource{prefix: keyPolaris}
}

func (r *gcpProjectResource) Metadata(ctx context.Context, _ resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "gcpProjectResource.Metadata")

	res.TypeName = r.prefix + "_" + keyGCPProject
}

func (r *gcpProjectResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "gcpProjectResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceGCPProjectDescription),
		Version:     3,
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "RSC cloud account ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyCredentials: schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				Description: "Base64 encoded GCP service account private key or path to GCP service account key " +
					"file.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyDeleteSnapshotsOnDestroy: schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Should snapshots be deleted when the resource is destroyed. Default value is `false`.",
			},
			keyOrganizationName: schema.StringAttribute{
				Optional:    true,
				Description: "GCP organization name.",
			},
			keyPermissionsHash: schema.StringAttribute{
				Optional: true,
				Description: "Signals that the permissions has been updated. **Deprecated:** use the " +
					"`permissions` field of `feature` instead.",
				DeprecationMessage: "Use the `permissions` field of `feature` instead.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyProject: schema.StringAttribute{
				Required:    true,
				Description: "GCP project ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyProjectName: schema.StringAttribute{
				Required:    true,
				Description: "GCP project name.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyProjectNumber: schema.StringAttribute{
				Required:    true,
				Description: "GCP project number.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isInt64(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			keyCloudNativeProtection: schema.ListNestedBlock{
				Description: "Enable the Cloud Native Protection feature for the GCP project. **Deprecated:** " +
					"use `feature` instead.",
				DeprecationMessage: "Use `feature` instead.",
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyStatus: schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "Status of the Cloud Native Protection feature.",
							// State upgraded from the SDKv2 resource can lack the
							// block, the status is then left unknown rather than
							// planned as the null of the missing element.
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseNonNullStateForUnknown(),
							},
						},
					},
				},
			},
			keyFeature: schema.SetNestedBlock{
				Description: "RSC feature to enable for the GCP project.",
				NestedObject: schema.NestedBlockObject{
					Validators: []validator.Object{
						featurePermissionGroupsValidator(core.GCPProjectFeatures()),
					},
					Attributes: map[string]schema.Attribute{
						keyName: schema.StringAttribute{
							Required: true,
							Description: fmt.Sprintf("RSC feature name. %s.",
								possibleValues(core.GCPProjectFeatureNames())),
							Validators: []validator.String{
								stringvalidator.OneOf(core.GCPProjectFeatureNames()...),
							},
						},
						keyPermissionGroups: schema.SetAttribute{
							ElementType: types.StringType,
							Required:    true,
							Description: fmt.Sprintf("Permission groups for the RSC feature. %s.",
								possibleValues(core.GCPProjectPermissionGroupNames())),
							Validators: []validator.Set{
								setvalidator.ValueStringsAre(
									stringvalidator.OneOf(core.GCPProjectPermissionGroupNames()...)),
							},
						},
						keyPermissions: schema.StringAttribute{
							Optional: true,
							Description: "Permissions updated signal. When this field changes, the provider will " +
								"notify RSC that the permissions for the feature has been updated. Use this " +
								"field with the `rubrik_gcp_permissions` data source.",
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
						keyStatus: schema.StringAttribute{
							Computed:    true,
							Description: "Status of the feature.",
						},
					},
				},
			},
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use `rubrik_gcp_project` instead."
	}
}

func (r *gcpProjectResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "gcpProjectResource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "RSC cloud account ID (UUID).",
			},
		},
	}
}

// ValidateConfig checks that exactly one of the feature and the
// cloud_native_protection blocks is specified. An absent block may be null
// or empty, so the elements are counted.
func (r *gcpProjectResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, res *resource.ValidateConfigResponse) {
	tflog.Trace(ctx, "gcpProjectResource.ValidateConfig")

	var features types.Set
	res.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(keyFeature), &features)...)
	var cnp types.List
	res.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(keyCloudNativeProtection), &cnp)...)
	if res.Diagnostics.HasError() || features.IsUnknown() || cnp.IsUnknown() {
		return
	}

	hasFeature := len(features.Elements()) > 0
	hasCNP := len(cnp.Elements()) > 0
	if hasFeature == hasCNP {
		res.Diagnostics.AddAttributeError(path.Root(keyFeature), "Invalid block combination",
			"Exactly one of feature and cloud_native_protection must be specified.")
	}
}

// ModifyPlan rejects a plan removing CLOUD_DISCOVERY from a project which still
// has a protection feature. RSC accepts the removal request and only rejects it
// later, in the task chain that disables the feature, so it's caught here.
//
// A state without features uses the deprecated cloud_native_protection block,
// the features in RSC aren't in the state. When the plan switches to feature
// blocks, the features are read from RSC, and the plan warns about the ones
// Update will remove, since the plan itself only shows the feature set going
// from empty to the configured features.
func (r *gcpProjectResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, res *resource.ModifyPlanResponse) {
	tflog.Trace(ctx, "gcpProjectResource.ModifyPlan")

	// Nothing to check on create or destroy.
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var stateFeatures, planFeatures types.Set
	res.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(keyFeature), &stateFeatures)...)
	res.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root(keyFeature), &planFeatures)...)
	if res.Diagnostics.HasError() || planFeatures.IsUnknown() {
		return
	}

	var stateModels, planModels []gcpProjectFeatureModel
	res.Diagnostics.Append(stateFeatures.ElementsAs(ctx, &stateModels, false)...)
	res.Diagnostics.Append(planFeatures.ElementsAs(ctx, &planModels, false)...)
	if res.Diagnostics.HasError() {
		return
	}

	// A plan without features uses the cloud_native_protection block, Update
	// leaves the features in RSC as they are.
	if len(planModels) == 0 {
		return
	}

	var newNames []string
	for _, feature := range planModels {
		if feature.Name.IsUnknown() {
			return
		}
		newNames = append(newNames, feature.Name.ValueString())
	}

	var oldNames []string
	for _, feature := range stateModels {
		oldNames = append(oldNames, feature.Name.ValueString())
	}
	if len(stateModels) == 0 {
		// Without an RSC client, e.g. when the provider configuration isn't
		// known yet, Update still checks the removal against RSC.
		if r.client == nil {
			res.Diagnostics.AddAttributeWarning(path.Root(keyFeature), "Unable to read GCP project features",
				"The provider isn't configured, the features in RSC not in the configuration will be removed "+
					"on apply.")
			return
		}
		polarisClient, err := r.client.polaris()
		if err != nil {
			res.Diagnostics.AddAttributeWarning(path.Root(keyFeature), "Unable to read GCP project features",
				fmt.Sprintf("RSC client error, the features in RSC not in the configuration will be removed "+
					"on apply: %s", err))
			return
		}
		var id types.String
		res.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(keyID), &id)...)
		if res.Diagnostics.HasError() {
			return
		}
		accountID, err := uuid.Parse(id.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Invalid GCP project ID", err.Error())
			return
		}
		account, err := gcp.Wrap(polarisClient).ProjectByID(ctx, accountID)
		if err != nil {
			res.Diagnostics.AddError("Failed to read GCP project", err.Error())
			return
		}

		var removed []string
		for _, feature := range account.Features {
			oldNames = append(oldNames, feature.Name)
			if !slices.Contains(newNames, feature.Name) {
				removed = append(removed, feature.Name)
			}
		}
		if len(removed) > 0 {
			var deleteSnapshots types.Bool
			res.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root(keyDeleteSnapshotsOnDestroy), &deleteSnapshots)...)
			if res.Diagnostics.HasError() {
				return
			}
			snapshots := "Their snapshots are kept."
			if deleteSnapshots.ValueBool() {
				snapshots = "Their snapshots are deleted, since delete_snapshots_on_destroy is true."
			}
			res.Diagnostics.AddAttributeWarning(path.Root(keyFeature), "GCP project features will be removed",
				fmt.Sprintf("The state uses the deprecated cloud_native_protection block and holds no features. "+
					"Switching to feature blocks removes the features enabled in RSC which aren't configured: "+
					"%s. %s", strings.Join(removed, ", "), snapshots))
		}
	}

	if err := gcpCheckCloudDiscoveryRemoval(oldNames, newNames); err != nil {
		res.Diagnostics.AddAttributeError(path.Root(keyFeature), "Invalid feature removal", err.Error())
	}
}

func (r *gcpProjectResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "gcpProjectResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *gcpProjectResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "gcpProjectResource.Create")

	var plan gcpProjectResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	projectNumber, err := strconv.ParseInt(plan.ProjectNumber.ValueString(), 10, 64)
	if err != nil {
		res.Diagnostics.AddAttributeError(path.Root(keyProjectNumber), "Invalid project number", err.Error())
		return
	}
	project := gcp.Project(plan.Project.ValueString(), projectNumber)
	if credentials := plan.Credentials.ValueString(); credentials != "" {
		project = gcp.KeyWithProjectAndNumber(credentials, plan.Project.ValueString(), projectNumber)
	}
	opts := []gcp.OptionFunc{gcp.Name(plan.ProjectName.ValueString())}
	if orgName := plan.OrganizationName.ValueString(); orgName != "" {
		opts = append(opts, gcp.Organization(orgName))
	}

	config, err := project(ctx)
	if err != nil {
		res.Diagnostics.AddError("Failed to look up native project ID", err.Error())
		return
	}
	account, err := gcp.Wrap(polarisClient).ProjectByNativeID(ctx, config.NativeID)
	if err == nil {
		res.Diagnostics.AddError("GCP project already added",
			fmt.Sprintf("project %q already added to RSC", account.NativeID))
		return
	}
	if !errors.Is(err, graphql.ErrNotFound) {
		res.Diagnostics.AddError("Failed to look up GCP project", err.Error())
		return
	}

	planFeatures, diags := gcpProjectFeatures(ctx, plan.Feature)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	features := stripPermissions(planFeatures)
	if len(plan.CloudNativeProtection.Elements()) > 0 {
		features = append(features, core.FeatureCloudNativeProtection)
	}

	id, err := gcp.Wrap(polarisClient).AddProject(ctx, project, features, opts...)
	if err != nil {
		res.Diagnostics.AddError("Failed to add GCP project", err.Error())
		return
	}

	// Save the ID to state and identity before the read-back so Terraform can
	// track the project even if the read fails.
	plan.ID = types.StringValue(id.String())
	res.Diagnostics.Append(setGCPProjectStatus(ctx, &plan, nil)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.Identity.Set(ctx, gcpProjectIdentityModel{ID: plan.ID})...)
	if res.Diagnostics.HasError() {
		return
	}

	account, err = gcp.Wrap(polarisClient).ProjectByID(ctx, id)
	if err != nil {
		res.Diagnostics.AddWarning("Failed to read GCP project after create",
			fmt.Sprintf("The project was added successfully but could not be read back: %s", err.Error()))
		return
	}
	res.Diagnostics.Append(setGCPProjectStatus(ctx, &plan, &account)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
}

func (r *gcpProjectResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "gcpProjectResource.Read")

	var state gcpProjectResourceModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid GCP project ID", err.Error())
		return
	}

	account, err := gcp.Wrap(polarisClient).ProjectByID(ctx, id)
	if errors.Is(err, graphql.ErrNotFound) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read GCP project", err.Error())
		return
	}

	res.Diagnostics.Append(updateGCPProjectResourceModel(ctx, &state, account)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.Identity.Set(ctx, gcpProjectIdentityModel{ID: state.ID})...)
}

func (r *gcpProjectResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "gcpProjectResource.Update")

	var plan gcpProjectResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	var state gcpProjectResourceModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid GCP project ID", err.Error())
		return
	}

	// The features are compared by name, permission groups and permissions,
	// the computed status is ignored. A plan without features uses the
	// cloud_native_protection block, which is only read, never updated, as in
	// the SDKv2 resource.
	stateFeatures, diags := gcpProjectFeatures(ctx, state.Feature)
	res.Diagnostics.Append(diags...)
	planFeatures, diags := gcpProjectFeatures(ctx, plan.Feature)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	// A state without features uses the cloud_native_protection block, so
	// the features are read from RSC, as Delete does. The SDKv2 resource
	// always held the features in RSC there, since the set was computed.
	// ModifyPlan checks the Cloud Discovery removal when it can reach RSC, it's
	// checked again here before anything in RSC is changed.
	if len(planFeatures) > 0 && len(stateFeatures) == 0 {
		account, err := gcp.Wrap(polarisClient).ProjectByID(ctx, id)
		if err != nil {
			res.Diagnostics.AddError("Failed to read GCP project", err.Error())
			return
		}
		var oldNames, newNames []string
		for _, feature := range account.Features {
			stateFeatures = append(stateFeatures, featureWithPermissions{Feature: feature.Feature})
			oldNames = append(oldNames, feature.Name)
		}
		for _, feature := range planFeatures {
			newNames = append(newNames, feature.Name)
		}
		if err := gcpCheckCloudDiscoveryRemoval(oldNames, newNames); err != nil {
			res.Diagnostics.AddAttributeError(path.Root(keyFeature), "Invalid feature removal", err.Error())
			return
		}
	}

	// Removing the credentials from the configuration leaves the service
	// account in RSC as it is, there is no key to set.
	if !plan.Credentials.IsNull() && !plan.Credentials.Equal(state.Credentials) {
		err := gcp.Wrap(polarisClient).SetProjectServiceAccount(ctx, id, gcp.Key(plan.Credentials.ValueString()))
		if err != nil {
			res.Diagnostics.AddError("Failed to set GCP project service account", err.Error())
			return
		}
	}

	if len(planFeatures) > 0 {
		oldFeatures := make(map[string]featureWithPermissions, len(stateFeatures))
		for _, feature := range stateFeatures {
			oldFeatures[feature.Name] = feature
		}

		// A new feature, or one with changed permission groups, is added. A
		// feature in both with a changed permissions signal is signalled, also
		// when it's added again, as in the SDKv2 resource.
		var addFeatures, removeFeatures, permFeatures []core.Feature
		for _, feature := range planFeatures {
			old, ok := oldFeatures[feature.Name]
			delete(oldFeatures, feature.Name)
			if !ok || !feature.Feature.DeepEqual(old.Feature) {
				addFeatures = append(addFeatures, feature.Feature)
			}
			if ok && feature.permissions != old.permissions {
				permFeatures = append(permFeatures, feature.Feature)
			}
		}
		for _, feature := range oldFeatures {
			removeFeatures = append(removeFeatures, feature.Feature)
		}

		if len(addFeatures) > 0 {
			projectNumber, err := strconv.ParseInt(plan.ProjectNumber.ValueString(), 10, 64)
			if err != nil {
				res.Diagnostics.AddAttributeError(path.Root(keyProjectNumber), "Invalid project number", err.Error())
				return
			}
			project := gcp.Project(plan.Project.ValueString(), projectNumber)
			if credentials := plan.Credentials.ValueString(); credentials != "" {
				project = gcp.KeyWithProjectAndNumber(credentials, plan.Project.ValueString(), projectNumber)
			}
			opts := []gcp.OptionFunc{gcp.Name(plan.ProjectName.ValueString())}
			if orgName := plan.OrganizationName.ValueString(); orgName != "" {
				opts = append(opts, gcp.Organization(orgName))
			}
			if _, err := gcp.Wrap(polarisClient).AddProject(ctx, project, addFeatures, opts...); err != nil {
				res.Diagnostics.AddError("Failed to add GCP project features", err.Error())
				return
			}
		}
		if len(removeFeatures) > 0 {
			err := gcp.Wrap(polarisClient).RemoveProject(ctx, id, removeFeatures,
				plan.DeleteSnapshotsOnDestroy.ValueBool())
			if err != nil {
				res.Diagnostics.AddError("Failed to remove GCP project features", err.Error())
				return
			}
		}
		if len(permFeatures) > 0 {
			if err := gcp.Wrap(polarisClient).PermissionsUpdated(ctx, id, permFeatures); err != nil {
				res.Diagnostics.AddError("Failed to signal GCP project permissions updated", err.Error())
				return
			}
		}
	}

	if !plan.PermissionsHash.Equal(state.PermissionsHash) {
		if err := gcp.Wrap(polarisClient).PermissionsUpdated(ctx, id, nil); err != nil {
			res.Diagnostics.AddError("Failed to signal GCP project permissions updated", err.Error())
			return
		}
	}

	plan.ID = state.ID
	account, err := gcp.Wrap(polarisClient).ProjectByID(ctx, id)
	if err != nil {
		res.Diagnostics.AddError("Failed to read GCP project", err.Error())
		return
	}
	res.Diagnostics.Append(setGCPProjectStatus(ctx, &plan, &account)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.Identity.Set(ctx, gcpProjectIdentityModel{ID: plan.ID})...)
}

func (r *gcpProjectResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "gcpProjectResource.Delete")

	var state gcpProjectResourceModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid GCP project ID", err.Error())
		return
	}

	// The features are read from RSC rather than taken from the state. With
	// the deprecated cloud_native_protection block the feature set in state is
	// empty, the SDKv2 resource always held the features in RSC there, since
	// the set was computed. Removing all features in RSC removes the project.
	account, err := gcp.Wrap(polarisClient).ProjectByID(ctx, id)
	if errors.Is(err, graphql.ErrNotFound) {
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read GCP project", err.Error())
		return
	}
	features := make([]core.Feature, 0, len(account.Features))
	for _, feature := range account.Features {
		features = append(features, feature.Feature)
	}

	err = gcp.Wrap(polarisClient).RemoveProject(ctx, id, features, state.DeleteSnapshotsOnDestroy.ValueBool())
	if err != nil && !errors.Is(err, graphql.ErrNotFound) {
		res.Diagnostics.AddError("Failed to remove GCP project", err.Error())
		return
	}
}

func (r *gcpProjectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "gcpProjectResource.ImportState")

	resource.ImportStatePassthroughWithIdentity(ctx, path.Root(keyID), path.Root(keyID), req, res)
	if res.Diagnostics.HasError() {
		return
	}

	// The default isn't applied to imported state, Read keeps the value.
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyDeleteSnapshotsOnDestroy), false)...)
}

// gcpCheckCloudDiscoveryRemoval returns an error if the CLOUD_DISCOVERY feature
// is being removed from a project while the new set of features still contains
// a protection feature. Cloud Discovery is the parent of the protection
// features and must outlive them.
func gcpCheckCloudDiscoveryRemoval(oldFeatures, newFeatures []string) error {
	if !slices.Contains(oldFeatures, core.FeatureCloudDiscovery.Name) ||
		slices.Contains(newFeatures, core.FeatureCloudDiscovery.Name) {
		return nil
	}

	for _, name := range newFeatures {
		if (core.Feature{Name: name}).IsProtectionFeature() {
			return fmt.Errorf("feature %s cannot be removed while the protection feature %s is enabled",
				core.FeatureCloudDiscovery.Name, name)
		}
	}

	return nil
}

// setGCPProjectStatus sets the computed status of each feature, and of the
// Cloud Native Protection block, from the GCP project read from RSC after an
// apply. Configured values are left as they are, Terraform requires the state
// after an apply to match them. A status RSC doesn't return, or all of them
// when the account is nil because the read failed, is set to null, since an
// unknown value can't be saved to state.
func setGCPProjectStatus(ctx context.Context, m *gcpProjectResourceModel, account *gcp.CloudAccount) diag.Diagnostics {
	var diags diag.Diagnostics

	status := func(feature core.Feature) types.String {
		if account == nil {
			return types.StringNull()
		}
		for _, f := range account.Features {
			if f.Name == feature.Name {
				return types.StringValue(string(f.Status))
			}
		}
		return types.StringNull()
	}

	features := []gcpProjectFeatureModel{}
	diags.Append(m.Feature.ElementsAs(ctx, &features, false)...)
	cnp := []gcpProjectCloudNativeProtectionModel{}
	diags.Append(m.CloudNativeProtection.ElementsAs(ctx, &cnp, false)...)
	if diags.HasError() {
		return diags
	}

	for i := range features {
		features[i].Status = status(core.Feature{Name: features[i].Name.ValueString()})
	}
	// The status of the Cloud Native Protection block is also optional, a
	// configured value is kept.
	for i := range cnp {
		if cnp[i].Status.IsUnknown() || cnp[i].Status.IsNull() {
			cnp[i].Status = status(core.FeatureCloudNativeProtection)
		}
	}

	set, d := types.SetValueFrom(ctx, gcpProjectFeatureType, features)
	diags.Append(d...)
	list, d := types.ListValueFrom(ctx, gcpProjectCloudNativeProtectionType, cnp)
	diags.Append(d...)
	m.Feature = set
	m.CloudNativeProtection = list

	return diags
}

// gcpProjectFeatures converts the feature block to features with their
// permissions signal.
func gcpProjectFeatures(ctx context.Context, set types.Set) ([]featureWithPermissions, diag.Diagnostics) {
	var models []gcpProjectFeatureModel
	diags := set.ElementsAs(ctx, &models, false)
	if diags.HasError() {
		return nil, diags
	}

	features := make([]featureWithPermissions, 0, len(models))
	for _, model := range models {
		var groups []string
		diags.Append(model.PermissionGroups.ElementsAs(ctx, &groups, false)...)
		if diags.HasError() {
			return nil, diags
		}
		feature := core.Feature{Name: model.Name.ValueString()}
		for _, group := range groups {
			feature.PermissionGroups = append(feature.PermissionGroups, core.PermissionGroup(group))
		}
		features = append(features, featureWithPermissions{
			Feature:     feature,
			permissions: model.Permissions.ValueString(),
		})
	}

	return features, diags
}

// updateGCPProjectResourceModel updates the model with the GCP project read
// from RSC.
//
// The Plugin Framework has no computed blocks, so only the block in use is
// populated. A model with Cloud Native Protection, the deprecated block, but
// no features keeps using the deprecated block. Otherwise the features are
// populated, including after an import. The permissions signal of a feature
// is never returned by RSC and is kept from the model.
func updateGCPProjectResourceModel(ctx context.Context, m *gcpProjectResourceModel, account gcp.CloudAccount) diag.Diagnostics {
	var diags diag.Diagnostics

	m.Project = types.StringValue(account.NativeID)
	m.ProjectName = types.StringValue(account.Name)
	m.ProjectNumber = types.StringValue(strconv.FormatInt(account.ProjectNumber, 10))
	// RSC returns an empty organization name when it has none. An empty
	// string in state can only come from a configuration setting it to "",
	// the state upgraders and movers turn the empty strings written by the
	// SDKv2 resource into null. It's kept, otherwise the next plan would show
	// a change from null to "".
	switch {
	case account.OrganizationName != "":
		m.OrganizationName = types.StringValue(account.OrganizationName)
	case m.OrganizationName.ValueString() != "":
		m.OrganizationName = types.StringNull()
	}

	if m.DeleteSnapshotsOnDestroy.IsNull() {
		m.DeleteSnapshotsOnDestroy = types.BoolValue(false)
	}

	useCNP := len(m.CloudNativeProtection.Elements()) > 0 && len(m.Feature.Elements()) == 0
	if useCNP {
		cnp := []gcpProjectCloudNativeProtectionModel{}
		if feature, ok := account.Feature(core.FeatureCloudNativeProtection); ok {
			cnp = append(cnp, gcpProjectCloudNativeProtectionModel{Status: types.StringValue(string(feature.Status))})
		}
		list, d := types.ListValueFrom(ctx, gcpProjectCloudNativeProtectionType, cnp)
		diags.Append(d...)
		m.CloudNativeProtection = list
		m.Feature = types.SetValueMust(gcpProjectFeatureType, nil)
		return diags
	}

	oldFeatures, d := gcpProjectFeatures(ctx, m.Feature)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	permissions := make(map[string]string, len(oldFeatures))
	for _, feature := range oldFeatures {
		permissions[feature.Name] = feature.permissions
	}

	features := make([]gcpProjectFeatureModel, 0, len(account.Features))
	for _, feature := range account.Features {
		groups := make([]string, 0, len(feature.PermissionGroups))
		for _, group := range feature.PermissionGroups {
			groups = append(groups, string(group))
		}
		groupSet, d := types.SetValueFrom(ctx, types.StringType, groups)
		diags.Append(d...)
		permission := types.StringNull()
		if p := permissions[feature.Name]; p != "" {
			permission = types.StringValue(p)
		}
		features = append(features, gcpProjectFeatureModel{
			Name:             types.StringValue(feature.Name),
			PermissionGroups: groupSet,
			Permissions:      permission,
			Status:           types.StringValue(string(feature.Status)),
		})
	}
	set, d := types.SetValueFrom(ctx, gcpProjectFeatureType, features)
	diags.Append(d...)
	m.Feature = set
	m.CloudNativeProtection = types.ListValueMust(gcpProjectCloudNativeProtectionType, nil)

	return diags
}
