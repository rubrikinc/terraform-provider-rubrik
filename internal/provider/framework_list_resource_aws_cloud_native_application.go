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
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/cloudapp"
)

const listResourceAWSCloudNativeApplicationDescription = `
The ´rubrik_aws_cloud_native_application´ list resource lists the AWS cloud native applications in RSC, shown as
Cloud Applications in the RSC UI.
`

var (
	_ list.ListResource              = &awsCloudNativeApplicationListResource{}
	_ list.ListResourceWithConfigure = &awsCloudNativeApplicationListResource{}
)

type awsCloudNativeApplicationListResource struct {
	client *client
}

// awsCloudNativeApplicationListConfigModel holds the optional filters of the
// list block.
type awsCloudNativeApplicationListConfigModel struct {
	Name types.String `tfsdk:"name"`
}

func newAWSCloudNativeApplicationListResource() list.ListResource {
	return &awsCloudNativeApplicationListResource{}
}

func (r *awsCloudNativeApplicationListResource) Metadata(ctx context.Context, _ resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "awsCloudNativeApplicationListResource.Metadata")

	res.TypeName = keyRubrik + "_" + keyAWSCloudNativeApplication
}

func (r *awsCloudNativeApplicationListResource) ListResourceConfigSchema(ctx context.Context, _ list.ListResourceSchemaRequest, res *list.ListResourceSchemaResponse) {
	tflog.Trace(ctx, "awsCloudNativeApplicationListResource.ListResourceConfigSchema")

	res.Schema = listschema.Schema{
		Description: description(listResourceAWSCloudNativeApplicationDescription),
		Attributes: map[string]listschema.Attribute{
			keyName: listschema.StringAttribute{
				Optional:    true,
				Description: "Filter applications by name. Matches names containing the given value.",
			},
		},
	}
}

func (r *awsCloudNativeApplicationListResource) Configure(ctx context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	tflog.Trace(ctx, "awsCloudNativeApplicationListResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *awsCloudNativeApplicationListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	tflog.Trace(ctx, "awsCloudNativeApplicationListResource.List")

	var config awsCloudNativeApplicationListConfigModel
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

	api := cloudapp.Wrap(polarisClient)
	refs, err := api.ListAWSApplications(ctx, config.Name.ValueString())
	if err != nil {
		diags.AddError("Failed to list cloud native applications", err.Error())
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	// The resource model carries a timeouts.Value whose zero value is a null
	// object with no attribute types. Setting it against the resource schema
	// would fail type conversion, so derive a correctly-typed null from the
	// schema's timeouts attribute.
	nullTimeouts := timeouts.Value{Object: types.ObjectNull(nil)}
	if obj, ok := req.ResourceSchema.Type().(types.ObjectType); ok {
		if t, ok := obj.AttrTypes[keyTimeouts].(timeouts.Type); ok {
			nullTimeouts = timeouts.Value{Object: types.ObjectNull(t.AttrTypes)}
		}
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for i, ref := range refs {
			if int64(i) >= req.Limit {
				return
			}

			result := req.NewListResult(ctx)
			result.DisplayName = ref.Name

			identity := awsCloudNativeApplicationIdentityModel{ID: types.StringValue(ref.ID.String())}
			result.Diagnostics.Append(result.Identity.Set(ctx, identity)...)
			if result.Diagnostics.HasError() {
				push(result)
				return
			}

			if req.IncludeResource {
				// The list doesn't always hold the resource lists while an
				// application refreshes, so read each application in full.
				app, err := api.ApplicationByID(ctx, ref.ID)
				if err != nil {
					result.Diagnostics.AddError("Failed to read cloud native application", err.Error())
					push(result)
					return
				}
				model := awsCloudNativeApplicationModel{
					MatchingTags:         types.MapNull(types.StringType),
					ResourceARNs:         types.SetNull(types.StringType),
					ExcludedResourceARNs: types.SetNull(types.StringType),
					Timeouts:             nullTimeouts,
				}
				result.Diagnostics.Append(model.fromAWSCloudNativeApplication(app)...)
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
