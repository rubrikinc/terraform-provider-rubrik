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
	"maps"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
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
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/azure"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
	regions "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/azure"
)

const resourceAzureManagementGroupDescription = `
The ´rubrik_azure_management_group´ resource onboards the Azure subscriptions of an Azure management group to RSC,
without OAuth. One resource manages the subscriptions onboarded through a single management group in a single tenant,
and the features enabled on them. All the subscriptions get the same features.

RSC signs in as the customer app registered with the ´rubrik_azure_service_principal´ resource, and creates the role
definitions at the management group scope and the role assignments for each subscription. The app's service principal
must therefore have the ´Contributor´ and ´User Access Administrator´ roles at the management group scope. RSC can
only remove its role assignments when these roles are in place, so keep them until the resource is destroyed.

~> **Note:** The RSC feature flag ´AZURE_CLOUDACCOUNTS_MANAGEMENT_GROUP_ENABLED´ must be enabled for the RSC account.

~> **Note:** A subscription must not be managed by both this resource and the ´rubrik_azure_subscription´ resource.
Once a subscription has been onboarded through a management group, it can no longer be updated through
´rubrik_azure_subscription´.

## Limitations
  * OAuth onboarding of management groups needs interactive consent in a browser and is not supported.
  * Subscriptions added to the management group later are not onboarded automatically. Add them to
    ´subscription_ids´.
  * Changing ´regions´, or the ´permission_groups´ or resource group of an existing ´feature´, is not supported. Remove
    the feature and add it again, or create a new resource.
  * Azure Government Cloud is not supported.

Subscriptions and features removed from the configuration are disabled in RSC, the snapshots are kept. When the resource
is destroyed, the snapshots are deleted if ´delete_snapshots_on_destroy´ is ´true´.

## Import
The resource ID is ´<tenant_domain>/<management_group_id>´.
`

var (
	_ resource.Resource                = &azureManagementGroupResource{}
	_ resource.ResourceWithConfigure   = &azureManagementGroupResource{}
	_ resource.ResourceWithIdentity    = &azureManagementGroupResource{}
	_ resource.ResourceWithImportState = &azureManagementGroupResource{}
)

type azureManagementGroupResource struct {
	client *client
}

// azureManagementGroupModel is the resource state. Field order follows the
// schema.
type azureManagementGroupModel struct {
	ID                        types.String `tfsdk:"id"`
	TenantDomain              types.String `tfsdk:"tenant_domain"`
	AppID                     types.String `tfsdk:"app_id"`
	ManagementGroupID         types.String `tfsdk:"management_group_id"`
	ManagementGroupName       types.String `tfsdk:"management_group_name"`
	CustomerManagementGroupID types.String `tfsdk:"customer_management_group_id"`
	Regions                   types.Set    `tfsdk:"regions"`
	SubscriptionIDs           types.Set    `tfsdk:"subscription_ids"`
	DeleteSnapshotsOnDestroy  types.Bool   `tfsdk:"delete_snapshots_on_destroy"`
	Feature                   types.Set    `tfsdk:"feature"`
}

// azureManagementGroupFeatureModel is the model of a feature block.
type azureManagementGroupFeatureModel struct {
	Name                types.String `tfsdk:"name"`
	PermissionGroups    types.Set    `tfsdk:"permission_groups"`
	ResourceGroupName   types.String `tfsdk:"resource_group_name"`
	ResourceGroupRegion types.String `tfsdk:"resource_group_region"`
	ResourceGroupTags   types.Map    `tfsdk:"resource_group_tags"`
}

type azureManagementGroupIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

// azureManagementGroupFeatureAttrTypes are the attribute types of a feature
// block.
var azureManagementGroupFeatureAttrTypes = map[string]attr.Type{
	keyName:                types.StringType,
	keyPermissionGroups:    types.SetType{ElemType: types.StringType},
	keyResourceGroupName:   types.StringType,
	keyResourceGroupRegion: types.StringType,
	keyResourceGroupTags:   types.MapType{ElemType: types.StringType},
}

func newAzureManagementGroupResource() resource.Resource {
	return &azureManagementGroupResource{}
}

func (r *azureManagementGroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "azureManagementGroupResource.Metadata")

	res.TypeName = keyRubrik + "_" + keyAzureManagementGroup
}

func (r *azureManagementGroupResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "azureManagementGroupResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceAzureManagementGroupDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "Resource ID, in the format `<tenant_domain>/<management_group_id>`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyTenantDomain: schema.StringAttribute{
				Required:    true,
				Description: "Azure tenant primary domain. Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyAppID: schema.StringAttribute{
				Required: true,
				Description: "Azure app registration application ID (UUID) of the customer app, as set with the " +
					"`rubrik_azure_service_principal` resource. The secret stored in RSC for the app is used. " +
					"Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyManagementGroupID: schema.StringAttribute{
				Required:    true,
				Description: "Azure management group ID. Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyManagementGroupName: schema.StringAttribute{
				Computed:    true,
				Description: "Azure management group display name.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyCustomerManagementGroupID: schema.StringAttribute{
				Computed:    true,
				Description: "RSC customer management group ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyRegions: schema.SetAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: "Azure regions to enable the features in. Should be specified in the standard Azure " +
					"style, e.g. `eastus`. Changing the regions of an existing resource is not supported.",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(stringvalidator.OneOf(regions.AllRegionNames()...)),
				},
			},
			keySubscriptionIDs: schema.SetAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: "Azure subscription IDs (UUIDs) to onboard. The subscriptions must be in the management " +
					"group and eligible for the features. Subscriptions removed from the set have their features " +
					"disabled in RSC.",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(isUUID()),
				},
			},
			keyDeleteSnapshotsOnDestroy: schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				Description: "Delete the snapshots of the subscriptions when the resource is destroyed. Default " +
					"value is `false`.",
			},
		},
		Blocks: map[string]schema.Block{
			keyFeature: schema.SetNestedBlock{
				Description: "RSC features to enable on all the subscriptions. At least one is required.",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyName: schema.StringAttribute{
							Required: true,
							Description: fmt.Sprintf("RSC feature name. %s.",
								possibleValues(core.AzureSubscriptionFeatureNames())),
							Validators: []validator.String{
								stringvalidator.OneOf(core.AzureSubscriptionFeatureNames()...),
							},
						},
						keyPermissionGroups: schema.SetAttribute{
							ElementType: types.StringType,
							Required:    true,
							Description: "Permission groups for the RSC feature. At least one is required.",
							Validators: []validator.Set{
								setvalidator.SizeAtLeast(1),
							},
						},
						keyResourceGroupName: schema.StringAttribute{
							Optional: true,
							Description: "Name of the Azure resource group RSC creates the feature's resources in. " +
								"Required for the `CLOUD_NATIVE_ARCHIVAL` feature, ignored for other features.",
							Validators: []validator.String{
								isNotWhiteSpace(),
								stringvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName(keyResourceGroupRegion)),
							},
						},
						keyResourceGroupRegion: schema.StringAttribute{
							Optional:    true,
							Description: "Region of the Azure resource group, e.g. `eastus`.",
							Validators: []validator.String{
								stringvalidator.OneOf(regions.AllRegionNames()...),
								stringvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName(keyResourceGroupName)),
							},
						},
						keyResourceGroupTags: schema.MapAttribute{
							ElementType: types.StringType,
							Optional:    true,
							Description: "Tags to add to the Azure resource group.",
							Validators: []validator.Map{
								mapvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName(keyResourceGroupName)),
							},
						},
					},
				},
			},
		},
	}
}

func (r *azureManagementGroupResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "azureManagementGroupResource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Resource ID, in the format `<tenant_domain>/<management_group_id>`.",
			},
		},
	}
}

func (r *azureManagementGroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "azureManagementGroupResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *azureManagementGroupResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "azureManagementGroupResource.Create")

	var plan azureManagementGroupModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}
	api := azure.Wrap(polarisClient)

	input, diags := azureManagementGroupInputFromModel(ctx, plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	tenantDomain, managementGroupID := plan.TenantDomain.ValueString(), plan.ManagementGroupID.ValueString()
	addErr := api.AddManagementGroupSubscriptions(ctx, tenantDomain, input.appID, managementGroupID, input.regions,
		azureManagementGroupSubscriptions(input.subscriptions, input.features))

	// Onboarding is asynchronous and can partly succeed. If any subscription
	// ended up in the management group, we save the ID and identity so the
	// resource is tainted rather than orphaned in RSC.
	data, err := readAzureManagementGroup(ctx, api, tenantDomain, managementGroupID)
	if err != nil && !errors.Is(err, graphql.ErrNotFound) && addErr == nil {
		res.Diagnostics.AddError("Failed to read management group after create", err.Error())
		return
	}
	if err == nil || addErr == nil {
		plan.ID = types.StringValue(tenantDomain + "/" + managementGroupID)
		plan.ManagementGroupName = types.StringNull()
		plan.CustomerManagementGroupID = types.StringNull()
		if err == nil {
			plan.ManagementGroupName = types.StringValue(data.group.Name)
			plan.CustomerManagementGroupID = types.StringValue(data.group.CustomerID.String())
		}
		res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
		res.Diagnostics.Append(res.Identity.Set(ctx, azureManagementGroupIdentityModel{ID: plan.ID})...)
	}
	if addErr != nil {
		res.Diagnostics.AddError("Failed to onboard management group subscriptions", addErr.Error())
	}
}

func (r *azureManagementGroupResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "azureManagementGroupResource.Read")

	var state azureManagementGroupModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	tenantDomain, managementGroupID, ok := strings.Cut(state.ID.ValueString(), "/")
	if !ok || tenantDomain == "" || managementGroupID == "" {
		res.Diagnostics.AddError("Invalid management group ID",
			fmt.Sprintf("Expected the format <tenant_domain>/<management_group_id>, got %q", state.ID.ValueString()))
		return
	}

	data, err := readAzureManagementGroup(ctx, azure.Wrap(polarisClient), tenantDomain, managementGroupID)
	if errors.Is(err, graphql.ErrNotFound) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read management group", err.Error())
		return
	}

	res.Diagnostics.Append(data.setModel(ctx, &state, tenantDomain, managementGroupID)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.Identity.Set(ctx, azureManagementGroupIdentityModel{ID: state.ID})...)
}

func (r *azureManagementGroupResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "azureManagementGroupResource.Update")

	var plan, state azureManagementGroupModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}
	api := azure.Wrap(polarisClient)

	newInput, diags := azureManagementGroupInputFromModel(ctx, plan)
	res.Diagnostics.Append(diags...)
	oldInput, diags := azureManagementGroupInputFromModel(ctx, state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	changes, err := azureManagementGroupDiff(oldInput, newInput)
	if err != nil {
		res.Diagnostics.AddError("Unsupported management group change", err.Error())
		return
	}

	tenantDomain, managementGroupID := plan.TenantDomain.ValueString(), plan.ManagementGroupID.ValueString()
	data, err := readAzureManagementGroup(ctx, api, tenantDomain, managementGroupID)
	if err != nil {
		res.Diagnostics.AddError("Failed to read management group", err.Error())
		return
	}
	accountIDs := func(nativeIDs []uuid.UUID) []uuid.UUID {
		var ids []uuid.UUID
		for _, account := range data.accounts {
			if slices.Contains(nativeIDs, account.NativeID) {
				ids = append(ids, account.ID)
			}
		}
		return ids
	}

	// Remove before adding, features first. The state is saved after every
	// step that changed RSC, so a failure later doesn't lose track of it.
	retained := make([]uuid.UUID, 0, len(newInput.subscriptions))
	for _, id := range newInput.subscriptions {
		if !slices.Contains(changes.addSubscriptions, id) {
			retained = append(retained, id)
		}
	}
	if len(changes.removeFeatures) > 0 {
		err := removeAzureManagementGroupFeatures(ctx, api, tenantDomain, newInput.appID, accountIDs(retained), changes.removeFeatures, false)
		if err != nil {
			res.Diagnostics.AddError("Failed to remove features", err.Error())
			return
		}
	}
	if len(changes.removeSubscriptions) > 0 {
		names := make([]string, 0, len(oldInput.features))
		for _, feature := range oldInput.features {
			names = append(names, feature.name)
		}
		err := removeAzureManagementGroupFeatures(ctx, api, tenantDomain, newInput.appID, accountIDs(changes.removeSubscriptions), names, false)
		if err != nil {
			res.Diagnostics.AddError("Failed to remove subscriptions", err.Error())
			return
		}
	}
	if len(changes.addFeatures) > 0 && len(retained) > 0 {
		err := api.AddManagementGroupSubscriptions(ctx, tenantDomain, newInput.appID, managementGroupID, newInput.regions,
			azureManagementGroupSubscriptions(retained, changes.addFeatures))
		if err != nil {
			res.Diagnostics.AddError("Failed to add features", err.Error())
			return
		}
	}
	if len(changes.addSubscriptions) > 0 {
		err := api.AddManagementGroupSubscriptions(ctx, tenantDomain, newInput.appID, managementGroupID, newInput.regions,
			azureManagementGroupSubscriptions(changes.addSubscriptions, newInput.features))
		if err != nil {
			res.Diagnostics.AddError("Failed to add subscriptions", err.Error())
			return
		}
	}

	plan.ID = state.ID
	plan.ManagementGroupName = state.ManagementGroupName
	plan.CustomerManagementGroupID = state.CustomerManagementGroupID
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.Identity.Set(ctx, azureManagementGroupIdentityModel{ID: plan.ID})...)
}

func (r *azureManagementGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "azureManagementGroupResource.Delete")

	var state azureManagementGroupModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}
	api := azure.Wrap(polarisClient)

	input, diags := azureManagementGroupInputFromModel(ctx, state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	data, err := readAzureManagementGroup(ctx, api, state.TenantDomain.ValueString(), state.ManagementGroupID.ValueString())
	if errors.Is(err, graphql.ErrNotFound) {
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read management group", err.Error())
		return
	}

	ids := make([]uuid.UUID, 0, len(data.accounts))
	for _, account := range data.accounts {
		ids = append(ids, account.ID)
	}
	names := make([]string, 0, len(input.features))
	for _, feature := range input.features {
		names = append(names, feature.name)
	}
	err = removeAzureManagementGroupFeatures(ctx, api, state.TenantDomain.ValueString(), input.appID, ids, names,
		state.DeleteSnapshotsOnDestroy.ValueBool())
	if err != nil {
		res.Diagnostics.AddError("Failed to remove management group subscriptions", err.Error())
	}
}

func (r *azureManagementGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "azureManagementGroupResource.ImportState")

	resource.ImportStatePassthroughWithIdentity(ctx, path.Root(keyID), path.Root(keyID), req, res)
	if res.Diagnostics.HasError() {
		return
	}

	// The default isn't applied to imported state.
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyDeleteSnapshotsOnDestroy), false)...)
}

// azureManagementGroupFeature is a feature block in plain Go types.
type azureManagementGroupFeature struct {
	name                string
	permissionGroups    []string
	resourceGroupName   string
	resourceGroupRegion string
	resourceGroupTags   map[string]string
}

// azureManagementGroupInput is the configuration of the resource in plain Go
// types.
type azureManagementGroupInput struct {
	appID         uuid.UUID
	regions       []regions.Region
	subscriptions []uuid.UUID
	features      []azureManagementGroupFeature
}

func azureManagementGroupInputFromModel(ctx context.Context, model azureManagementGroupModel) (azureManagementGroupInput, diag.Diagnostics) {
	var diags diag.Diagnostics
	var input azureManagementGroupInput

	appID, err := uuid.Parse(model.AppID.ValueString())
	if err != nil {
		diags.AddError("Invalid app ID", err.Error())
		return input, diags
	}
	input.appID = appID

	var regionNames, subscriptionIDs []string
	diags.Append(model.Regions.ElementsAs(ctx, &regionNames, false)...)
	diags.Append(model.SubscriptionIDs.ElementsAs(ctx, &subscriptionIDs, false)...)
	for _, name := range regionNames {
		input.regions = append(input.regions, regions.RegionFromName(name))
	}
	for _, id := range subscriptionIDs {
		subscriptionID, err := uuid.Parse(id)
		if err != nil {
			diags.AddError("Invalid subscription ID", err.Error())
			continue
		}
		input.subscriptions = append(input.subscriptions, subscriptionID)
	}

	var featureModels []azureManagementGroupFeatureModel
	diags.Append(model.Feature.ElementsAs(ctx, &featureModels, false)...)
	for _, featureModel := range featureModels {
		feature := azureManagementGroupFeature{
			name:                featureModel.Name.ValueString(),
			resourceGroupName:   featureModel.ResourceGroupName.ValueString(),
			resourceGroupRegion: featureModel.ResourceGroupRegion.ValueString(),
		}
		diags.Append(featureModel.PermissionGroups.ElementsAs(ctx, &feature.permissionGroups, false)...)
		diags.Append(featureModel.ResourceGroupTags.ElementsAs(ctx, &feature.resourceGroupTags, false)...)
		slices.Sort(feature.permissionGroups)
		input.features = append(input.features, feature)
	}
	slices.SortFunc(input.features, func(a, b azureManagementGroupFeature) int { return strings.Compare(a.name, b.name) })

	return input, diags
}

// azureManagementGroupSubscriptions returns the SDK subscriptions for the
// subscription IDs, all with the given features.
func azureManagementGroupSubscriptions(ids []uuid.UUID, features []azureManagementGroupFeature) []azure.ManagementGroupSubscription {
	sdkFeatures := make([]azure.ManagementGroupFeature, 0, len(features))
	for _, feature := range features {
		permGroups := make([]core.PermissionGroup, 0, len(feature.permissionGroups))
		for _, group := range feature.permissionGroups {
			permGroups = append(permGroups, core.PermissionGroup(group))
		}
		sdkFeature := azure.ManagementGroupFeature{Feature: core.Feature{Name: feature.name, PermissionGroups: permGroups}}
		if feature.resourceGroupName != "" {
			sdkFeature.ResourceGroup = &azure.ManagementGroupResourceGroup{
				Name:   feature.resourceGroupName,
				Region: regions.RegionFromName(feature.resourceGroupRegion),
				Tags:   feature.resourceGroupTags,
			}
		}
		sdkFeatures = append(sdkFeatures, sdkFeature)
	}

	subscriptions := make([]azure.ManagementGroupSubscription, 0, len(ids))
	for _, id := range ids {
		subscriptions = append(subscriptions, azure.ManagementGroupSubscription{NativeID: id, Features: sdkFeatures})
	}

	return subscriptions
}

// azureManagementGroupChanges are the changes needed to go from one
// configuration to another.
type azureManagementGroupChanges struct {
	addSubscriptions    []uuid.UUID
	removeSubscriptions []uuid.UUID
	// addFeatures are the features to add to the retained subscriptions.
	addFeatures []azureManagementGroupFeature
	// removeFeatures are the names of the features to remove from the retained
	// subscriptions.
	removeFeatures []string
}

// azureManagementGroupDiff returns the changes needed to go from the old to
// the new configuration. Changes RSC has no operation for, changing the
// regions or the details of an existing feature, are returned as an error.
func azureManagementGroupDiff(oldInput, newInput azureManagementGroupInput) (azureManagementGroupChanges, error) {
	var changes azureManagementGroupChanges

	oldRegions := slices.Sorted(maps.Keys(setOf(oldInput.regions)))
	newRegions := slices.Sorted(maps.Keys(setOf(newInput.regions)))
	if !slices.Equal(oldRegions, newRegions) {
		return changes, errors.New("changing regions is not supported, create a new resource instead")
	}

	for _, id := range newInput.subscriptions {
		if !slices.Contains(oldInput.subscriptions, id) {
			changes.addSubscriptions = append(changes.addSubscriptions, id)
		}
	}
	for _, id := range oldInput.subscriptions {
		if !slices.Contains(newInput.subscriptions, id) {
			changes.removeSubscriptions = append(changes.removeSubscriptions, id)
		}
	}

	for _, feature := range newInput.features {
		idx := slices.IndexFunc(oldInput.features, func(f azureManagementGroupFeature) bool { return f.name == feature.name })
		if idx < 0 {
			changes.addFeatures = append(changes.addFeatures, feature)
			continue
		}
		old := oldInput.features[idx]
		if !slices.Equal(old.permissionGroups, feature.permissionGroups) || old.resourceGroupName != feature.resourceGroupName ||
			old.resourceGroupRegion != feature.resourceGroupRegion || !maps.Equal(old.resourceGroupTags, feature.resourceGroupTags) {
			return changes, fmt.Errorf("changing the permission groups or the resource group of the existing feature %s "+
				"is not supported, remove the feature and add it again", feature.name)
		}
	}
	for _, feature := range oldInput.features {
		if !slices.ContainsFunc(newInput.features, func(f azureManagementGroupFeature) bool { return f.name == feature.name }) {
			changes.removeFeatures = append(changes.removeFeatures, feature.name)
		}
	}

	return changes, nil
}

// setOf returns the values as a set.
func setOf[T comparable](values []T) map[T]struct{} {
	set := make(map[T]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}

	return set
}

// removeAzureManagementGroupFeatures disables the features on the cloud
// accounts, one feature at a time. The Cloud Discovery feature is the parent of
// the protection features and must be removed last.
func removeAzureManagementGroupFeatures(ctx context.Context, api azure.API, tenantDomain string, appID uuid.UUID,
	accountIDs []uuid.UUID, featureNames []string, deleteSnapshots bool) error {
	names := slices.Clone(featureNames)
	slices.SortStableFunc(names, func(a, b string) int {
		return boolToInt(a == core.FeatureCloudDiscovery.Name) - boolToInt(b == core.FeatureCloudDiscovery.Name)
	})

	for _, name := range names {
		if err := api.RemoveManagementGroupSubscriptionFeature(ctx, tenantDomain, appID, accountIDs, core.Feature{Name: name},
			deleteSnapshots); err != nil {
			return err
		}
	}

	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}

	return 0
}

// azureManagementGroupData is the management group as read from RSC.
type azureManagementGroupData struct {
	appID    uuid.UUID
	group    azure.ManagementGroup
	accounts []azure.CloudAccount
}

// readAzureManagementGroup reads the subscriptions onboarded through the
// management group. Features that are disabled are ignored, and so are
// subscriptions without any other features. Returns graphql.ErrNotFound if no
// subscriptions remain.
func readAzureManagementGroup(ctx context.Context, api azure.API, tenantDomain, managementGroupID string) (azureManagementGroupData, error) {
	accounts, err := api.Subscriptions(ctx, "")
	if err != nil {
		return azureManagementGroupData{}, err
	}

	var data azureManagementGroupData
	for _, account := range accounts {
		mg := account.ManagementGroup
		if mg == nil || !strings.EqualFold(mg.NativeID, managementGroupID) || !strings.EqualFold(account.TenantDomain, tenantDomain) {
			continue
		}

		account.Features = slices.DeleteFunc(slices.Clone(account.Features), func(f azure.Feature) bool {
			return f.Status == core.StatusDisabled
		})
		if len(account.Features) == 0 {
			continue
		}
		data.group = *mg
		data.accounts = append(data.accounts, account)
	}
	if len(data.accounts) == 0 {
		return azureManagementGroupData{}, fmt.Errorf("management group %s: %w", managementGroupID, graphql.ErrNotFound)
	}

	tenants, err := api.Tenants(ctx, tenantDomain)
	if err != nil {
		return azureManagementGroupData{}, err
	}
	for _, tenant := range tenants {
		if strings.EqualFold(tenant.DomainName, tenantDomain) {
			data.appID = tenant.ClientID
		}
	}

	return data, nil
}

// setModel updates the model with the management group read from RSC. Feature
// details RSC always returns, but that have not been configured, e.g. the
// resource group of a protection feature, are kept out of the state.
func (d azureManagementGroupData) setModel(ctx context.Context, model *azureManagementGroupModel, tenantDomain, managementGroupID string) diag.Diagnostics {
	var diags diag.Diagnostics

	var prior []azureManagementGroupFeatureModel
	diags.Append(model.Feature.ElementsAs(ctx, &prior, false)...)

	model.ID = types.StringValue(tenantDomain + "/" + managementGroupID)
	model.TenantDomain = types.StringValue(tenantDomain)
	model.ManagementGroupID = types.StringValue(d.group.NativeID)
	model.ManagementGroupName = types.StringValue(d.group.Name)
	model.CustomerManagementGroupID = types.StringValue(d.group.CustomerID.String())
	if d.appID != uuid.Nil {
		model.AppID = types.StringValue(d.appID.String())
	}
	if model.DeleteSnapshotsOnDestroy.IsNull() || model.DeleteSnapshotsOnDestroy.IsUnknown() {
		model.DeleteSnapshotsOnDestroy = types.BoolValue(false)
	}

	subscriptionIDs := make([]attr.Value, 0, len(d.accounts))
	regionSet := make(map[string]struct{})
	featureByName := make(map[string]azure.Feature)
	for _, account := range d.accounts {
		subscriptionIDs = append(subscriptionIDs, types.StringValue(account.NativeID.String()))
		for _, feature := range account.Features {
			if _, ok := featureByName[feature.Name]; !ok {
				featureByName[feature.Name] = feature
			}
			for _, region := range feature.Regions {
				regionSet[region] = struct{}{}
			}
		}
	}
	model.SubscriptionIDs = types.SetValueMust(types.StringType, subscriptionIDs)

	regionValues := make([]attr.Value, 0, len(regionSet))
	for _, region := range slices.Sorted(maps.Keys(regionSet)) {
		regionValues = append(regionValues, types.StringValue(region))
	}
	model.Regions = types.SetValueMust(types.StringType, regionValues)

	featureValues := make([]attr.Value, 0, len(featureByName))
	for _, name := range slices.Sorted(maps.Keys(featureByName)) {
		feature := featureByName[name]

		permGroups := make([]attr.Value, 0, len(feature.PermissionGroups))
		for _, group := range feature.PermissionGroups {
			permGroups = append(permGroups, types.StringValue(string(group)))
		}

		featureModel := azureManagementGroupFeatureModel{
			Name:                types.StringValue(name),
			PermissionGroups:    types.SetValueMust(types.StringType, permGroups),
			ResourceGroupName:   types.StringNull(),
			ResourceGroupRegion: types.StringNull(),
			ResourceGroupTags:   types.MapNull(types.StringType),
		}
		idx := slices.IndexFunc(prior, func(p azureManagementGroupFeatureModel) bool { return p.Name.ValueString() == name })
		configured := idx < 0 || !prior[idx].ResourceGroupName.IsNull()
		if configured && feature.ResourceGroup.Name != "" {
			featureModel.ResourceGroupName = types.StringValue(feature.ResourceGroup.Name)
			featureModel.ResourceGroupRegion = types.StringValue(feature.ResourceGroup.Region)
			if len(feature.ResourceGroup.Tags) > 0 || (idx >= 0 && !prior[idx].ResourceGroupTags.IsNull()) {
				tags, tagDiags := types.MapValueFrom(ctx, types.StringType, feature.ResourceGroup.Tags)
				diags.Append(tagDiags...)
				featureModel.ResourceGroupTags = tags
			}
		}

		value, objDiags := types.ObjectValueFrom(ctx, azureManagementGroupFeatureAttrTypes, featureModel)
		diags.Append(objDiags...)
		featureValues = append(featureValues, value)
	}
	model.Feature = types.SetValueMust(types.ObjectType{AttrTypes: azureManagementGroupFeatureAttrTypes}, featureValues)

	return diags
}
