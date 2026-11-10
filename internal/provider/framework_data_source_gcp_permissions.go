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
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/gcp"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

const dataSourceGCPPermissionsDescription = `
The ´rubrik_gcp_permissions´ data source is used to access information about
the permissions required by RSC for an RSC feature.

The ´rubrik_gcp_permissions´ data source can be used with the
´google_project_iam_custom_role´ resource and the ´permissions´ field of the
´rubrik_gcp_project´ resource to automatically update the permissions of roles
and notify RSC about the updated.

{{template "gcpPermissionGroups"}}

-> **Note:** Due to backward compatibility, the ´features´ field allow the
   feature names to be given in 3 different styles: ´EXAMPLE_FEATURE_NAME´,
   ´example-feature-name´ or ´example_feature_name´. The recommended style is
   ´EXAMPLE_FEATURE_NAME´ as it is what the RSC API itself uses.
`

var (
	_ datasource.DataSource              = &gcpPermissionsDataSource{}
	_ datasource.DataSourceWithConfigure = &gcpPermissionsDataSource{}
)

type gcpPermissionsDataSource struct {
	client *client
	prefix string
}

type gcpPermissionsModel struct {
	ID                types.String `tfsdk:"id"`
	Conditions        types.Set    `tfsdk:"conditions"`
	Feature           types.String `tfsdk:"feature"`
	Features          types.Set    `tfsdk:"features"`
	Hash              types.String `tfsdk:"hash"`
	PermissionGroups  types.Set    `tfsdk:"permission_groups"`
	Permissions       types.List   `tfsdk:"permissions"`
	Services          types.Set    `tfsdk:"services"`
	WithConditions    types.Set    `tfsdk:"with_conditions"`
	WithoutConditions types.Set    `tfsdk:"without_conditions"`
}

func newGcpPermissionsDataSource() datasource.DataSource {
	return &gcpPermissionsDataSource{prefix: keyRubrik}
}

func newPolarisGcpPermissionsDataSource() datasource.DataSource {
	return &gcpPermissionsDataSource{prefix: keyPolaris}
}

func (d *gcpPermissionsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "gcpPermissionsDataSource.Metadata")

	res.TypeName = d.prefix + "_" + keyGCPPermissions
}

func (d *gcpPermissionsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "gcpPermissionsDataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSourceGCPPermissionsDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed: true,
				Description: "SHA-256 hash of the required permissions, will be updated as the required permissions " +
					"changes.",
			},
			keyConditions: schema.SetAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Conditions for the permissions with conditions.",
			},
			keyFeature: schema.StringAttribute{
				Optional: true,
				Description: fmt.Sprintf("RSC feature to look up permissions for. %s.",
					possibleValues(core.GCPProjectFeatureNames())),
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.MatchRoot(keyFeature), path.MatchRoot(keyFeatures)),
					stringvalidator.OneOf(core.GCPProjectFeatureNames()...),
				},
			},
			keyFeatures: schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: fmt.Sprintf("RSC features. %s. **Deprecated:** use `feature` instead.",
					possibleValues(core.GCPProjectFeatureNames())),
				DeprecationMessage: "Use `feature` instead",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(stringvalidator.OneOf(core.GCPProjectFeatureNames()...)),
				},
			},
			keyHash: schema.StringAttribute{
				Computed: true,
				Description: "SHA-256 hash of the permissions, can be used to detect changes to the permissions. " +
					"**Deprecated:** use `id` instead.",
				DeprecationMessage: "Use `id` instead.",
			},
			// Optional and computed, since the permission groups are read back
			// from RSC when they are not specified. The SDKv2 implementation
			// did the same, but SDKv2 allowed setting optional attributes.
			keyPermissionGroups: schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Description: fmt.Sprintf("Permission groups for the RSC feature. %s.",
					possibleValues(core.GCPProjectPermissionGroupNames())),
				Validators: []validator.Set{
					setvalidator.ConflictsWith(path.MatchRoot(keyFeatures)),
					setvalidator.AlsoRequires(path.MatchRoot(keyFeature)),
					setvalidator.ValueStringsAre(stringvalidator.OneOf(core.GCPProjectPermissionGroupNames()...)),
				},
			},
			keyPermissions: schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Permissions required for the set of RSC features. Includes permissions with " +
					"conditions. **Deprecated:** use `with_conditions` and `without_conditions` instead.",
				DeprecationMessage: "use: `with_conditions` and `without_conditions` instead.",
			},
			keyServices: schema.SetAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "GCP services required for the RSC feature.",
			},
			keyWithConditions: schema.SetAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Permissions with conditions required for the RSC feature.",
			},
			keyWithoutConditions: schema.SetAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Permissions without conditions required for the RSC feature.",
			},
		},
	}

	if d.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_gcp_permissions` data source instead."
	}
}

func (d *gcpPermissionsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "gcpPermissionsDataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

func (d *gcpPermissionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "gcpPermissionsDataSource.Read")

	var config gcpPermissionsModel
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
	var features []core.Feature
	if !config.Feature.IsNull() {
		var groups []string
		if !config.PermissionGroups.IsNull() {
			res.Diagnostics.Append(config.PermissionGroups.ElementsAs(ctx, &groups, false)...)
			if res.Diagnostics.HasError() {
				return
			}
		}
		if err := checkFeaturePermissionGroups(core.GCPProjectFeatures(), config.Feature.ValueString(), groups); err != nil {
			res.Diagnostics.AddError("Invalid permission group", err.Error())
			return
		}
		feature := core.Feature{Name: config.Feature.ValueString()}
		for _, group := range groups {
			feature.PermissionGroups = append(feature.PermissionGroups, core.PermissionGroup(group))
		}
		features = append(features, feature)
	} else {
		var names []string
		res.Diagnostics.Append(config.Features.ElementsAs(ctx, &names, false)...)
		if res.Diagnostics.HasError() {
			return
		}
		for _, name := range names {
			features = append(features, core.Feature{Name: name})
		}
	}
	featurePerms, err := gcp.Wrap(polarisClient).FeaturePermissions(ctx, features)
	if err != nil {
		res.Diagnostics.AddError("Failed to read GCP permissions", err.Error())
		return
	}

	// The single feature fields are empty, not null, when more than one
	// feature is looked up, which matches the SDKv2 implementation.
	emptySet := types.SetValueMust(types.StringType, []attr.Value{})
	state := gcpPermissionsModel{
		Conditions:        emptySet,
		Feature:           config.Feature,
		Features:          config.Features,
		PermissionGroups:  config.PermissionGroups,
		Services:          emptySet,
		WithConditions:    emptySet,
		WithoutConditions: emptySet,
	}

	// The order the values are written to the hash must not change, since it
	// would change the ID of the data source.
	hash := sha256.New()
	if len(featurePerms) == 1 {
		groupValues := make([]attr.Value, 0, len(featurePerms[0].Feature.PermissionGroups))
		for _, group := range featurePerms[0].Feature.PermissionGroups {
			groupValues = append(groupValues, types.StringValue(string(group)))
		}
		groupSet, diags := types.SetValue(types.StringType, groupValues)
		res.Diagnostics.Append(diags...)

		conditionValues := make([]attr.Value, 0, len(featurePerms[0].Conditions))
		for _, condition := range featurePerms[0].Conditions {
			conditionValues = append(conditionValues, types.StringValue(condition))
			hash.Write([]byte(condition))
		}
		conditionSet, diags := types.SetValue(types.StringType, conditionValues)
		res.Diagnostics.Append(diags...)

		withValues := make([]attr.Value, 0, len(featurePerms[0].WithConditions))
		for _, perm := range featurePerms[0].WithConditions {
			withValues = append(withValues, types.StringValue(perm))
			hash.Write([]byte(perm))
		}
		withSet, diags := types.SetValue(types.StringType, withValues)
		res.Diagnostics.Append(diags...)

		withoutValues := make([]attr.Value, 0, len(featurePerms[0].WithoutConditions))
		for _, perm := range featurePerms[0].WithoutConditions {
			withoutValues = append(withoutValues, types.StringValue(perm))
			hash.Write([]byte(perm))
		}
		withoutSet, diags := types.SetValue(types.StringType, withoutValues)
		res.Diagnostics.Append(diags...)

		serviceValues := make([]attr.Value, 0, len(featurePerms[0].Services))
		for _, service := range featurePerms[0].Services {
			if service == "COMPUTE_ENGINE_API" {
				service = "COMPUTE"
			}
			serviceValues = append(serviceValues,
				types.StringValue(strings.ToLower(strings.ReplaceAll(service, "_", "")+".googleapis.com")))
			hash.Write([]byte(service))
		}
		serviceSet, diags := types.SetValue(types.StringType, serviceValues)
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return
		}

		// A configured value must be kept as is, Terraform rejects a data
		// source changing a configured value.
		state.PermissionGroups = groupSet
		if !config.PermissionGroups.IsNull() {
			state.PermissionGroups = config.PermissionGroups
		}
		state.Conditions = conditionSet
		state.WithConditions = withSet
		state.WithoutConditions = withoutSet
		state.Services = serviceSet
	}

	var perms []string
	for _, featurePerms := range featurePerms {
		perms = append(perms, featurePerms.WithConditions...)
		perms = append(perms, featurePerms.WithoutConditions...)
	}
	slices.Sort(perms)
	perms = slices.Compact(perms)
	permValues := make([]attr.Value, 0, len(perms))
	for _, perm := range perms {
		permValues = append(permValues, types.StringValue(perm))
		hash.Write([]byte(perm))
	}
	permList, diags := types.ListValue(types.StringType, permValues)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	state.Permissions = permList

	id := fmt.Sprintf("%x", hash.Sum(nil))
	state.ID = types.StringValue(id)
	state.Hash = types.StringValue(id)

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}
