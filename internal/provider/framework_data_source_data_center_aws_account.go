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
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/archival"
)

const dataSourceDataCenterAWSAccountDescription = `
The ´rubrik_data_center_aws_account´ data source is used to access information
about an AWS data center account added to RSC. A data center account is looked
up using the name.

-> **Note:** Data center accounts and cloud native accounts are different and
   cannot be used interchangeably.

-> **Note:** The name is the name of the data center account as it appears in
   RSC.
`

var _ datasource.DataSource = &dataCenterAWSAccountDataSource{}
var _ datasource.DataSourceWithConfigure = &dataCenterAWSAccountDataSource{}

type dataCenterAWSAccountDataSource struct {
	client *client
	prefix string
}

type dataCenterAWSAccountDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	AccessKey        types.String `tfsdk:"access_key"`
	ConnectionStatus types.String `tfsdk:"connection_status"`
	Description      types.String `tfsdk:"description"`
	Name             types.String `tfsdk:"name"`
}

func newDataCenterAWSAccountDataSource() datasource.DataSource {
	return &dataCenterAWSAccountDataSource{prefix: keyRubrik}
}

func newPolarisDataCenterAWSAccountDataSource() datasource.DataSource {
	return &dataCenterAWSAccountDataSource{prefix: keyPolaris}
}

func (d *dataCenterAWSAccountDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "dataCenterAWSAccountDataSource.Metadata")

	res.TypeName = d.prefix + "_" + keyDataCenterAWSAccount
}

func (d *dataCenterAWSAccountDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "dataCenterAWSAccountDataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSourceDataCenterAWSAccountDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "RSC data center cloud account ID (UUID).",
			},
			keyAccessKey: schema.StringAttribute{
				Computed:    true,
				Description: "AWS access key.",
			},
			keyConnectionStatus: schema.StringAttribute{
				Computed:    true,
				Description: "Connection status.",
			},
			keyDescription: schema.StringAttribute{
				Computed:    true,
				Description: "Data center account description.",
			},
			keyName: schema.StringAttribute{
				Required:    true,
				Description: "Data center account name.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
		},
	}

	if d.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_data_center_aws_account` data source instead."
	}
}

func (d *dataCenterAWSAccountDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "dataCenterAWSAccountDataSource.Configure")

	if req.ProviderData == nil {
		return
	}

	d.client = req.ProviderData.(*client)
}

func (d *dataCenterAWSAccountDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "dataCenterAWSAccountDataSource.Read")

	var config dataCenterAWSAccountDataSourceModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	client, err := d.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	// Read the AWS cloud account using the name. We don't allow prefix searches
	// since it would be impossible to uniquely identify a cloud account with a
	// name being the prefix of another cloud account.
	cloudAccount, err := archival.Wrap(client).AWSCloudAccountByName(ctx, config.Name.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Failed to Read Data Center AWS Account", err.Error())
		return
	}

	state := dataCenterAWSAccountDataSourceModel{
		ID:               types.StringValue(cloudAccount.ID.String()),
		AccessKey:        types.StringValue(cloudAccount.AccessKey),
		ConnectionStatus: types.StringValue(cloudAccount.ConnectionStatus),
		Description:      types.StringValue(cloudAccount.Description),
		Name:             types.StringValue(config.Name.ValueString()),
	}

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}
