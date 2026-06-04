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
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/cluster"
	gqlcluster "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/cluster"
)

const dataSourceClusterVersionsDescription = `
The ´rubrik_cluster_versions´ data source returns the CDM releases available to a
single Rubrik cluster registered with RSC, as reported by the Rubrik support
portal.

Feed ´recommended_version´ (or a chosen entry from ´available_versions´) into the
´version´ argument of the ´rubrik_cluster_settings´ resource to drive upgrades.
`

var _ datasource.DataSource = &clusterVersionsDataSource{}

type clusterVersionsDataSource struct {
	client *client
}

func newClusterVersionsDataSource() datasource.DataSource {
	return &clusterVersionsDataSource{}
}

func (d *clusterVersionsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "clusterVersionsDataSource.Metadata")

	res.TypeName = keyRubrik + "_" + keyClusterVersions
}

func (d *clusterVersionsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "clusterVersionsDataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSourceClusterVersionsDescription),
		Attributes: map[string]schema.Attribute{
			keyClusterID: schema.StringAttribute{
				Required:    true,
				Description: "Cluster UUID to look up.",
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyAvailableVersions: schema.ListNestedAttribute{
				Computed:    true,
				Description: "CDM releases available to the cluster, in the order reported by the support portal.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyVersion: schema.StringAttribute{
							Computed:    true,
							Description: "CDM release version.",
						},
						keyRecommended: schema.BoolAttribute{
							Computed:    true,
							Description: "True if Rubrik recommends this release for the cluster.",
						},
						keyUpgradable: schema.BoolAttribute{
							Computed:    true,
							Description: "True if this release is a direct (single-hop) upgrade target from the currently installed version.",
						},
					},
				},
			},
			keyRecommendedVersion: schema.StringAttribute{
				Computed:    true,
				Description: "Version flagged as recommended for the cluster, if any.",
			},
			keyLatestVersion: schema.StringAttribute{
				Computed:    true,
				Description: "Highest available CDM version.",
			},
		},
	}
}

func (d *clusterVersionsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "clusterVersionsDataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

func (d *clusterVersionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "clusterVersionsDataSource.Read")

	var config clusterVersions
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	clusterUUID, err := uuid.Parse(config.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid cluster UUID", err.Error())
		return
	}

	polarisClient, err := d.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	// FetchLinks must be true for the support portal to compute recommendations:
	// the backend only sets a release's isRecommended flag (and resolves the
	// recommended version) when links are fetched. We do not surface the link,
	// md5, or size fields; we only need the recommendation side effect.
	releases, err := gqlcluster.ListUpgrades(ctx, polarisClient.GQL, []uuid.UUID{clusterUUID}, gqlcluster.ListUpgradesOptions{
		ShouldShowAll: true,
		FetchLinks:    true,
	})
	if err != nil {
		res.Diagnostics.AddError("Failed to list available cluster versions", err.Error())
		return
	}

	state, diags := fromReleaseDetails(ctx, clusterUUID.String(), releases)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}

// clusterVersions holds the available CDM releases for a single cluster, as
// reported by the Rubrik support portal.
type clusterVersions struct {
	ID          types.String `tfsdk:"cluster_id"`
	Available   types.List   `tfsdk:"available_versions"`
	Recommended types.String `tfsdk:"recommended_version"`
	Latest      types.String `tfsdk:"latest_version"`
}

// availableVersionAttrTypes describes a single entry of the available_versions
// list.
func availableVersionAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		keyVersion:     types.StringType,
		keyRecommended: types.BoolType,
		keyUpgradable:  types.BoolType,
	}
}

// fromReleaseDetails converts the support-portal release listing for clusterID
// into a clusterVersions. The available_versions list preserves the order
// returned by the SDK. recommended_version is the release flagged Recommended
// (null if none), and latest_version is the highest full release name (null if
// the list is empty or no version parses).
func fromReleaseDetails(ctx context.Context, clusterID string, releases []gqlcluster.ReleaseDetail) (clusterVersions, diag.Diagnostics) {
	var diags diag.Diagnostics

	model := clusterVersions{
		ID:          types.StringValue(clusterID),
		Available:   types.ListNull(types.ObjectType{AttrTypes: availableVersionAttrTypes()}),
		Recommended: types.StringNull(),
		Latest:      types.StringNull(),
	}

	elems := make([]attr.Value, 0, len(releases))
	var latest string
	for _, release := range releases {
		obj, objDiags := types.ObjectValue(availableVersionAttrTypes(), map[string]attr.Value{
			keyVersion:     types.StringValue(release.Name),
			keyRecommended: types.BoolValue(release.Recommended),
			keyUpgradable:  types.BoolValue(release.Upgradable),
		})
		diags.Append(objDiags...)
		elems = append(elems, obj)

		if release.Recommended {
			model.Recommended = types.StringValue(release.Name)
		}
		if isHigherVersion(release.Name, latest) {
			latest = release.Name
		}
	}

	list, listDiags := types.ListValue(types.ObjectType{AttrTypes: availableVersionAttrTypes()}, elems)
	diags.Append(listDiags...)
	model.Available = list

	if latest != "" {
		model.Latest = types.StringValue(latest)
	}

	return model, diags
}

// isHigherVersion reports whether the candidate release name sorts above the
// current one. An empty current is always beaten by any parseable candidate;
// unparseable candidates never win. Comparison (including the trailing build
// number) is delegated to cluster.CDMVersion.
func isHigherVersion(candidate, current string) bool {
	v, err := cluster.ParseCDMVersion(candidate)
	if err != nil {
		return false
	}
	if current == "" {
		return true
	}
	return v.GreaterThan(current)
}
