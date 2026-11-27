// Copyright 2021 Rubrik, Inc.
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
	"crypto/sha256"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/azure"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

const dataSourceAzurePermissionsDescription = `
The ´rubrik_azure_permissions´ data source is used to access information about
the permissions required by RSC for an RSC feature.

The ´rubrik_azure_permissions´ data source can be used with the
´azurerm_role_definition´ resource and the ´permissions´ field of the
´rubrik_azure_subscription´ resource to automatically update the permissions
of roles and notify RSC about the updated.

{{template "azurePermissionGroups"}}

-> **Note:** To better fit the RSC Azure permission model where each RSC feature
   have two Azure roles, the ´features´ field has been deprecated and replaced
   with the ´feature´ field.

-> **Note:** Due to the RSC Azure permission model having been refined into
   subscription level permissions and resource group level permissions, the
   ´actions´, ´data_actions´, ´not_actions´ and ´not_data_actions´ fields have
   been deprecated and replaced with the corresponding subscription and resource
   group fields.
`

var (
	_ datasource.DataSource              = &azurePermissionsDataSource{}
	_ datasource.DataSourceWithConfigure = &azurePermissionsDataSource{}
)

type azurePermissionsDataSource struct {
	client *client
	prefix string
}

type azurePermissionsModel struct {
	ID                          types.String `tfsdk:"id"`
	Actions                     []string     `tfsdk:"actions"`
	DataActions                 []string     `tfsdk:"data_actions"`
	Feature                     types.String `tfsdk:"feature"`
	Features                    []string     `tfsdk:"features"`
	Hash                        types.String `tfsdk:"hash"`
	NotActions                  []string     `tfsdk:"not_actions"`
	NotDataActions              []string     `tfsdk:"not_data_actions"`
	PermissionGroups            []string     `tfsdk:"permission_groups"`
	ResourceGroupActions        []string     `tfsdk:"resource_group_actions"`
	ResourceGroupDataActions    []string     `tfsdk:"resource_group_data_actions"`
	ResourceGroupNotActions     []string     `tfsdk:"resource_group_not_actions"`
	ResourceGroupNotDataActions []string     `tfsdk:"resource_group_not_data_actions"`
	SubscriptionActions         []string     `tfsdk:"subscription_actions"`
	SubscriptionDataActions     []string     `tfsdk:"subscription_data_actions"`
	SubscriptionNotActions      []string     `tfsdk:"subscription_not_actions"`
	SubscriptionNotDataActions  []string     `tfsdk:"subscription_not_data_actions"`
}

func newAzurePermissionsDataSource() datasource.DataSource {
	return &azurePermissionsDataSource{prefix: keyRubrik}
}

func newPolarisAzurePermissionsDataSource() datasource.DataSource {
	return &azurePermissionsDataSource{prefix: keyPolaris}
}

func (d *azurePermissionsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "azurePermissionsDataSource.Metadata")

	res.TypeName = d.prefix + "_" + keyAzurePermissions
}

func (d *azurePermissionsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "azurePermissionsDataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSourceAzurePermissionsDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed: true,
				Description: "SHA-256 hash of the required permissions, will be updated as the required permissions " +
					"changes.",
			},
			keyActions: schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Azure allowed actions. **Deprecated:** use `subscription_actions` and " +
					"`resource_group_actions` instead.",
				DeprecationMessage: "Use `subscription_actions` and `resource_group_actions` instead.",
			},
			keyDataActions: schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Azure allowed data actions. **Deprecated:** use `subscription_data_actions` and " +
					"`resource_group_data_actions` instead.",
				DeprecationMessage: "Use `subscription_data_actions` and `resource_group_data_actions` instead.",
			},
			keyFeature: schema.StringAttribute{
				Optional: true,
				Description: fmt.Sprintf("RSC feature to look up permissions for. %s.",
					possibleValues(core.AzureSubscriptionFeatureNames())),
				Validators: []validator.String{
					stringvalidator.OneOf(core.AzureSubscriptionFeatureNames()...),
					stringvalidator.ExactlyOneOf(path.MatchRoot(keyFeatures)),
				},
			},
			keyFeatures: schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: fmt.Sprintf("RSC features. %s. **Deprecated:** use `feature` instead.",
					possibleValues(core.AzureSubscriptionFeatureNames())),
				DeprecationMessage: "Use `feature` instead",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(stringvalidator.OneOf(core.AzureSubscriptionFeatureNames()...)),
				},
			},
			keyHash: schema.StringAttribute{
				Computed: true,
				Description: "SHA-256 hash of the permissions, can be used to detect changes to the permissions. " +
					"**Deprecated:** use `id` instead.",
				DeprecationMessage: "Use `id` instead.",
			},
			keyNotActions: schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Azure disallowed actions. **Deprecated:** use `subscription_not_actions` and " +
					"`resource_group_not_actions` instead.",
				DeprecationMessage: "Use `subscription_not_actions` and `resource_group_not_actions` instead.",
			},
			keyNotDataActions: schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Azure disallowed data actions. **Deprecated:** use `subscription_not_data_actions` and " +
					"`resource_group_not_data_actions` instead.",
				DeprecationMessage: "Use `subscription_not_data_actions` and `resource_group_not_data_actions` instead.",
			},
			keyPermissionGroups: schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: fmt.Sprintf("Permission groups for the RSC feature. %s.",
					possibleValues(core.AzureSubscriptionPermissionGroupNames())),
				Validators: []validator.Set{
					setvalidator.ValueStringsAre(stringvalidator.OneOf(core.AzureSubscriptionPermissionGroupNames()...)),
					setvalidator.ConflictsWith(path.MatchRoot(keyFeatures)),
					setvalidator.AlsoRequires(path.MatchRoot(keyFeature)),
				},
			},
			keyResourceGroupActions: schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Azure allowed actions on the resource group level.",
			},
			keyResourceGroupDataActions: schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Azure allowed data actions on the resource group level.",
			},
			keyResourceGroupNotActions: schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Azure disallowed actions on the resource group level.",
			},
			keyResourceGroupNotDataActions: schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Azure disallowed data actions on the resource group level.",
			},
			keySubscriptionActions: schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Azure allowed actions on the subscription level.",
			},
			keySubscriptionDataActions: schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Azure allowed data actions on the subscription level.",
			},
			keySubscriptionNotActions: schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Azure disallowed actions on the subscription level.",
			},
			keySubscriptionNotDataActions: schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Azure disallowed data actions on the subscription level.",
			},
		},
	}

	if d.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_azure_permissions` data source instead."
	}
}

func (d *azurePermissionsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "azurePermissionsDataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

func (d *azurePermissionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "azurePermissionsDataSource.Read")

	var config azurePermissionsModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := d.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	// Check both feature and features.
	var perms []azure.Permissions
	var groups []azure.PermissionGroupWithVersion
	if !config.Feature.IsNull() {
		if err := checkFeaturePermissionGroups(core.AzureSubscriptionFeatures(), config.Feature.ValueString(), config.PermissionGroups); err != nil {
			res.Diagnostics.AddError("Invalid permission group", err.Error())
			return
		}
		var permGroups []core.PermissionGroup
		for _, permGroupName := range config.PermissionGroups {
			permGroups = append(permGroups, core.PermissionGroup(permGroupName))
		}
		feature := core.Feature{Name: config.Feature.ValueString(), PermissionGroups: permGroups}
		perms, groups, err = azure.Wrap(polarisClient).ScopedPermissions(ctx, feature)
	} else {
		var features []core.Feature
		for _, featureName := range config.Features {
			features = append(features, core.Feature{Name: featureName})
		}
		perms, err = azure.Wrap(polarisClient).ScopedPermissionsForFeatures(ctx, features)
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read Azure permissions", err.Error())
		return
	}

	hash := sha256.New()

	// Legacy scope. The legacy scope contains the union of the subscription
	// and the resource group scopes, so we only need to update the hash value
	// here, with the added benefit of keeping it backwards compatible.
	for _, perm := range perms[azure.ScopeLegacy].Actions {
		hash.Write([]byte(perm))
	}
	for _, perm := range perms[azure.ScopeLegacy].DataActions {
		hash.Write([]byte(perm))
	}
	for _, perm := range perms[azure.ScopeLegacy].NotActions {
		hash.Write([]byte(perm))
	}
	for _, perm := range perms[azure.ScopeLegacy].NotDataActions {
		hash.Write([]byte(perm))
	}

	// Hash permission groups. This generates a diff for subscription onboarded
	// with the old onboarding workflow. Applying the diff fixes the backend
	// state.
	for _, group := range groups {
		hash.Write([]byte(group.Name))
		hash.Write([]byte(fmt.Sprintf("%d", group.Version)))
	}
	hashValue := fmt.Sprintf("%x", hash.Sum(nil))

	state := azurePermissionsModel{
		ID:                          types.StringValue(hashValue),
		Actions:                     append([]string{}, perms[azure.ScopeLegacy].Actions...),
		DataActions:                 append([]string{}, perms[azure.ScopeLegacy].DataActions...),
		Feature:                     config.Feature,
		Features:                    config.Features,
		Hash:                        types.StringValue(hashValue),
		NotActions:                  append([]string{}, perms[azure.ScopeLegacy].NotActions...),
		NotDataActions:              append([]string{}, perms[azure.ScopeLegacy].NotDataActions...),
		PermissionGroups:            config.PermissionGroups,
		ResourceGroupActions:        append([]string{}, perms[azure.ScopeResourceGroup].Actions...),
		ResourceGroupDataActions:    append([]string{}, perms[azure.ScopeResourceGroup].DataActions...),
		ResourceGroupNotActions:     append([]string{}, perms[azure.ScopeResourceGroup].NotActions...),
		ResourceGroupNotDataActions: append([]string{}, perms[azure.ScopeResourceGroup].NotDataActions...),
		SubscriptionActions:         append([]string{}, perms[azure.ScopeSubscription].Actions...),
		SubscriptionDataActions:     append([]string{}, perms[azure.ScopeSubscription].DataActions...),
		SubscriptionNotActions:      append([]string{}, perms[azure.ScopeSubscription].NotActions...),
		SubscriptionNotDataActions:  append([]string{}, perms[azure.ScopeSubscription].NotDataActions...),
	}

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}
