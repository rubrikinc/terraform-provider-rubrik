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
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/archival"
)

const dataSourceNCDArchivalLocationDescription = `
The ´rubrik_ncd_archival_location´ data source is used to access information about
a NAS Cloud Direct (NCD) archival location for a specific cluster. An archival location
is looked up using the cluster ID and archival location name.

Use this data source to get the archival location ID for use with ´ncd_config´ backup
location fields in an SLA domain resource.
`

var _ datasource.DataSource = &ncdArchivalLocationDataSource{}

type ncdArchivalLocationDataSource struct {
	client *client
	prefix string
}

// ncdArchivalLocationModel is the data source state. The cluster ID and the
// name are the lookup keys, everything else is computed.
type ncdArchivalLocationModel struct {
	ID             types.String `tfsdk:"id"`
	ClusterID      types.String `tfsdk:"cluster_id"`
	Name           types.String `tfsdk:"name"`
	Status         types.String `tfsdk:"status"`
	TargetType     types.String `tfsdk:"target_type"`
	Active         types.Bool   `tfsdk:"active"`
	ClusterName    types.String `tfsdk:"cluster_name"`
	ClusterStatus  types.String `tfsdk:"cluster_status"`
	ClusterVersion types.String `tfsdk:"cluster_version"`
}

func newNCDArchivalLocationDataSource() datasource.DataSource {
	return &ncdArchivalLocationDataSource{prefix: keyRubrik}
}

func newPolarisNCDArchivalLocationDataSource() datasource.DataSource {
	return &ncdArchivalLocationDataSource{prefix: keyPolaris}
}

func (d *ncdArchivalLocationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "ncdArchivalLocationDataSource.Metadata")

	res.TypeName = d.prefix + "_" + keyNCDArchivalLocation
}

func (d *ncdArchivalLocationDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "ncdArchivalLocationDataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSourceNCDArchivalLocationDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "Archival location ID (UUID).",
			},
			keyClusterID: schema.StringAttribute{
				Required:    true,
				Description: "Cluster ID (UUID).",
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyName: schema.StringAttribute{
				Required:    true,
				Description: "Name of the archival location.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyStatus: schema.StringAttribute{
				Computed:    true,
				Description: "Status of the archival location.",
			},
			keyTargetType: schema.StringAttribute{
				Computed:    true,
				Description: "Type of the archival target.",
			},
			keyActive: schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the archival location is active.",
			},
			keyClusterName: schema.StringAttribute{
				Computed:    true,
				Description: "Name of the cluster.",
			},
			keyClusterStatus: schema.StringAttribute{
				Computed:    true,
				Description: "Status of the cluster.",
			},
			keyClusterVersion: schema.StringAttribute{
				Computed:    true,
				Description: "Version of the cluster.",
			},
		},
	}

	if d.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_ncd_archival_location` data source instead."
	}
}

func (d *ncdArchivalLocationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "ncdArchivalLocationDataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

func (d *ncdArchivalLocationDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "ncdArchivalLocationDataSource.Read")

	var config ncdArchivalLocationModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := d.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	clusterID, err := uuid.Parse(config.ClusterID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid cluster ID", err.Error())
		return
	}
	name := config.Name.ValueString()

	locations, err := archival.Wrap(polarisClient).NCDArchivalLocationByName(ctx, clusterID, name)
	if err != nil {
		res.Diagnostics.AddError("Failed to read NCD archival location", err.Error())
		return
	}

	// Filter for an exact name match since the API uses "contains" matching.
	inx := -1
	for i, loc := range locations {
		if loc.Name == name {
			if inx != -1 {
				res.Diagnostics.AddError("Failed to read NCD archival location",
					fmt.Sprintf("multiple NCD archival locations found for cluster %s with name %s", clusterID, name))
				return
			}
			inx = i
		}
	}
	if inx == -1 {
		res.Diagnostics.AddError("Failed to read NCD archival location",
			fmt.Sprintf("no NCD archival location found for cluster %s with name %s", clusterID, name))
		return
	}
	location := locations[inx]

	// The location is selected by exact name match, so the name is the
	// configured name. A configured attribute cannot change during the read,
	// so the configured cluster ID and name are kept.
	state := ncdArchivalLocationModel{
		ID:             types.StringValue(location.ID.String()),
		ClusterID:      config.ClusterID,
		Name:           config.Name,
		Status:         types.StringValue(location.Status),
		TargetType:     types.StringValue(location.TargetType),
		Active:         types.BoolValue(location.IsActive),
		ClusterName:    types.StringValue(location.Cluster.Name),
		ClusterStatus:  types.StringValue(location.Cluster.Status),
		ClusterVersion: types.StringValue(location.Cluster.Version),
	}
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}
