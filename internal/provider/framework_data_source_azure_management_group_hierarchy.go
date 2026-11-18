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
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/azure"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

const dataSourceAzureManagementGroupHierarchyDescription = `
The ´rubrik_azure_management_group_hierarchy´ data source lists the subscriptions and management groups under an Azure
management group, and whether each subscription is eligible for onboarding through the management group. Use it to
compute the ´subscription_ids´ of the ´rubrik_azure_management_group´ resource.

RSC signs in as the customer app registered with the ´rubrik_azure_service_principal´ resource. The app's service
principal must have the ´Contributor´ and ´User Access Administrator´ roles at the management group scope.

~> **Note:** Use explicit subscription IDs for onboarding. ´eligible_subscription_ids´ can shrink after onboarding or
when permissions change. ´all_subscription_ids´ includes subscriptions that cannot be onboarded through this group.

~> **Note:** The RSC feature flag ´AZURE_CLOUDACCOUNTS_MANAGEMENT_GROUP_ENABLED´ must be enabled for the RSC account.
`

var _ datasource.DataSource = &azureManagementGroupHierarchyDataSource{}

type azureManagementGroupHierarchyDataSource struct {
	client *client
}

// azureManagementGroupHierarchyDataSourceModel is the data source state.
type azureManagementGroupHierarchyDataSourceModel struct {
	ID                       types.String `tfsdk:"id"`
	TenantDomain             types.String `tfsdk:"tenant_domain"`
	AppID                    types.String `tfsdk:"app_id"`
	ManagementGroupID        types.String `tfsdk:"management_group_id"`
	ScopedManagementGroupIDs types.Set    `tfsdk:"scoped_management_group_ids"`
	Recurse                  types.Bool   `tfsdk:"recurse"`
	Features                 types.Set    `tfsdk:"features"`
	Subscriptions            types.List   `tfsdk:"subscriptions"`
	ManagementGroups         types.List   `tfsdk:"management_groups"`
	AllSubscriptionIDs       types.Set    `tfsdk:"all_subscription_ids"`
	EligibleSubscriptionIDs  types.Set    `tfsdk:"eligible_subscription_ids"`
}

var azureManagementGroupHierarchySubscriptionAttrTypes = map[string]attr.Type{
	keyNativeID:            types.StringType,
	keyName:                types.StringType,
	keyEligible:            types.BoolType,
	keyIneligibilityReason: types.StringType,
}

var azureManagementGroupHierarchyGroupAttrTypes = map[string]attr.Type{
	keyNativeID: types.StringType,
	keyName:     types.StringType,
}

func newAzureManagementGroupHierarchyDataSource() datasource.DataSource {
	return &azureManagementGroupHierarchyDataSource{}
}

func (d *azureManagementGroupHierarchyDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "azureManagementGroupHierarchyDataSource.Metadata")

	res.TypeName = keyRubrik + "_" + keyAzureManagementGroupHierarchy
}

func (d *azureManagementGroupHierarchyDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "azureManagementGroupHierarchyDataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSourceAzureManagementGroupHierarchyDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "Data source ID, in the format `<tenant_domain>/<management_group_id>`.",
			},
			keyTenantDomain: schema.StringAttribute{
				Required:    true,
				Description: "Azure tenant primary domain.",
				Validators:  []validator.String{isNotWhiteSpace()},
			},
			keyAppID: schema.StringAttribute{
				Required:    true,
				Description: "Azure app registration application ID (UUID) of the customer app.",
				Validators:  []validator.String{isUUID()},
			},
			keyManagementGroupID: schema.StringAttribute{
				Required:    true,
				Description: "Azure management group ID.",
				Validators:  []validator.String{isNotWhiteSpace()},
			},
			keyScopedManagementGroupIDs: schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Child management group IDs to limit the hierarchy to. They must be descendants of " +
					"`management_group_id`.",
			},
			keyRecurse: schema.BoolAttribute{
				Optional: true,
				Description: "Include the child management groups recursively. When `false`, only the immediate " +
					"children are included. Default value is `true`.",
			},
			keyFeatures: schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "RSC features to check the eligibility of the subscriptions for.",
				Validators: []validator.Set{
					setvalidator.ValueStringsAre(stringvalidator.OneOf(core.AzureSubscriptionFeatureNames()...)),
				},
			},
			keySubscriptions: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Subscriptions in the hierarchy.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyNativeID: schema.StringAttribute{
							Computed:    true,
							Description: "Azure subscription ID (UUID).",
						},
						keyName: schema.StringAttribute{
							Computed:    true,
							Description: "Azure subscription name.",
						},
						keyEligible: schema.BoolAttribute{
							Computed:    true,
							Description: "Whether the subscription can be onboarded through the management group.",
						},
						keyIneligibilityReason: schema.StringAttribute{
							Computed: true,
							Description: "Why the subscription cannot be onboarded, or `AZURE_ONBOARDING_INELIGIBILITY_REASON_UNSPECIFIED` " +
								"when it is eligible.",
						},
					},
				},
			},
			keyManagementGroups: schema.ListNestedAttribute{
				Computed:    true,
				Description: "Management groups in the hierarchy.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyNativeID: schema.StringAttribute{
							Computed:    true,
							Description: "Azure management group ID.",
						},
						keyName: schema.StringAttribute{
							Computed:    true,
							Description: "Azure management group display name.",
						},
					},
				},
			},
			keyAllSubscriptionIDs: schema.SetAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Azure IDs of all subscriptions in the hierarchy, eligible or not.",
			},
			keyEligibleSubscriptionIDs: schema.SetAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Azure IDs of the subscriptions in the hierarchy that are eligible for onboarding.",
			},
		},
	}
}

func (d *azureManagementGroupHierarchyDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "azureManagementGroupHierarchyDataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

func (d *azureManagementGroupHierarchyDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "azureManagementGroupHierarchyDataSource.Read")

	var config azureManagementGroupHierarchyDataSourceModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := d.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	appID, err := uuid.Parse(config.AppID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid app ID", err.Error())
		return
	}

	opts := azure.ManagementGroupHierarchyOptions{NoRecurse: !config.Recurse.IsNull() && !config.Recurse.ValueBool()}
	res.Diagnostics.Append(config.ScopedManagementGroupIDs.ElementsAs(ctx, &opts.ScopedManagementGroupIDs, false)...)
	var featureNames []string
	res.Diagnostics.Append(config.Features.ElementsAs(ctx, &featureNames, false)...)
	if res.Diagnostics.HasError() {
		return
	}
	for _, name := range featureNames {
		opts.Features = append(opts.Features, core.Feature{Name: name})
	}

	hierarchy, err := azure.Wrap(polarisClient).ManagementGroupHierarchy(ctx, config.TenantDomain.ValueString(), appID,
		config.ManagementGroupID.ValueString(), opts)
	if err != nil {
		res.Diagnostics.AddError("Failed to read management group hierarchy", err.Error())
		return
	}

	subscriptions := make([]attr.Value, 0, len(hierarchy.Subscriptions))
	allIDs := make([]attr.Value, 0, len(hierarchy.Subscriptions))
	var eligibleIDs []attr.Value
	for _, sub := range hierarchy.Subscriptions {
		id := types.StringValue(strings.ToLower(sub.NativeID.String()))
		allIDs = append(allIDs, id)
		if sub.IsEligible {
			eligibleIDs = append(eligibleIDs, id)
		}
		subscriptions = append(subscriptions, types.ObjectValueMust(azureManagementGroupHierarchySubscriptionAttrTypes, map[string]attr.Value{
			keyNativeID:            id,
			keyName:                types.StringValue(sub.Name),
			keyEligible:            types.BoolValue(sub.IsEligible),
			keyIneligibilityReason: types.StringValue(sub.IneligibilityReason),
		}))
	}
	groups := make([]attr.Value, 0, len(hierarchy.ManagementGroups))
	for _, group := range hierarchy.ManagementGroups {
		groups = append(groups, types.ObjectValueMust(azureManagementGroupHierarchyGroupAttrTypes, map[string]attr.Value{
			keyNativeID: types.StringValue(group.NativeID),
			keyName:     types.StringValue(group.Name),
		}))
	}

	config.ID = types.StringValue(config.TenantDomain.ValueString() + "/" + config.ManagementGroupID.ValueString())
	config.Subscriptions = types.ListValueMust(types.ObjectType{AttrTypes: azureManagementGroupHierarchySubscriptionAttrTypes}, subscriptions)
	config.ManagementGroups = types.ListValueMust(types.ObjectType{AttrTypes: azureManagementGroupHierarchyGroupAttrTypes}, groups)
	config.AllSubscriptionIDs = types.SetValueMust(types.StringType, allIDs)
	config.EligibleSubscriptionIDs = types.SetValueMust(types.StringType, eligibleIDs)
	res.Diagnostics.Append(res.State.Set(ctx, &config)...)
}
