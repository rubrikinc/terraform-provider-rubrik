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

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/cluster"
)

const dataSourceSLASourceClusterDescription = `
The ´rubrik_sla_source_cluster´ data source is used to access information about
an SLA source cluster in RSC. A source cluster is looked up using the cluster name.
`

var _ datasource.DataSource = &slaSourceClusterDataSource{}

type slaSourceClusterDataSource struct {
	client *client
	prefix string
}

// slaSourceClusterModel is the data source state. The name is the lookup key,
// everything else is computed.
type slaSourceClusterModel struct {
	ID      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	Version types.String `tfsdk:"version"`
}

func newSLASourceClusterDataSource() datasource.DataSource {
	return &slaSourceClusterDataSource{prefix: keyRubrik}
}

func newPolarisSLASourceClusterDataSource() datasource.DataSource {
	return &slaSourceClusterDataSource{prefix: keyPolaris}
}

func (d *slaSourceClusterDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "slaSourceClusterDataSource.Metadata")

	res.TypeName = d.prefix + "_" + keySLASourceCluster
}

func (d *slaSourceClusterDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "slaSourceClusterDataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSourceSLASourceClusterDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "Cluster ID (UUID).",
			},
			keyName: schema.StringAttribute{
				Required:    true,
				Description: "Cluster name.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyVersion: schema.StringAttribute{
				Computed:    true,
				Description: "Cluster version.",
			},
		},
	}

	if d.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_sla_source_cluster` data source instead."
	}
}

func (d *slaSourceClusterDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "slaSourceClusterDataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

func (d *slaSourceClusterDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "slaSourceClusterDataSource.Read")

	var config slaSourceClusterModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := d.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	clusterData, err := cluster.Wrap(polarisClient).SLASourceClusterByName(ctx, config.Name.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Failed to read SLA source cluster", err.Error())
		return
	}

	// The lookup matches the name exactly, so the name returned by RSC is the
	// configured name. A configured attribute cannot change during the read,
	// so the configured value is kept.
	state := slaSourceClusterModel{
		ID:      types.StringValue(clusterData.ID.String()),
		Name:    config.Name,
		Version: types.StringValue(clusterData.Version),
	}
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}
