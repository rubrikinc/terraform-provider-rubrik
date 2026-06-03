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
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/cluster"
)

const dataSourceClusterSettingsDescription = `
The ´rubrik_cluster_settings´ data source returns the upgrade state of a
single Rubrik cluster registered with RSC.
`

var _ datasource.DataSource = &clusterSettingsDataSource{}

type clusterSettingsDataSource struct {
	client *client
}

func newClusterSettingsDataSource() datasource.DataSource {
	return &clusterSettingsDataSource{}
}

func (d *clusterSettingsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "clusterSettingsDataSource.Metadata")

	res.TypeName = keyRubrik + "_" + keyClusterSettings
}

func (d *clusterSettingsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "clusterSettingsDataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSourceClusterSettingsDescription),
		Attributes: map[string]schema.Attribute{
			keyClusterID: schema.StringAttribute{
				Required:    true,
				Description: "Cluster UUID to look up.",
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyName: schema.StringAttribute{
				Computed:    true,
				Description: "Cluster name.",
			},
			keyVersion: schema.StringAttribute{
				Computed:    true,
				Description: "Currently installed CDM version.",
			},
			keyFastUpgradePreferred: schema.BoolAttribute{
				Computed:    true,
				Description: "True if the cluster is configured for FAST upgrades; false for ROLLING.",
			},
			keyRollingUpgradeSupported: schema.BoolAttribute{
				Computed:    true,
				Description: "True if the cluster supports rolling upgrades.",
			},
			keyUpgradeStatusV2: schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Authoritative upgrade status of the cluster.",
				Attributes: map[string]schema.Attribute{
					keyRSCClusterUpgradeStatus: schema.StringAttribute{
						Computed:    true,
						Description: "RSC cluster upgrade status.",
					},
					keyUIStatusAttributes: schema.SingleNestedAttribute{
						Computed:    true,
						Description: "Detailed attributes of the current upgrade status.",
						Attributes: map[string]schema.Attribute{
							keySourceVersion: schema.StringAttribute{
								Computed:    true,
								Description: "Version being upgraded from.",
							},
							keyTargetVersion: schema.StringAttribute{
								Computed:    true,
								Description: "Version being upgraded to.",
							},
							keyProgress: schema.Float64Attribute{
								Computed:    true,
								Description: "Upgrade progress percentage.",
							},
							keyErrorMsg: schema.StringAttribute{
								Computed:    true,
								Description: "Error message for the current upgrade status, if any.",
							},
							keyUpgradeMode: schema.StringAttribute{
								Computed:    true,
								Description: "Upgrade mode (FAST or ROLLING).",
							},
						},
					},
				},
			},
			keyLastUpgradeDuration: schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Duration in seconds of the last successful upgrade, by mode.",
				Attributes: map[string]schema.Attribute{
					keyFastUpgradeDuration: schema.Int64Attribute{
						Computed:    true,
						Description: "Last successful FAST upgrade duration, in seconds.",
					},
					keyRollingUpgradeDuration: schema.Int64Attribute{
						Computed:    true,
						Description: "Last successful ROLLING upgrade duration, in seconds.",
					},
				},
			},
		},
	}
}

func (d *clusterSettingsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "clusterSettingsDataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

func (d *clusterSettingsDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "clusterSettingsDataSource.Read")

	var config clusterSettingsModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	clusterUUID, err := uuid.Parse(config.ClusterID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid cluster UUID", err.Error())
		return
	}

	polarisClient, err := d.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	details, err := cluster.Wrap(polarisClient).ClusterUpgrade(ctx, clusterUUID)
	if err != nil {
		res.Diagnostics.AddError("Failed to read cluster settings", err.Error())
		return
	}

	state, diags := fromUpgradeDetails(ctx, details)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}
