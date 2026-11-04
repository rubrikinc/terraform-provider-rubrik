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

const dataSourceDataCenterArchivalLocationDescription = `
The ´rubrik_data_center_archival_location´ data source is used to access information about
a data center archival location for a specific cluster. An archival location is looked
up using the cluster ID and archival location name.
`

var _ datasource.DataSource = &dataCenterArchivalLocationDataSource{}

type dataCenterArchivalLocationDataSource struct {
	client *client
	prefix string
}

// dataCenterArchivalLocationModel is the data source state. The cluster ID
// and the name are the lookup keys, everything else is computed.
type dataCenterArchivalLocationModel struct {
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

func newDataCenterArchivalLocationDataSource() datasource.DataSource {
	return &dataCenterArchivalLocationDataSource{prefix: keyRubrik}
}

func newPolarisDataCenterArchivalLocationDataSource() datasource.DataSource {
	return &dataCenterArchivalLocationDataSource{prefix: keyPolaris}
}

func (d *dataCenterArchivalLocationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "dataCenterArchivalLocationDataSource.Metadata")

	res.TypeName = d.prefix + "_" + keyDataCenterArchivalLocation
}

func (d *dataCenterArchivalLocationDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "dataCenterArchivalLocationDataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSourceDataCenterArchivalLocationDescription),
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
		res.Schema.DeprecationMessage = "use the `rubrik_data_center_archival_location` data source instead."
	}
}

func (d *dataCenterArchivalLocationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "dataCenterArchivalLocationDataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

func (d *dataCenterArchivalLocationDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "dataCenterArchivalLocationDataSource.Read")

	var config dataCenterArchivalLocationModel
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

	locations, err := archival.Wrap(polarisClient).ClusterArchivalLocationByName(ctx, clusterID, name)
	if err != nil {
		res.Diagnostics.AddError("Failed to read archival location", err.Error())
		return
	}
	if len(locations) == 0 {
		res.Diagnostics.AddError("Failed to read archival location",
			fmt.Sprintf("no archival location found for cluster %s with name %s", clusterID, name))
		return
	}
	if len(locations) > 1 {
		res.Diagnostics.AddError("Failed to read archival location",
			fmt.Sprintf("multiple archival locations found for cluster %s with name %s", clusterID, name))
		return
	}
	location := locations[0]

	// The configured cluster ID and name are kept as is. The lookup is done by
	// name filter in RSC and a configured attribute cannot change during the
	// read.
	state := dataCenterArchivalLocationModel{
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
