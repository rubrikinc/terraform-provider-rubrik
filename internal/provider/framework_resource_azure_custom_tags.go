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

var resourceAzureCustomTagsDescription = `
The ´rubrik_azure_custom_tags´ resource manages RSC custom Azure tags. Simplify
your cloud resource management by assigning custom tags for easy identification.
These custom tags will be used on all existing and future Azure subscriptions in
your RSC account.

-> **Note:** The newly updated custom tags will be applied to all existing and
   new resources, while the previously applied tags will remain unchanged.

~> **Warning:** When using multiple ´rubrik_azure_custom_tags´ resources in the
   same RSC account, there is a risk of a race condition when the resources are
   destroyed. This can result in custom tags remaining in RSC even after all
   ´rubrik_azure_custom_tags´ resources have been destroyed. The race condition
   can be avoided by either managing all custom tags using a single
   ´rubrik_azure_custom_tags´ resource or by using the ´depends_on´ field to
   ensure that the resources are destroyed in a serial fashion.

~> **Warning:** The ´override_resource_tags´ field refers to a single global
   value in RSC. So multiple ´rubrik_azure_custom_tags´ resources with
   different values for the ´override_resource_tags´ field will result in a
   perpetual diff.
`

const azureCustomTagsID = "3140d22d8cb307e2e7ffbae4a07225e09537ce90c32033582f01d979c0ad8f26"

var (
	_ resource.Resource                = &azureCustomTagsResource{}
	_ resource.ResourceWithConfigure   = &azureCustomTagsResource{}
	_ resource.ResourceWithImportState = &azureCustomTagsResource{}
	_ resource.ResourceWithMoveState   = &azureCustomTagsResource{}
)

type azureCustomTagsResource struct {
	client *client
	prefix string
}

func newAzureCustomTagsResource() resource.Resource {
	return &azureCustomTagsResource{prefix: keyRubrik}
}

func newPolarisAzureCustomTagsResource() resource.Resource {
	return &azureCustomTagsResource{prefix: keyPolaris}
}

func (r *azureCustomTagsResource) Metadata(ctx context.Context, _ resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "azureCustomTagsResource.Metadata")

	res.TypeName = r.prefix + "_" + keyAzureCustomTags
}

func (r *azureCustomTagsResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "awsCustomTagsResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceAzureCustomTagsDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "SHA-256 hash of the string \"Azure\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyCustomTags: schema.MapAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: "Custom tags to add to cloud resources.",
			},
			keyOverrideResourceTags: schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Should custom tags overwrite existing tags with the same keys. Default value is `true`.",
			},
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use `rubrik_azure_custom_tags` instead."
	}
}

func (r *azureCustomTagsResource) Configure(ctx context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	tflog.Trace(ctx, "azureCustomTagsResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *azureCustomTagsResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "azureCustomTagsResource.Create")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	createCustomTagsResource(ctx, polarisClient, core.CloudVendorAzure, req, res)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyID), types.StringValue(azureCustomTagsID))...)
}

func (r *azureCustomTagsResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "azureCustomTagsResource.Read")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	readCustomTagsResource(ctx, polarisClient, core.CloudVendorAzure, req, res)
}

func (r *azureCustomTagsResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "azureCustomTagsResource.Update")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	updateCustomTagsResource(ctx, polarisClient, core.CloudVendorAzure, req, res)
}

func (r *azureCustomTagsResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "azureCustomTagsResource.Delete")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	deleteCustomTagsResource(ctx, polarisClient, core.CloudVendorAzure, req, res)
}

func (r *azureCustomTagsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "azureCustomTagsResource.ImportState")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	importCustomTagsResource(ctx, polarisClient, core.CloudVendorAzure, req, res)
	if res.Diagnostics.HasError() {
		return
	}

	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyID), types.StringValue(azureCustomTagsID))...)
}

func (r *azureCustomTagsResource) MoveState(ctx context.Context) []resource.StateMover {
	tflog.Trace(ctx, "azureCustomTagsResource.MoveState")

	return []resource.StateMover{
		moveCustomTagsResourceV0(core.CloudVendorAzure),
	}
}
