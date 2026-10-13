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

	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/archival"
)

const listResourceAWSRCVArchivalLocationDescription = `
The ´rubrik_aws_rcv_archival_location´ list resource lists RCV AWS archival
locations in RSC.
`

var (
	_ list.ListResource              = &awsRCVArchivalLocationListResource{}
	_ list.ListResourceWithConfigure = &awsRCVArchivalLocationListResource{}
)

type awsRCVArchivalLocationListResource struct {
	client *client
	prefix string
}

type awsRCVArchivalLocationListConfigModel struct {
	Name types.String `tfsdk:"name"`
}

func newAWSRCVArchivalLocationListResource() list.ListResource {
	return &awsRCVArchivalLocationListResource{prefix: keyRubrik}
}

func newPolarisAWSRCVArchivalLocationListResource() list.ListResource {
	return &awsRCVArchivalLocationListResource{prefix: keyPolaris}
}

func (r *awsRCVArchivalLocationListResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "awsRCVArchivalLocationListResource.Metadata")

	res.TypeName = r.prefix + "_" + keyAWSRCVArchivalLocation
}

func (r *awsRCVArchivalLocationListResource) ListResourceConfigSchema(ctx context.Context, _ list.ListResourceSchemaRequest, res *list.ListResourceSchemaResponse) {
	tflog.Trace(ctx, "awsRCVArchivalLocationListResource.ListResourceConfigSchema")

	res.Schema = listschema.Schema{
		Description: description(listResourceAWSRCVArchivalLocationDescription),
		Attributes: map[string]listschema.Attribute{
			keyName: listschema.StringAttribute{
				Optional:    true,
				Description: "Filter archival locations by name. Matches names starting with the given value.",
			},
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_aws_rcv_archival_location` list resource instead."
	}
}

func (r *awsRCVArchivalLocationListResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "awsRCVArchivalLocationListResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *awsRCVArchivalLocationListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	tflog.Trace(ctx, "awsRCVArchivalLocationListResource.List")

	var config awsRCVArchivalLocationListConfigModel
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

	targetMappings, err := archival.Wrap(polarisClient).RCVAWSTargetMappings(ctx, config.Name.ValueString())
	if err != nil {
		diags.AddError("Failed to list RCV AWS archival locations", err.Error())
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for i, targetMapping := range targetMappings {
			if int64(i) >= req.Limit {
				return
			}

			result := req.NewListResult(ctx)
			result.DisplayName = targetMapping.Name

			identity := awsRCVArchivalLocationIdentityModel{
				ID: types.StringValue(targetMapping.ID.String()),
			}
			result.Diagnostics.Append(result.Identity.Set(ctx, identity)...)
			if result.Diagnostics.HasError() {
				push(result)
				return
			}

			if req.IncludeResource {
				model := newAWSRCVArchivalLocationModel(targetMapping)
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
