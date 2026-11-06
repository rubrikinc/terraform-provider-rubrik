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

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/azure"
)

const dataSourceAzureSubscriptionDescription = `
The ´rubrik_azure_subscription´ data source is used to access information
about an Azure subscription added to RSC. An Azure subscription is looked up
using either the Azure subscription ID, the RSC cloud account ID, or the name.
When looking up an Azure subscription using the subscription name, the tenant
domain can be used to specify in which tenant to look for the name.

The ´feature´ attribute exposes the RSC features enabled for the subscription,
including each feature's permission groups, regions, status, resource group and
user-assigned managed identity.

-> **Note:** The subscription name is the name of the Azure subscription as it
   appears in RSC.
`

var _ datasource.DataSource = &azureSubscriptionDataSource{}

type azureSubscriptionDataSource struct {
	client *client
	prefix string
}

type azureSubscriptionModel struct {
	ID             types.String `tfsdk:"id"`
	Feature        types.Set    `tfsdk:"feature"`
	Name           types.String `tfsdk:"name"`
	SubscriptionID types.String `tfsdk:"subscription_id"`
	TenantDomain   types.String `tfsdk:"tenant_domain"`
}

func newAzureSubscriptionDataSource() datasource.DataSource {
	return &azureSubscriptionDataSource{prefix: keyRubrik}
}

func newPolarisAzureSubscriptionDataSource() datasource.DataSource {
	return &azureSubscriptionDataSource{prefix: keyPolaris}
}

func (d *azureSubscriptionDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "azureSubscriptionDataSource.Metadata")

	res.TypeName = d.prefix + "_" + keyAzureSubscription
}

func (d *azureSubscriptionDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "azureSubscriptionDataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSourceAzureSubscriptionDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "RSC cloud account ID (UUID).",
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.MatchRoot(keyName), path.MatchRoot(keySubscriptionID)),
				},
			},
			keyFeature: schema.SetNestedAttribute{
				Computed:    true,
				Description: "RSC features enabled for the subscription.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyName: schema.StringAttribute{
							Computed:    true,
							Description: "RSC feature name.",
						},
						keyPermissionGroups: schema.SetAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: "Permission groups enabled for the feature.",
						},
						keyRegions: schema.SetAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: "Azure regions where the feature is enabled.",
						},
						keyStatus: schema.StringAttribute{
							Computed:    true,
							Description: "Status of the feature.",
						},
						keyResourceGroupName: schema.StringAttribute{
							Computed:    true,
							Description: "Name of the Azure resource group used by the feature, if any.",
						},
						keyResourceGroupRegion: schema.StringAttribute{
							Computed:    true,
							Description: "Region of the Azure resource group used by the feature, if any.",
						},
						keyResourceGroupTags: schema.MapAttribute{
							ElementType: types.StringType,
							Computed:    true,
							Description: "Tags of the Azure resource group used by the feature, if any.",
						},
						keyUserAssignedManagedIdentityName: schema.StringAttribute{
							Computed:    true,
							Description: "Name of the user-assigned managed identity used by the feature, if any.",
						},
						keyUserAssignedManagedIdentityPrincipalID: schema.StringAttribute{
							Computed: true,
							Description: "Principal ID of the user-assigned managed identity used by the feature, if " +
								"any. This is the same ID used as `principal_id` in Azure RBAC role assignments.",
						},
					},
				},
			},
			keyName: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Azure subscription name.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			keySubscriptionID: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Azure subscription ID.",
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyTenantDomain: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Azure tenant primary domain.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
		},
	}

	if d.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_azure_subscription` data source instead."
	}
}

func (d *azureSubscriptionDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "azureSubscriptionDataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

func (d *azureSubscriptionDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "azureSubscriptionDataSource.Read")

	var config azureSubscriptionModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := d.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	// We don't allow prefix searches since it would be impossible to uniquely
	// identify a subscription with a name being the prefix of another
	// subscription.
	var subscription azure.CloudAccount
	switch {
	case config.SubscriptionID.ValueString() != "":
		id, err := uuid.Parse(config.SubscriptionID.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Invalid subscription ID", err.Error())
			return
		}
		subscription, err = azure.Wrap(polarisClient).SubscriptionByNativeID(ctx, id)
		if err != nil {
			res.Diagnostics.AddError("Failed to read Azure subscription", err.Error())
			return
		}
	case config.Name.ValueString() != "":
		subscription, err = azure.Wrap(polarisClient).SubscriptionByName(ctx, config.Name.ValueString(),
			config.TenantDomain.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Failed to read Azure subscription", err.Error())
			return
		}
	default:
		id, err := uuid.Parse(config.ID.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Invalid ID", err.Error())
			return
		}
		subscription, err = azure.Wrap(polarisClient).SubscriptionByID(ctx, id)
		if err != nil {
			res.Diagnostics.AddError("Failed to read Azure subscription", err.Error())
			return
		}
	}

	featureAttrTypes := map[string]attr.Type{
		keyName:                            types.StringType,
		keyPermissionGroups:                types.SetType{ElemType: types.StringType},
		keyRegions:                         types.SetType{ElemType: types.StringType},
		keyStatus:                          types.StringType,
		keyResourceGroupName:               types.StringType,
		keyResourceGroupRegion:             types.StringType,
		keyResourceGroupTags:               types.MapType{ElemType: types.StringType},
		keyUserAssignedManagedIdentityName: types.StringType,
		keyUserAssignedManagedIdentityPrincipalID: types.StringType,
	}
	featureValues := make([]attr.Value, 0, len(subscription.Features))
	for _, feature := range subscription.Features {
		groupValues := make([]attr.Value, 0, len(feature.PermissionGroups))
		for _, group := range feature.PermissionGroups {
			groupValues = append(groupValues, types.StringValue(string(group)))
		}
		groups, diags := types.SetValue(types.StringType, groupValues)
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return
		}

		regionValues := make([]attr.Value, 0, len(feature.Regions))
		for _, region := range feature.Regions {
			regionValues = append(regionValues, types.StringValue(region))
		}
		regions, diags := types.SetValue(types.StringType, regionValues)
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return
		}

		// Only features supporting a resource group have one.
		resourceGroupName := types.StringNull()
		resourceGroupRegion := types.StringNull()
		resourceGroupTags := types.MapNull(types.StringType)
		if feature.SupportResourceGroup() {
			tagValues := make(map[string]attr.Value, len(feature.ResourceGroup.Tags))
			for key, value := range feature.ResourceGroup.Tags {
				tagValues[key] = types.StringValue(value)
			}
			tags, diags := types.MapValue(types.StringType, tagValues)
			res.Diagnostics.Append(diags...)
			if res.Diagnostics.HasError() {
				return
			}
			resourceGroupName = types.StringValue(feature.ResourceGroup.Name)
			resourceGroupRegion = types.StringValue(feature.ResourceGroup.Region)
			resourceGroupTags = tags
		}

		// The data source is read-only, so the managed identity is exposed
		// whenever RSC returns one, regardless of the feature.
		identityName := types.StringNull()
		identityPrincipalID := types.StringNull()
		if identity := feature.UserAssignedManagedIdentity; identity.Name != "" || identity.PrincipalID != "" {
			identityName = types.StringValue(identity.Name)
			identityPrincipalID = types.StringValue(identity.PrincipalID)
		}

		featureValue, diags := types.ObjectValue(featureAttrTypes, map[string]attr.Value{
			keyName:                            types.StringValue(feature.Name),
			keyPermissionGroups:                groups,
			keyRegions:                         regions,
			keyStatus:                          types.StringValue(string(feature.Status)),
			keyResourceGroupName:               resourceGroupName,
			keyResourceGroupRegion:             resourceGroupRegion,
			keyResourceGroupTags:               resourceGroupTags,
			keyUserAssignedManagedIdentityName: identityName,
			keyUserAssignedManagedIdentityPrincipalID: identityPrincipalID,
		})
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return
		}
		featureValues = append(featureValues, featureValue)
	}
	features, diags := types.SetValue(types.ObjectType{AttrTypes: featureAttrTypes}, featureValues)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	state := azureSubscriptionModel{
		ID:             types.StringValue(subscription.ID.String()),
		Feature:        features,
		Name:           types.StringValue(subscription.Name),
		SubscriptionID: types.StringValue(subscription.NativeID.String()),
		TenantDomain:   types.StringValue(subscription.TenantDomain),
	}

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}
