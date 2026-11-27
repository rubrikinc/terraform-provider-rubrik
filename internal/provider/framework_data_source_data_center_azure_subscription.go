// Copyright 2024 Rubrik, Inc.
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

const dataSourceDataCenterAzureSubscriptionDescription = `
The ´rubrik_data_center_azure_subscription´ data source is used to access
information about an Azure data center subscription added to RSC. A data center
subscription is looked up using the name.

-> **Note:** Data center subscriptions and cloud native subscriptions are
   different and cannot be used interchangeably.

-> **Note:** The name is the name of the data center subscription as it appears
   in RSC.
`

var (
	_ datasource.DataSource              = &dataCenterAzureSubscriptionDataSource{}
	_ datasource.DataSourceWithConfigure = &dataCenterAzureSubscriptionDataSource{}
)

type dataCenterAzureSubscriptionDataSource struct {
	client *client
	prefix string
}

type dataCenterAzureSubscriptionModel struct {
	ID               types.String `tfsdk:"id"`
	ConnectionStatus types.String `tfsdk:"connection_status"`
	Description      types.String `tfsdk:"description"`
	Name             types.String `tfsdk:"name"`
	SubscriptionID   types.String `tfsdk:"subscription_id"`
	TenantID         types.String `tfsdk:"tenant_id"`
}

func newDataCenterAzureSubscriptionDataSource() datasource.DataSource {
	return &dataCenterAzureSubscriptionDataSource{prefix: keyRubrik}
}

func newPolarisDataCenterAzureSubscriptionDataSource() datasource.DataSource {
	return &dataCenterAzureSubscriptionDataSource{prefix: keyPolaris}
}

func (d *dataCenterAzureSubscriptionDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "dataCenterAzureSubscriptionDataSource.Metadata")

	res.TypeName = d.prefix + "_" + keyDataCenterAzureSubscription
}

func (d *dataCenterAzureSubscriptionDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "dataCenterAzureSubscriptionDataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSourceDataCenterAzureSubscriptionDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "RSC data center cloud account ID (UUID).",
			},
			keyConnectionStatus: schema.StringAttribute{
				Computed:    true,
				Description: "Connection status.",
			},
			keyDescription: schema.StringAttribute{
				Computed:    true,
				Description: "Data center subscription description.",
			},
			keyName: schema.StringAttribute{
				Required:    true,
				Description: "Data center subscription name.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keySubscriptionID: schema.StringAttribute{
				Computed:    true,
				Description: "Azure subscription ID (UUID).",
			},
			keyTenantID: schema.StringAttribute{
				Computed:    true,
				Description: "Azure tenant ID (UUID).",
			},
		},
	}

	if d.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_data_center_azure_subscription` data source instead."
	}
}

func (d *dataCenterAzureSubscriptionDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "dataCenterAzureSubscriptionDataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

func (d *dataCenterAzureSubscriptionDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "dataCenterAzureSubscriptionDataSource.Read")

	var config dataCenterAzureSubscriptionModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := d.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	// Read the Azure cloud account using the name. We don't allow prefix
	// searches since it would be impossible to uniquely identify a cloud
	// account with a name being the prefix of another cloud account.
	cloudAccount, err := archival.Wrap(polarisClient).AzureCloudAccountByName(ctx, config.Name.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Failed to read data center Azure subscription", err.Error())
		return
	}

	state := dataCenterAzureSubscriptionModel{
		ID:               types.StringValue(cloudAccount.ID.String()),
		ConnectionStatus: types.StringValue(cloudAccount.ConnectionStatus),
		Description:      types.StringValue(cloudAccount.Description),
		Name:             config.Name,
		SubscriptionID:   types.StringValue(cloudAccount.SubscriptionID.String()),
		TenantID:         types.StringValue(cloudAccount.TenantID.String()),
	}

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}
