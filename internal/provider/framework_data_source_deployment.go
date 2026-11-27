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
	"crypto/sha256"
	"fmt"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

const dataSourceDeploymentDescription = `
The ´rubrik_deployment´ data source is used to access information about the RSC
deployment.
`

var _ datasource.DataSource = &deploymentDataSource{}

type deploymentDataSource struct {
	client *client
	prefix string
}

// deploymentModel is the data source state. Every attribute is computed.
type deploymentModel struct {
	ID          types.String `tfsdk:"id"`
	IPAddresses []string     `tfsdk:"ip_addresses"`
	Version     types.String `tfsdk:"version"`
}

func newDeploymentDataSource() datasource.DataSource {
	return &deploymentDataSource{prefix: keyRubrik}
}

func newPolarisDeploymentDataSource() datasource.DataSource {
	return &deploymentDataSource{prefix: keyPolaris}
}

func (d *deploymentDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "deploymentDataSource.Metadata")

	res.TypeName = d.prefix + "_" + keyDeployment
}

func (d *deploymentDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "deploymentDataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSourceDeploymentDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "SHA-256 hash of the fields in order.",
			},
			keyIPAddresses: schema.SetAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Deployment IP addresses.",
			},
			keyVersion: schema.StringAttribute{
				Computed:    true,
				Description: "Deployment version.",
			},
		},
	}

	if d.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_deployment` data source instead."
	}
}

func (d *deploymentDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "deploymentDataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

func (d *deploymentDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "deploymentDataSource.Read")

	polarisClient, err := d.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	ipAddresses, err := core.Wrap(polarisClient.GQL).DeploymentIPAddresses(ctx)
	if err != nil {
		res.Diagnostics.AddError("Failed to read deployment IP addresses", err.Error())
		return
	}
	version, err := polarisClient.GQL.DeploymentVersion(ctx)
	if err != nil {
		res.Diagnostics.AddError("Failed to read deployment version", err.Error())
		return
	}

	// The ID is a hash of the IP addresses in the order returned by RSC,
	// followed by the version.
	hash := sha256.New()
	for _, ipAddress := range ipAddresses {
		hash.Write([]byte(ipAddress))
	}
	hash.Write([]byte(version))

	// The ip_addresses attribute is a set, so duplicates are removed. The slice
	// is never nil so that an empty result is an empty set, not null.
	addresses := slices.Clone(ipAddresses)
	if addresses == nil {
		addresses = []string{}
	}
	slices.Sort(addresses)
	addresses = slices.Compact(addresses)

	state := deploymentModel{
		ID:          types.StringValue(fmt.Sprintf("%x", hash.Sum(nil))),
		IPAddresses: addresses,
		Version:     types.StringValue(string(version)),
	}
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}
