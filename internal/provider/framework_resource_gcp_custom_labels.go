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

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

var resourceGCPCustomLabelsDescription = `
The ´rubrik_gcp_custom_labels´ resource manages RSC custom GCP labels.
Simplify your cloud resource management by assigning custom labels for easy
identification. These custom labels will be used on all existing and future GCP
projects in your RSC account.

-> **Note:** The newly updated custom labels will be applied to all existing and
   new resources, while the previously applied labels will remain unchanged.

~> **Warning:** When using multiple ´rubrik_gcp_custom_labels´ resources in the
   same RSC account, there is a risk of a race condition when the resources are
   destroyed. This can result in custom labels remaining in RSC even after all
   ´rubrik_gcp_custom_labels´ resources have been destroyed. The race condition
   can be avoided by either managing all custom labels using a single
   ´rubrik_gcp_custom_labels´ resource or by using ´depends_on´ to ensure that
   the resources are destroyed in a serial fashion.

~> **Warning:** The ´override_resource_labels´ field refers to a single global
   value in RSC. So multiple ´rubrik_gcp_custom_labels´ resources with
   different values for the ´override_resource_labels´ field will result in a
   perpetual diff.
`

const gcpCustomLabelsID = "31e3cbd5c7bd25c4de00fdd6635f2d0bf237930e0d6a4e6b1bbf8a4fcccc6c4c"

var (
	_ resource.Resource                = &gcpCustomLabelsResource{}
	_ resource.ResourceWithConfigure   = &gcpCustomLabelsResource{}
	_ resource.ResourceWithImportState = &gcpCustomLabelsResource{}
	_ resource.ResourceWithMoveState   = &gcpCustomLabelsResource{}
)

type gcpCustomLabelsResource struct {
	client *client
	prefix string
}

func newGcpCustomLabelsResource() resource.Resource {
	return &gcpCustomLabelsResource{prefix: keyRubrik}
}

func newPolarisGcpCustomLabelsResource() resource.Resource {
	return &gcpCustomLabelsResource{prefix: keyPolaris}
}

func (r *gcpCustomLabelsResource) Metadata(ctx context.Context, _ resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "gcpCustomLabelsResource.Metadata")

	res.TypeName = r.prefix + "_" + keyGCPCustomLabels
}

func (r *gcpCustomLabelsResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "gcpCustomLabelsResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceGCPCustomLabelsDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "SHA-256 hash of the string \"GCP\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyCustomLabels: schema.MapAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: "Custom labels to add to cloud resources.",
			},
			keyOverrideResourceLabels: schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Should custom labels overwrite existing labels with the same keys. Default value is `true`.",
			},
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use `rubrik_aws_custom_tags` instead."
	}
}

func (r *gcpCustomLabelsResource) Configure(ctx context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	tflog.Trace(ctx, "gcpCustomLabelsResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *gcpCustomLabelsResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "gcpCustomLabelsResource.Create")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	createCustomTagsResource(ctx, polarisClient, core.CloudVendorGCP, req, res)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyID), types.StringValue(gcpCustomLabelsID))...)
}

func (r *gcpCustomLabelsResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "gcpCustomLabelsResource.Read")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	readCustomTagsResource(ctx, polarisClient, core.CloudVendorGCP, req, res)
}

func (r *gcpCustomLabelsResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "gcpCustomLabelsResource.Update")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	updateCustomTagsResource(ctx, polarisClient, core.CloudVendorGCP, req, res)
}

func (r *gcpCustomLabelsResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "gcpCustomLabelsResource.Delete")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	deleteCustomTagsResource(ctx, polarisClient, core.CloudVendorGCP, req, res)
}

func (r *gcpCustomLabelsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "gcpCustomLabelsResource.ImportState")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	importCustomTagsResource(ctx, polarisClient, core.CloudVendorGCP, req, res)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyID), types.StringValue(gcpCustomLabelsID))...)
}

func (r *gcpCustomLabelsResource) MoveState(ctx context.Context) []resource.StateMover {
	tflog.Trace(ctx, "gcpCustomLabelsResource.MoveState")

	return []resource.StateMover{
		moveCustomTagsResourceV0(core.CloudVendorGCP),
	}
}
