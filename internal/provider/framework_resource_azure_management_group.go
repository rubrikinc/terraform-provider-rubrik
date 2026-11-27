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
	"time"

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
	gqlazure "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/azure"
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

~> **Note:** The RSC account must have ´AZURE_CLOUDACCOUNTS_MANAGEMENT_GROUP_ENABLED´ enabled.
´CNP_AZURE_DISABLE_DELETE_COMBINED´ is not required. If it is off, the provider deletes the feature and its Azure role
assignments in a separate step after any required disable job completes.

~> **Note:** A subscription must not be managed by both this resource and the ´rubrik_azure_subscription´ resource.
Once a subscription has been onboarded through a management group, it can no longer be updated through
´rubrik_azure_subscription´.

## Cloud Discovery and Cost Reporting
Include a ´CLOUD_DISCOVERY´ feature entry with the ´BASIC´ permission group. The resource onboards Cloud Discovery on
every managed subscription alongside any other configured features. You cannot remove Cloud Discovery while keeping
a subscription in this resource.

RSC may also add a Cloud Cost Reporting permission prompt after protection connects. The resulting ´CLOUD_COST_REPORT´
row is not managed by this Azure resource. It does not change Terraform state, and destroying this resource does not
remove the Cost Reporting row.

## Exocompute
As with standalone Azure subscriptions, RSC maps an ´EXOCOMPUTE´ subscription to itself if it has no existing mapping.
The default mapping alone does not mean a host is configured. Use ´rubrik_azure_exocompute´ to configure a host or map
workload subscriptions to a different host. Look up a management-group subscription with the ´rubrik_azure_subscription´
**data source** and use its RSC ´id´ as ´cloud_account_id´ or ´host_cloud_account_id´. Do not use the native Azure
subscription ID or the management group ID for either field. If the data source could be read before onboarding, add
a dependency on the management-group resource.

## Limitations
  * OAuth onboarding of management groups needs interactive consent in a browser and is not supported.
  * Subscriptions added to the management group later are not onboarded automatically. Add them to
    ´subscription_ids´.
  * Region additions require an already connected ´CLOUD_DISCOVERY´ feature managed by this resource on every
    subscription. Apply the region addition separately from subscription or feature changes. RSC aligns regional
    features such as CNP, Blob, and SQL; Exocompute host regions are configured separately. The provider refuses an
    update that would overwrite unmanaged feature regions or remove an existing region. Region removals are not supported.
  * Changing the ´permission_groups´ or resource group of an existing ´feature´ is not supported. Remove the feature
    and add it again, or create a new resource.
  * Azure Government Cloud is not supported.
  * For features that require a user-assigned managed identity, such as PostgreSQL Flexible Server Protection and
    Cloud Native Archival Encryption, specify the resource group. RSC creates the managed identity as part of
    management-group onboarding. The public management-group API does not accept an existing identity as input.

Only subscription/feature pairs that this resource submitted or imported are removed from RSC. Changes made outside
Terraform are not adopted on refresh. Removing Cloud Native Protection preserves snapshots by default; resource destroy
deletes them only if ´delete_snapshots_on_destroy´ is ´true´.

## Import
The resource ID is ´<tenant_domain>/<management_group_id>´. Import adopts the group's existing subscriptions and their
supported features. Only import a non-OAuth group whose subscriptions all have the same connected supported features and settings.
`

const azureManagementGroupOperationTimeout = 45 * time.Minute

var (
	_ resource.Resource                = &azureManagementGroupResource{}
	_ resource.ResourceWithConfigure   = &azureManagementGroupResource{}
	_ resource.ResourceWithIdentity    = &azureManagementGroupResource{}
	_ resource.ResourceWithImportState = &azureManagementGroupResource{}
	_ resource.ResourceWithModifyPlan  = &azureManagementGroupResource{}
)

type azureManagementGroupResource struct {
	client *client
	prefix string
}

// azureManagementGroupModel is the resource state. Field order follows the
// schema.
type azureManagementGroupModel struct {
	ID                          types.String `tfsdk:"id"`
	TenantDomain                types.String `tfsdk:"tenant_domain"`
	AppID                       types.String `tfsdk:"app_id"`
	ManagementGroupID           types.String `tfsdk:"management_group_id"`
	ManagementGroupName         types.String `tfsdk:"management_group_name"`
	CustomerManagementGroupID   types.String `tfsdk:"customer_management_group_id"`
	Regions                     types.Set    `tfsdk:"regions"`
	SubscriptionIDs             types.Set    `tfsdk:"subscription_ids"`
	DeleteSnapshotsOnDestroy    types.Bool   `tfsdk:"delete_snapshots_on_destroy"`
	Feature                     types.Set    `tfsdk:"feature"`
	ManagedSubscriptionFeatures types.Map    `tfsdk:"managed_subscription_features"`
	OnboardingTaskID            types.String `tfsdk:"onboarding_task_id"`
}

// azureManagementGroupFeatureModel is the model of a feature entry.
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

// azureManagementGroupFeatureAttrTypes are the attribute types of a feature entry.
var azureManagementGroupFeatureAttrTypes = map[string]attr.Type{
	keyName:                types.StringType,
	keyPermissionGroups:    types.SetType{ElemType: types.StringType},
	keyResourceGroupName:   types.StringType,
	keyResourceGroupRegion: types.StringType,
	keyResourceGroupTags:   types.MapType{ElemType: types.StringType},
}

func newAzureManagementGroupResource() resource.Resource {
	return &azureManagementGroupResource{prefix: keyRubrik}
}

func newPolarisAzureManagementGroupResource() resource.Resource {
	return &azureManagementGroupResource{prefix: keyPolaris}
}

func (r *azureManagementGroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "azureManagementGroupResource.Metadata")

	res.TypeName = r.prefix + "_" + keyAzureManagementGroup
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
					"style, e.g. `eastus`. Regions can be added to an existing resource with connected Cloud Discovery, " +
					"but cannot be removed.",
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
			keyManagedSubscriptionFeatures: schema.MapAttribute{
				Computed:    true,
				ElementType: types.SetType{ElemType: types.StringType},
				Description: "Feature names managed by this resource, keyed by Azure subscription ID.",
			},
			keyOnboardingTaskID: schema.StringAttribute{
				Computed:    true,
				Description: "Accepted onboarding task that has not yet been cleared from state.",
			},
			keyFeature: schema.SetNestedAttribute{
				Required:    true,
				Description: "RSC features to enable on all the subscriptions. Include `CLOUD_DISCOVERY` with the `BASIC` permission group.",
				Validators: []validator.Set{
					azureManagementGroupDiscoveryValidator{},
				},
				NestedObject: schema.NestedAttributeObject{
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
								"Required for `CLOUD_NATIVE_ARCHIVAL` and may be required for other features, such as " +
								"`CLOUD_NATIVE_PROTECTION`, depending on RSC configuration.",
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
					Validators: []validator.Object{
						featurePermissionGroupsValidator(core.AzureSubscriptionFeatures()),
					},
				},
			},
		},
	}
	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_azure_management_group` resource instead."
	}
}

type azureManagementGroupDiscoveryValidator struct{}

func (azureManagementGroupDiscoveryValidator) Description(_ context.Context) string {
	return "Include CLOUD_DISCOVERY with the BASIC permission group."
}

func (v azureManagementGroupDiscoveryValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v azureManagementGroupDiscoveryValidator) ValidateSet(ctx context.Context, req validator.SetRequest, res *validator.SetResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	for _, value := range req.ConfigValue.Elements() {
		feature, ok := value.(types.Object)
		if !ok || feature.IsNull() {
			continue
		}
		if feature.IsUnknown() {
			return
		}
		name, ok := feature.Attributes()[keyName].(types.String)
		if ok && (name.IsUnknown() || name.ValueString() == core.CloudDiscovery) {
			return
		}
	}
	res.Diagnostics.AddAttributeError(req.Path, "Cloud Discovery is required", v.Description(ctx))
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
	ctx, cancel := context.WithTimeout(ctx, azureManagementGroupOperationTimeout)
	defer cancel()
	data, err := readAzureManagementGroup(ctx, api, tenantDomain, managementGroupID)
	if err != nil && !errors.Is(err, graphql.ErrNotFound) {
		res.Diagnostics.AddError("Failed to check existing subscriptions", err.Error())
		return
	}
	wanted, _, err := azureManagementGroupReconcile(data.accounts, nil, input)
	if err != nil {
		res.Diagnostics.AddError("Cannot onboard subscriptions", err.Error())
		return
	}
	taskID, err := api.StartManagementGroupOnboarding(ctx, azure.StartManagementGroupOnboardingParams{
		TenantDomain: tenantDomain, AppID: input.appID, ManagementGroupID: managementGroupID,
		Regions: input.regions, Subscriptions: wanted,
	})
	if err != nil {
		res.Diagnostics.AddError("Failed to submit management group onboarding", err.Error())
		return
	}

	// Persist accepted work before a waiter or read-back can fail.
	plan.ID = types.StringValue(tenantDomain + "/" + managementGroupID)
	plan.ManagementGroupName = types.StringNull()
	plan.CustomerManagementGroupID = types.StringNull()
	plan.OnboardingTaskID = types.StringValue(taskID.String())
	res.Diagnostics.Append(plan.setManagedFeatures(ctx, input.managedFeatures())...)
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	res.Diagnostics.Append(res.Identity.Set(ctx, azureManagementGroupIdentityModel{ID: plan.ID})...)
	if res.Diagnostics.HasError() {
		return
	}
	if err := api.WaitForManagementGroupOnboarding(ctx, azure.WaitForManagementGroupOnboardingParams{
		TaskChainID: taskID, TenantDomain: tenantDomain, ManagementGroupID: managementGroupID, Subscriptions: wanted,
	}); err != nil {
		res.Diagnostics.AddError("Failed to complete management group onboarding", err.Error())
		return
	}
	plan.OnboardingTaskID = types.StringNull()
	data, err = readAzureManagementGroup(ctx, api, tenantDomain, managementGroupID)
	if err != nil {
		res.Diagnostics.AddWarning("Failed to read management group after create", err.Error())
	} else {
		plan.ManagementGroupName = types.StringValue(data.group.Name)
		plan.CustomerManagementGroupID = types.StringValue(data.group.CustomerID.String())
	}
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
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

	if !state.OnboardingTaskID.IsNull() {
		taskID, err := uuid.Parse(state.OnboardingTaskID.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Invalid onboarding task ID", err.Error())
			return
		}
		task, err := core.Wrap(polarisClient.GQL).KorgTaskChainStatus(ctx, taskID)
		if err != nil {
			res.Diagnostics.AddError("Cannot refresh pending onboarding task", err.Error())
			return
		}
		switch task.State {
		case core.TaskChainSucceeded, core.TaskChainFailed, core.TaskChainCanceled:
			state.OnboardingTaskID = types.StringNull()
		default:
			// Keep the accepted job and ownership while it can still create rows.
			return
		}
	}
	data, err := readAzureManagementGroup(ctx, azure.Wrap(polarisClient), tenantDomain, managementGroupID)
	if errors.Is(err, graphql.ErrNotFound) && state.ManagedSubscriptionFeatures.IsNull() {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil && !errors.Is(err, graphql.ErrNotFound) {
		res.Diagnostics.AddError("Failed to read management group", err.Error())
		return
	}
	if state.ManagedSubscriptionFeatures.IsNull() {
		// Import explicitly adopts the group's existing subscriptions.
		appID, err := azure.Wrap(polarisClient).ManagementGroupCustomerAppID(ctx, tenantDomain)
		if err != nil {
			res.Diagnostics.AddError("Failed to read management group's non-OAuth customer app", err.Error())
			return
		}
		data.appID = appID
		res.Diagnostics.Append(data.setModel(ctx, &state, tenantDomain, managementGroupID)...)
	} else {
		input, diags := azureManagementGroupInputFromModel(ctx, state)
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return
		}
		// A missing pair makes its subscription drift, while the separate
		// ownership map still protects it during repair or destroy.
		ids := azureManagementGroupPresentSubscriptions(data.accounts, input.managed, state.SubscriptionIDs.Elements())
		values, diags := types.SetValueFrom(ctx, types.StringType, ids)
		res.Diagnostics.Append(diags...)
		state.SubscriptionIDs = values
		if err == nil {
			state.ManagementGroupName = types.StringValue(data.group.Name)
			state.CustomerManagementGroupID = types.StringValue(data.group.CustomerID.String())
		}
	}
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

	if err := azureManagementGroupValidateUpdate(oldInput, newInput); err != nil {
		res.Diagnostics.AddError("Unsupported management group change", err.Error())
		return
	}
	if !state.OnboardingTaskID.IsNull() {
		res.Diagnostics.AddError("Management group onboarding still pending", "Refresh after the accepted onboarding task finishes before updating.")
		return
	}
	ctx, cancel := context.WithTimeout(ctx, azureManagementGroupOperationTimeout)
	defer cancel()
	tenantDomain, managementGroupID := plan.TenantDomain.ValueString(), plan.ManagementGroupID.ValueString()
	data, err := readAzureManagementGroup(ctx, api, tenantDomain, managementGroupID)
	if err != nil && !errors.Is(err, graphql.ErrNotFound) {
		res.Diagnostics.AddError("Failed to read management group", err.Error())
		return
	}
	adds, removes, err := azureManagementGroupReconcile(data.accounts, oldInput.managed, newInput)
	if err != nil {
		res.Diagnostics.AddError("Cannot reconcile management group", err.Error())
		return
	}
	if !state.Regions.Equal(plan.Regions) {
		if !state.SubscriptionIDs.Equal(plan.SubscriptionIDs) || !state.Feature.Equal(plan.Feature) || len(adds) != 0 || len(removes) != 0 {
			res.Diagnostics.AddError("Apply region additions separately", "Do not change subscriptions or features in the same apply as a region addition. Repair any missing features before adding regions.")
			return
		}
		if err := azureManagementGroupAddRegions(ctx, gqlazure.Wrap(polarisClient.GQL), tenantDomain, managementGroupID,
			oldInput.managed, newInput.subscriptions, newInput.regions); err != nil {
			res.Diagnostics.AddError("Failed to add management group regions", err.Error())
			return
		}
	}
	if len(adds) != 0 {
		taskID, err := api.StartManagementGroupOnboarding(ctx, azure.StartManagementGroupOnboardingParams{
			TenantDomain: tenantDomain, AppID: newInput.appID, ManagementGroupID: managementGroupID,
			Regions: newInput.regions, Subscriptions: adds,
		})
		if err != nil {
			res.Diagnostics.AddError("Failed to submit management group onboarding", err.Error())
			return
		}
		// Journal only accepted pairs; never widen to the cross-product of
		// old and new subscription and feature sets.
		for _, sub := range adds {
			id := sub.NativeID.String()
			for _, feature := range sub.Features {
				if !slices.Contains(oldInput.managed[id], feature.Feature.Name) {
					oldInput.managed[id] = append(oldInput.managed[id], feature.Feature.Name)
				}
			}
		}
		state.OnboardingTaskID = types.StringValue(taskID.String())
		res.Diagnostics.Append(state.setManagedFeatures(ctx, oldInput.managed)...)
		res.Diagnostics.Append(res.State.Set(ctx, &state)...)
		if res.Diagnostics.HasError() {
			return
		}
		if err := api.WaitForManagementGroupOnboarding(ctx, azure.WaitForManagementGroupOnboardingParams{
			TaskChainID: taskID, TenantDomain: tenantDomain, ManagementGroupID: managementGroupID, Subscriptions: adds,
		}); err != nil {
			res.Diagnostics.AddError("Failed to complete management group onboarding", err.Error())
			return
		}
		state.OnboardingTaskID = types.StringNull()
		res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	}
	if err := removeAzureManagementGroupFeatures(ctx, api, tenantDomain, newInput.appID, removes, false); err != nil {
		res.Diagnostics.AddError("Failed to remove management group features", err.Error())
		return
	}
	plan.OnboardingTaskID = types.StringNull()
	res.Diagnostics.Append(plan.setManagedFeatures(ctx, newInput.managedFeatures())...)
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

	ctx, cancel := context.WithTimeout(ctx, azureManagementGroupOperationTimeout)
	defer cancel()
	if !state.OnboardingTaskID.IsNull() {
		taskID, err := uuid.Parse(state.OnboardingTaskID.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Invalid onboarding task ID", err.Error())
			return
		}
		// Failed/canceled onboarding can still leave rows to delete.
		if _, err := core.Wrap(polarisClient.GQL).WaitForTaskChain(ctx, taskID, 10*time.Second); err != nil {
			res.Diagnostics.AddError("Cannot delete while onboarding may still be running", err.Error())
			return
		}
	}
	data, err := readAzureManagementGroup(ctx, api, state.TenantDomain.ValueString(), state.ManagementGroupID.ValueString())
	if errors.Is(err, graphql.ErrNotFound) {
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read management group", err.Error())
		return
	}
	_, removes, err := azureManagementGroupReconcile(data.accounts, input.managed, azureManagementGroupInput{})
	if err != nil {
		res.Diagnostics.AddError("Cannot determine managed subscription features", err.Error())
		return
	}
	err = removeAzureManagementGroupFeatures(ctx, api, state.TenantDomain.ValueString(), input.appID, removes,
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

// azureManagementGroupFeature is a feature entry in plain Go types.
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
	managed       map[string][]string
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
	input.managed = make(map[string][]string)
	if !model.ManagedSubscriptionFeatures.IsNull() && !model.ManagedSubscriptionFeatures.IsUnknown() {
		diags.Append(model.ManagedSubscriptionFeatures.ElementsAs(ctx, &input.managed, false)...)
	}

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

	featureNames := make(map[string]bool)
	var featureModels []azureManagementGroupFeatureModel
	diags.Append(model.Feature.ElementsAs(ctx, &featureModels, false)...)
	for _, featureModel := range featureModels {
		feature := azureManagementGroupFeature{
			name:                featureModel.Name.ValueString(),
			resourceGroupName:   featureModel.ResourceGroupName.ValueString(),
			resourceGroupRegion: featureModel.ResourceGroupRegion.ValueString(),
		}
		if featureNames[feature.name] {
			diags.AddError("Duplicate management group feature", fmt.Sprintf("Feature %s must appear only once.", feature.name))
		}
		featureNames[feature.name] = true
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

// azureManagementGroupValidateUpdate rejects changes with no public in-place API.
func azureManagementGroupValidateUpdate(oldInput, newInput azureManagementGroupInput) error {
	if slices.ContainsFunc(oldInput.regions, func(region regions.Region) bool {
		return !slices.Contains(newInput.regions, region)
	}) {
		return errors.New("removing regions is not supported")
	}
	for _, feature := range newInput.features {
		idx := slices.IndexFunc(oldInput.features, func(f azureManagementGroupFeature) bool { return f.name == feature.name })
		if idx < 0 {
			continue
		}
		old := oldInput.features[idx]
		if !slices.Equal(old.permissionGroups, feature.permissionGroups) || old.resourceGroupName != feature.resourceGroupName ||
			old.resourceGroupRegion != feature.resourceGroupRegion || !maps.Equal(old.resourceGroupTags, feature.resourceGroupTags) {
			return fmt.Errorf("changing the permission groups or the resource group of existing feature %s is not supported", feature.name)
		}
	}
	return nil
}

// azureManagementGroupPresentSubscriptions returns only owned subscriptions
// with all owned features connected. Missing pairs produce a Terraform diff
// without causing refresh to forget which remote pairs it is allowed to delete.
func azureManagementGroupPresentSubscriptions(accounts []azure.CloudAccount, owned map[string][]string, previous []attr.Value) []string {
	spelling := make(map[string]string, len(previous))
	for _, value := range previous {
		if str, ok := value.(types.String); ok {
			spelling[strings.ToLower(str.ValueString())] = str.ValueString()
		}
	}
	ids := make([]string, 0, len(owned))
	for _, account := range accounts {
		id := account.NativeID.String()
		names := owned[id]
		if len(names) == 0 || slices.ContainsFunc(names, func(name string) bool {
			feature, exists := account.Feature(core.Feature{Name: name})
			return !exists || feature.Status != core.StatusConnected
		}) {
			continue
		}
		if value := spelling[id]; value != "" {
			id = value
		}
		ids = append(ids, id)
	}
	return ids
}

// managedFeatures is the desired ownership matrix for this configuration.
func (input azureManagementGroupInput) managedFeatures() map[string][]string {
	result := make(map[string][]string, len(input.subscriptions))
	for _, id := range input.subscriptions {
		for _, feature := range input.features {
			result[id.String()] = append(result[id.String()], feature.name)
		}
	}
	return result
}

func (model *azureManagementGroupModel) setManagedFeatures(ctx context.Context, pairs map[string][]string) diag.Diagnostics {
	value, diags := types.MapValueFrom(ctx, types.SetType{ElemType: types.StringType}, pairs)
	model.ManagedSubscriptionFeatures = value
	return diags
}

// azureManagementGroupReconcile only acts on exact owned or explicitly desired pairs.
// It does not claim an existing feature merely because it shares the same group.
func azureManagementGroupReconcile(accounts []azure.CloudAccount, owned map[string][]string, desired azureManagementGroupInput) ([]azure.ManagementGroupSubscription, map[string][]uuid.UUID, error) {
	// The configured feature name may not have been known during validation.
	if len(desired.subscriptions) != 0 && !slices.ContainsFunc(desired.features, func(feature azureManagementGroupFeature) bool {
		return feature.name == core.CloudDiscovery && slices.Equal(feature.permissionGroups, []string{string(core.PermissionGroupBasic)})
	}) {
		return nil, nil, errors.New("include CLOUD_DISCOVERY with the BASIC permission group")
	}
	byID := make(map[uuid.UUID]azure.CloudAccount, len(accounts))
	for _, account := range accounts {
		byID[account.NativeID] = account
	}
	var adds []azure.ManagementGroupSubscription
	desiredPairs := desired.managedFeatures()
	for _, sub := range azureManagementGroupSubscriptions(desired.subscriptions, desired.features) {
		var missing []azure.ManagementGroupFeature
		for _, feature := range sub.Features {
			actual, exists := byID[sub.NativeID].Feature(feature.Feature)
			if !exists {
				missing = append(missing, feature)
			} else if !slices.Contains(owned[sub.NativeID.String()], feature.Feature.Name) {
				return nil, nil, fmt.Errorf("subscription %s feature %s already exists and is not managed by this resource; import the group or choose a different subscription", sub.NativeID, feature.Feature.Name)
			} else if actual.Status != core.StatusConnected {
				return nil, nil, fmt.Errorf("subscription %s feature %s exists with status %s; repair it in RSC before retrying", sub.NativeID, feature.Feature.Name, actual.Status)
			}
		}
		if len(missing) != 0 {
			adds = append(adds, azure.ManagementGroupSubscription{NativeID: sub.NativeID, Features: missing})
		}
	}
	removes := make(map[string][]uuid.UUID)
	for _, account := range accounts {
		id := account.NativeID.String()
		shouldRemove := func(feature azure.Feature) bool {
			return slices.Contains(owned[id], feature.Name) && !slices.Contains(desiredPairs[id], feature.Name)
		}
		for _, feature := range account.Features {
			if !shouldRemove(feature) {
				continue
			}
			if account.AuthType == gqlazure.AuthTypeOAuthRubrikApp || account.AuthType == gqlazure.AuthTypeOAuthCustomerApp {
				return nil, nil, fmt.Errorf("cannot remove feature %s from OAuth subscription %s with this non-OAuth resource", feature.Name, id)
			}
			if feature.Equal(core.FeatureCloudDiscovery) && slices.ContainsFunc(account.Features, func(other azure.Feature) bool {
				return !other.Equal(core.FeatureCloudDiscovery) && other.IsProtectionFeature() && !shouldRemove(other)
			}) {
				return nil, nil, fmt.Errorf("cannot remove Cloud Discovery from subscription %s while a retained or unmanaged protection feature depends on it", id)
			}
			if feature.Equal(core.FeatureCloudNativeArchival) {
				if encryption, exists := account.Feature(core.FeatureCloudNativeArchivalEncryption); exists && !shouldRemove(encryption) {
					return nil, nil, fmt.Errorf("cannot remove Cloud Native Archival from subscription %s while a retained or unmanaged Archival Encryption feature depends on it", id)
				}
			}
			removes[feature.Name] = append(removes[feature.Name], account.ID)
		}
	}
	return adds, removes, nil
}

// removeAzureManagementGroupFeatures removes dependent features before their parents.
func removeAzureManagementGroupFeatures(ctx context.Context, api azure.API, tenantDomain string, appID uuid.UUID,
	removes map[string][]uuid.UUID, deleteSnapshots bool) error {
	names := slices.Sorted(maps.Keys(removes))
	// RSC also removes Encryption when its Archival parent is removed.
	// Delete it first so we do not issue a second delete against a missing row.
	if parent, child := slices.Index(names, core.CloudNativeArchival), slices.Index(names, core.CloudNativeArchivalEncryption); parent >= 0 && child > parent {
		names[parent], names[child] = names[child], names[parent]
	}
	if idx := slices.Index(names, core.FeatureCloudDiscovery.Name); idx >= 0 {
		names = append(slices.Delete(names, idx, idx+1), core.FeatureCloudDiscovery.Name)
	}
	for _, name := range names {
		if err := api.RemoveManagementGroupSubscriptionFeature(ctx, azure.RemoveManagementGroupSubscriptionFeatureParams{
			TenantDomain: tenantDomain, AppID: appID, CloudAccountIDs: removes[name], Feature: core.Feature{Name: name},
			DeleteSnapshots: deleteSnapshots && name == core.FeatureCloudNativeProtection.Name,
		}); err != nil {
			return err
		}
	}
	return nil
}

// azureManagementGroupData is the management group as read from RSC.
type azureManagementGroupData struct {
	appID    uuid.UUID
	group    azure.ManagementGroup
	accounts []azure.CloudAccount
}

// readAzureManagementGroup reads all observed pairs in the group. Disabled
// rows are retained because disabling alone does not complete Azure cleanup.
func readAzureManagementGroup(ctx context.Context, api azure.API, tenantDomain, managementGroupID string) (azureManagementGroupData, error) {
	accounts, err := api.Subscriptions(ctx, "")
	if err != nil {
		return azureManagementGroupData{}, err
	}

	var data azureManagementGroupData
	for _, account := range accounts {
		mg := account.ManagementGroup
		if mg == nil || len(account.Features) == 0 || !strings.EqualFold(mg.NativeID, managementGroupID) || !strings.EqualFold(account.TenantDomain, tenantDomain) {
			continue
		}

		data.group = *mg
		data.accounts = append(data.accounts, account)
	}
	if len(data.accounts) == 0 {
		return azureManagementGroupData{}, fmt.Errorf("management group %s: %w", managementGroupID, graphql.ErrNotFound)
	}

	return data, nil
}

// setModel reconstructs the configuration and ownership for an explicit import.
func (d azureManagementGroupData) setModel(ctx context.Context, model *azureManagementGroupModel, tenantDomain, managementGroupID string) diag.Diagnostics {
	var diags diag.Diagnostics

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

	pairs := make(map[string][]string, len(d.accounts))
	var expected []string
	for _, account := range d.accounts {
		var names []string
		if account.AuthType == gqlazure.AuthTypeOAuthRubrikApp || account.AuthType == gqlazure.AuthTypeOAuthCustomerApp {
			diags.AddError("Cannot import management group", fmt.Sprintf("Subscription %s is onboarded with OAuth.", account.NativeID))
			return diags
		}
		for _, feature := range account.Features {
			if feature.Status != core.StatusConnected {
				diags.AddError("Cannot import management group", fmt.Sprintf("Subscription %s feature %s is not connected.", account.NativeID, feature.Name))
				return diags
			}
			names = append(names, feature.Name)
		}
		slices.Sort(names)
		if expected != nil && !slices.Equal(expected, names) {
			diags.AddError("Cannot import management group", "Subscriptions must have identical feature sets before import.")
			return diags
		}
		expected = names
		pairs[account.NativeID.String()] = names
	}
	diags.Append(model.setManagedFeatures(ctx, pairs)...)

	subscriptionIDs := make([]attr.Value, 0, len(d.accounts))
	regionSet := make(map[string]struct{})
	featureByName := make(map[string]azure.Feature)
	for _, account := range d.accounts {
		subscriptionIDs = append(subscriptionIDs, types.StringValue(account.NativeID.String()))
		for _, feature := range account.Features {
			if previous, ok := featureByName[feature.Name]; ok {
				// One Terraform feature entry applies to every subscription. Do not
				// import settings from only the first subscription if they differ.
				if !previous.Feature.DeepEqual(feature.Feature) ||
					!slices.Equal(slices.Sorted(slices.Values(previous.Regions)), slices.Sorted(slices.Values(feature.Regions))) ||
					(feature.SupportResourceGroup() && (previous.ResourceGroup.Name != feature.ResourceGroup.Name ||
						previous.ResourceGroup.Region != feature.ResourceGroup.Region || !maps.Equal(previous.ResourceGroup.Tags, feature.ResourceGroup.Tags))) {
					diags.AddError("Cannot import management group", fmt.Sprintf("Subscriptions have different settings for feature %s.", feature.Name))
					return diags
				}
			} else {
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
		if feature.SupportResourceGroup() && feature.ResourceGroup.Name != "" {
			featureModel.ResourceGroupName = types.StringValue(feature.ResourceGroup.Name)
			featureModel.ResourceGroupRegion = types.StringValue(feature.ResourceGroup.Region)
			if len(feature.ResourceGroup.Tags) > 0 {
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
