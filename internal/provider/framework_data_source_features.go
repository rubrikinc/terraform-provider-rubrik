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
	"cmp"
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

const dataSourceFeaturesDescription = `
The ´rubrik_feature´ data source is used to access information about features enabled
for an RSC account.

!> **WARNING:** This resource is deprecated and will be removed in a future version.
   Use the ´features´ field of the ´rubrik_account´ data source instead.
`

var _ datasource.DataSource = &featuresDataSource{}

type featuresDataSource struct {
	client *client
	prefix string
}

// featuresDataSourceModel is the data source state. Every attribute is
// computed.
type featuresDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Features []string     `tfsdk:"features"`
}

func newFeaturesDataSource() datasource.DataSource {
	return &featuresDataSource{prefix: keyRubrik}
}

func newPolarisFeaturesDataSource() datasource.DataSource {
	return &featuresDataSource{prefix: keyPolaris}
}

func (d *featuresDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "featuresDataSource.Metadata")

	res.TypeName = d.prefix + "_" + keyFeatures
}

func (d *featuresDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "featuresDataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSourceFeaturesDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "SHA-256 hash of the fields in order.",
			},
			keyFeatures: schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Features enabled for the RSC account.",
			},
		},
		// The data source is deprecated in favour of the account data source,
		// under both the rubrik and the polaris names. The polaris alias
		// points at the rubrik_account data source, not at this data source,
		// since the replacement is the account data source.
		DeprecationMessage: "use `rubrik_account` instead.",
	}
}

func (d *featuresDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "featuresDataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

func (d *featuresDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "featuresDataSource.Read")

	polarisClient, err := d.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	features, err := core.Wrap(polarisClient.GQL).EnabledFeaturesForAccount(ctx)
	if err != nil {
		res.Diagnostics.AddError("Failed to read account features", err.Error())
		return
	}
	slices.SortFunc(features, func(lhs, rhs core.Feature) int {
		return cmp.Compare(lhs.Name, rhs.Name)
	})

	// The slice is never nil so that no features gives an empty list, not null.
	names := make([]string, 0, len(features))
	hash := sha256.New()
	for _, feature := range features {
		names = append(names, feature.Name)
		hash.Write([]byte(feature.Name))
	}

	state := featuresDataSourceModel{
		ID:       types.StringValue(fmt.Sprintf("%x", hash.Sum(nil))),
		Features: names,
	}
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}
