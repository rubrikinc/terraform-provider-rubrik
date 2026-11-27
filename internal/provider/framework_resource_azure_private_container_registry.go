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

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/pcr"
)

const resourceAzurePrivateContainerRegistryDescription = `
The ´rubrik_azure_private_container_registry´ resource enables the private
container registry (PCR) feature for the RSC customer account. This disables the
standard Rubrik container registry.

~> **Note:** Even though the ´rubrik_azure_private_container_registry´ resource
   ID is an RSC cloud account ID, there can only be a single PCR per RSC
   customer account.

## Exocompute Image Bundles
The following GraphQL query can be used to retrieve information about the image
bundles used by RSC for exocompute:
´´´graphql
query ExotaskImageBundle {
  exotaskImageBundle {
    bundleImages {
      name
      sha
      tag
    }
    bundleVersion
    eksVersion
    repoUrl
  }
}
´´´
The ´repoUrl´ field holds the URL to the RSC container registry from where the
RSC images can be pulled.

The following GraphQL mutation can be used to set the approved bundle version
for the RSC customer account:
´´´graphql
mutation SetBundleApprovalStatus($input: SetBundleApprovalStatusInput!) {
  setBundleApprovalStatus(input: $input)
}
´´´
The input is an object with the following structure:
´´´json
{
  "input": {
    "approvalStatus": "ACCEPTED",
    "bundleVersion": "1.164",
	"bundleMetadata": {
      "eksVersion": "1.29"
    }
  }
}
´´´
Where ´approvalStatus´ can be either ´ACCEPTED´ or ´REJECTED´. ´bundleVersion´
is the the bundle version being approved or rejected. ´eksVersion´ is the
version of the customer's EKS cluster.
`

var (
	_ resource.Resource                = &azurePrivateContainerRegistryResource{}
	_ resource.ResourceWithConfigure   = &azurePrivateContainerRegistryResource{}
	_ resource.ResourceWithIdentity    = &azurePrivateContainerRegistryResource{}
	_ resource.ResourceWithImportState = &azurePrivateContainerRegistryResource{}
	_ resource.ResourceWithMoveState   = &azurePrivateContainerRegistryResource{}
)

type azurePrivateContainerRegistryResource struct {
	client *client
	prefix string
}

type azurePrivateContainerRegistryResourceModel struct {
	ID             types.String `tfsdk:"id"`
	CloudAccountID types.String `tfsdk:"cloud_account_id"`
	AppID          types.String `tfsdk:"app_id"`
	URL            types.String `tfsdk:"url"`
}

// azurePrivateContainerRegistryIdentityModel is the resource identity.
type azurePrivateContainerRegistryIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

func newAzurePrivateContainerRegistryResource() resource.Resource {
	return &azurePrivateContainerRegistryResource{prefix: keyRubrik}
}

func newPolarisAzurePrivateContainerRegistryResource() resource.Resource {
	return &azurePrivateContainerRegistryResource{prefix: keyPolaris}
}

func (r *azurePrivateContainerRegistryResource) Metadata(ctx context.Context, _ resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "azurePrivateContainerRegistryResource.Metadata")

	res.TypeName = r.prefix + "_" + keyAzurePrivateContainerRegistry
}

func (r *azurePrivateContainerRegistryResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "azurePrivateContainerRegistryResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceAzurePrivateContainerRegistryDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "RSC cloud account ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyCloudAccountID: schema.StringAttribute{
				Required:    true,
				Description: "RSC cloud account ID (UUID). Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyAppID: schema.StringAttribute{
				Required:    true,
				Description: "Azure app registration application ID. Also known as the client ID.",
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyURL: schema.StringAttribute{
				Required:    true,
				Description: "URL for customer provided private container registry.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use `rubrik_azure_private_container_registry` instead."
	}
}

func (r *azurePrivateContainerRegistryResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "azurePrivateContainerRegistryResource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "RSC cloud account ID (UUID).",
			},
		},
	}
}

func (r *azurePrivateContainerRegistryResource) Configure(ctx context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	tflog.Trace(ctx, "azurePrivateContainerRegistryResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *azurePrivateContainerRegistryResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "azurePrivateContainerRegistryResource.Create")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	var plan azurePrivateContainerRegistryResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(plan.CloudAccountID.ValueString())
	if err != nil {
		res.Diagnostics.AddAttributeError(path.Root(keyCloudAccountID), "Invalid cloud account ID", err.Error())
		return
	}
	appID, err := uuid.Parse(plan.AppID.ValueString())
	if err != nil {
		res.Diagnostics.AddAttributeError(path.Root(keyAppID), "Invalid app ID", err.Error())
		return
	}

	if err := pcr.Wrap(polarisClient).SetAzureRegistry(ctx, id, appID, plan.URL.ValueString()); err != nil {
		res.Diagnostics.AddError("Failed to set private container registry", err.Error())
		return
	}

	plan.ID = types.StringValue(id.String())
	res.Diagnostics.Append(res.State.Set(ctx, plan)...)
	res.Diagnostics.Append(res.Identity.Set(ctx, azurePrivateContainerRegistryIdentityModel{ID: plan.ID})...)
}

func (r *azurePrivateContainerRegistryResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "azurePrivateContainerRegistryResource.Read")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	var state azurePrivateContainerRegistryResourceModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid resource ID", err.Error())
		return
	}

	// RSC returns an empty registry URL, and no error, when there is no
	// registry registered for the cloud account.
	registry, err := pcr.Wrap(polarisClient).AzureRegistry(ctx, id)
	if err != nil {
		res.Diagnostics.AddError("Failed to read private container registry", err.Error())
		return
	}
	if registry.PCRDetails.RegistryURL == "" {
		res.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(id.String())
	state.CloudAccountID = types.StringValue(id.String())
	state.AppID = types.StringValue(registry.PCRDetails.ImagePullDetails.CustomerAppId)
	state.URL = types.StringValue(registry.PCRDetails.RegistryURL)
	res.Diagnostics.Append(res.State.Set(ctx, state)...)
	res.Diagnostics.Append(res.Identity.Set(ctx, azurePrivateContainerRegistryIdentityModel{ID: state.ID})...)
}

func (r *azurePrivateContainerRegistryResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "azurePrivateContainerRegistryResource.Update")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	var plan azurePrivateContainerRegistryResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(plan.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid resource ID", err.Error())
		return
	}
	appID, err := uuid.Parse(plan.AppID.ValueString())
	if err != nil {
		res.Diagnostics.AddAttributeError(path.Root(keyAppID), "Invalid app ID", err.Error())
		return
	}

	if err := pcr.Wrap(polarisClient).SetAzureRegistry(ctx, id, appID, plan.URL.ValueString()); err != nil {
		res.Diagnostics.AddError("Failed to set private container registry", err.Error())
		return
	}

	res.Diagnostics.Append(res.State.Set(ctx, plan)...)
	res.Diagnostics.Append(res.Identity.Set(ctx, azurePrivateContainerRegistryIdentityModel{ID: plan.ID})...)
}

func (r *azurePrivateContainerRegistryResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "azurePrivateContainerRegistryResource.Delete")

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	var state azurePrivateContainerRegistryResourceModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid resource ID", err.Error())
		return
	}

	if err := pcr.Wrap(polarisClient).RemoveRegistry(ctx, id); err != nil {
		res.Diagnostics.AddError("Failed to remove private container registry", err.Error())
	}
}

func (r *azurePrivateContainerRegistryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "azurePrivateContainerRegistryResource.ImportState")

	// Handles both `import { id = "..." }` and `import { identity = { id = "..." } }`.
	// Read fills in cloud_account_id from the ID.
	resource.ImportStatePassthroughWithIdentity(ctx, path.Root(keyID), path.Root(keyID), req, res)
}
