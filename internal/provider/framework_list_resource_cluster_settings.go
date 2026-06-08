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

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/cluster"
	gqlcluster "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/cluster"
)

const listResourceClusterSettingsDescription = `
The ´rubrik_cluster_settings´ list resource lists the upgrade and download
state of Rubrik clusters registered with RSC.
`

var (
	_ list.ListResource              = &clusterSettingsListResource{}
	_ list.ListResourceWithConfigure = &clusterSettingsListResource{}
)

type clusterSettingsListResource struct {
	client *client
}

type clusterSettingsListConfigModel struct {
	Name    types.String `tfsdk:"name"`
	Version types.String `tfsdk:"version"`
}

func newClusterSettingsListResource() list.ListResource {
	return &clusterSettingsListResource{}
}

func (r *clusterSettingsListResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "clusterSettingsListResource.Metadata")

	res.TypeName = keyRubrik + "_" + keyClusterSettings
}

func (r *clusterSettingsListResource) ListResourceConfigSchema(ctx context.Context, _ list.ListResourceSchemaRequest, res *list.ListResourceSchemaResponse) {
	tflog.Trace(ctx, "clusterSettingsListResource.ListResourceConfigSchema")

	res.Schema = listschema.Schema{
		Description: description(listResourceClusterSettingsDescription),
		Attributes: map[string]listschema.Attribute{
			keyName: listschema.StringAttribute{
				Optional:    true,
				Description: "Filter clusters by name (exact match).",
			},
			keyVersion: listschema.StringAttribute{
				Optional:    true,
				Description: "Filter clusters by installed CDM version (exact match).",
			},
		},
	}
}

func (r *clusterSettingsListResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "clusterSettingsListResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *clusterSettingsListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	tflog.Trace(ctx, "clusterSettingsListResource.List")

	var config clusterSettingsListConfigModel
	diags := req.Config.Get(ctx, &config)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		diags.AddError("RSC client error", err.Error())
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	filter := &gqlcluster.CDMInfoFilter{}
	if v := config.Name.ValueString(); v != "" {
		filter.Name = []string{v}
	}
	if v := config.Version.ValueString(); v != "" {
		filter.InstalledVersion = []string{v}
	}

	clusters, err := cluster.Wrap(polarisClient).ListClusterUpgrades(ctx, filter, "", "")
	if err != nil {
		diags.AddError("Failed to list cluster settings", err.Error())
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	// The resource model carries a timeouts.Value whose zero value is a null
	// object with no attribute types. Setting it against the resource schema
	// would fail type conversion, so derive a correctly-typed null from the
	// schema's timeouts attribute.
	nullTimeouts := timeouts.Value{Object: types.ObjectNull(nil)}
	if obj, ok := req.ResourceSchema.Type().(types.ObjectType); ok {
		if t, ok := obj.AttrTypes[keyTimeouts].(types.ObjectType); ok {
			nullTimeouts = timeouts.Value{Object: types.ObjectNull(t.AttrTypes)}
		}
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for i, details := range clusters {
			if int64(i) >= req.Limit {
				return
			}

			result := req.NewListResult(ctx)
			result.DisplayName = details.Name

			identity := clusterSettingsIdentityModel{
				ClusterID: types.StringValue(details.ID.String()),
			}
			result.Diagnostics.Append(result.Identity.Set(ctx, identity)...)
			if result.Diagnostics.HasError() {
				push(result)
				return
			}

			if req.IncludeResource {
				model := clusterSettingsResourceModel{
					ClusterID: types.StringValue(details.ID.String()),
					Timeouts:  nullTimeouts,
				}
				model.applyComputed(details)
				result.Diagnostics.Append(result.Resource.Set(ctx, model)...)
				if result.Diagnostics.HasError() {
					push(result)
					return
				}
			}

			if !push(result) {
				return
			}
		}
	}
}
