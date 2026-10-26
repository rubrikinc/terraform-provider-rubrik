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
	"regexp"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core/secret"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/pcr"
)

const resourceGCPPrivateContainerRegistryDescription = `
The ´rubrik_gcp_private_container_registry´ resource enables the private
container registry (PCR) feature for GCP Exocompute in the RSC customer account.
This disables the standard Rubrik container registry for GCP Exocompute. Instead
of pulling the Exocompute images from Rubrik, Exocompute pulls them from a
customer owned Google Artifact Registry repository.

~> **Note:** Even though the ´rubrik_gcp_private_container_registry´ resource ID
   is an RSC cloud account ID, there can only be a single PCR per RSC customer
   account.

## Prerequisites
  * The GCP project must have been onboarded using the ´rubrik_gcp_project´
    resource with the ´credentials´ field set. Projects onboarded using OAuth
    are rejected by RSC.
  * Terraform v1.11.0 or later is required, the ´service_account_ownership_token´
    field is a write-only field.
  * The private container registry feature for GCP must be enabled for the RSC
    account.
  * The registry must be a Google Artifact Registry repository in the GCP project
    running Exocompute. A registry hosted elsewhere can be mirrored into Artifact
    Registry using a remote repository. RSC does not verify this when the registry
    is registered.
  * The service account RSC uses for the GCP project, either the one in the
    ´credentials´ of the ´rubrik_gcp_project´ resource or the account-wide one
    managed by the ´rubrik_gcp_service_account´ resource, must have the
    ´roles/artifactregistry.reader´ role on the customer owned Artifact Registry
    repository, otherwise the GKE nodes cannot pull the images.

## Service Account Ownership Token
RSC grants the ´roles/artifactregistry.reader´ role on the Rubrik Artifact
Registry, where the Exocompute images are published, to the service account given
by ´service_account_email´. To prove that the caller owns the service account, a
Google OIDC identity token issued for the service account must be passed in
´service_account_ownership_token´. The token must meet the following
requirements:
  * The audience must be ´https://rubrik.com/exocompute/pcr/gcp´.
  * The ´email´ claim must be present and equal to ´service_account_email´.
  * The token must be less than one hour old.

The token is verified by RSC and then discarded, it is never stored by RSC or in
the Terraform state. The token must be fresh every time the resource is created or
updated. The ´google_service_account_id_token´ ephemeral resource of the Google
provider can be used to create the token, see the example below. Ephemeral values
are never stored in the plan or the Terraform state and are created again on every
run, which keeps the token fresh. Set ´include_email´ to ´true´ when the ephemeral
resource impersonates a service account, otherwise Google leaves out the ´email´
claim. For manual use, the ´gcloud auth print-identity-token´ command can be used
with the ´--audiences´ and ´--include-email´ flags.

~> **Warning:** Destroying the resource requires the PCR feature for GCP to remain
   enabled for the RSC account. If the feature is disabled, RSC rejects the
   request to remove the registry and the destroy fails.

## Exocompute Image Bundles
The following GraphQL query can be used to retrieve information about the image
bundles used by RSC for GCP Exocompute:
´´´graphql
query ExotaskImageBundle {
  exotaskImageBundle {
    gcpImages {
      repoUrl
      bundleVersion
      bundleImages {
        name
        sha
        tag
      }
    }
  }
}
´´´
The ´repoUrl´ field holds the URL to the RSC Artifact Registry from where the RSC
images can be pulled. The images must be pulled using the service account given
by ´service_account_email´. To copy the images into the customer owned Artifact
Registry:
  1. Authenticate as the service account, e.g. using
     ´gcloud auth activate-service-account´.
  2. Configure Docker to authenticate to the registry host of ´repoUrl´, e.g.
     ´gcloud auth configure-docker us-docker.pkg.dev´.
  3. Pull each image listed in ´bundleImages´. An image with a ´sha´ is pulled by
     digest using ´docker pull <repoUrl>/<name>@sha256:<sha>´, and an image with a
     ´tag´ using ´docker pull <repoUrl>/<name>:<tag>´.
  4. Tag each image with ´<url>/<name>:<bundleVersion>´, where ´url´ is the value
     of the ´url´ field of this resource, and push it.

The following GraphQL mutation can be used to set the approved bundle version for
the RSC customer account once all images have been pushed:
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
    "bundleVersion": "1.164"
  }
}
´´´
Where ´approvalStatus´ can be either ´ACCEPTED´ or ´REJECTED´ and ´bundleVersion´
is the bundle version being approved or rejected.
`

var (
	// Matches the Artifact Registry repository URLs RSC accepts, e.g.
	// us-docker.pkg.dev/my-project/my-repo. RSC does not validate the URL when
	// the registry is set, so it's validated here.
	gcpArtifactRegistryURLRegexp = regexp.MustCompile(
		`^([a-z]+(?:-[a-z]+[0-9]+)?)-docker\.pkg\.dev/([a-z][a-z0-9-]{4,28}[a-z0-9])/([a-z](?:[a-z0-9-]{0,61}[a-z0-9])?)$`)

	// Matches the service account emails RSC accepts.
	gcpServiceAccountEmailRegexp = regexp.MustCompile(
		`^([a-z][a-z0-9-]{5,29})@([a-z][a-z0-9-]{4,28}[a-z0-9])\.iam\.gserviceaccount\.com$`)
)

var (
	_ resource.Resource                = &gcpPrivateContainerRegistryResource{}
	_ resource.ResourceWithConfigure   = &gcpPrivateContainerRegistryResource{}
	_ resource.ResourceWithImportState = &gcpPrivateContainerRegistryResource{}
	_ resource.ResourceWithMoveState   = &gcpPrivateContainerRegistryResource{}
)

type gcpPrivateContainerRegistryResource struct {
	client *client
	prefix string
}

type gcpPrivateContainerRegistryResourceModel struct {
	ID                           types.String `tfsdk:"id"`
	CloudAccountID               types.String `tfsdk:"cloud_account_id"`
	ServiceAccountEmail          types.String `tfsdk:"service_account_email"`
	ServiceAccountOwnershipToken types.String `tfsdk:"service_account_ownership_token"`
	URL                          types.String `tfsdk:"url"`
}

func newGcpPrivateContainerRegistryResource() resource.Resource {
	return &gcpPrivateContainerRegistryResource{prefix: keyRubrik}
}

func newPolarisGcpPrivateContainerRegistryResource() resource.Resource {
	return &gcpPrivateContainerRegistryResource{prefix: keyPolaris}
}

func (r *gcpPrivateContainerRegistryResource) Metadata(ctx context.Context, _ resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "gcpPrivateContainerRegistryResource.Metadata")

	res.TypeName = r.prefix + "_" + keyGCPPrivateContainerRegistry
}

func (r *gcpPrivateContainerRegistryResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "gcpPrivateContainerRegistryResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceGCPPrivateContainerRegistryDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "RSC cloud account ID (UUID). Same value as `cloud_account_id`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyCloudAccountID: schema.StringAttribute{
				Required: true,
				Description: "RSC cloud account ID (UUID) of the GCP project running Exocompute. Changing this " +
					"forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyServiceAccountEmail: schema.StringAttribute{
				Required: true,
				Description: "Email of the GCP service account used to pull the Exocompute images from the Rubrik " +
					"Artifact Registry.",
				Validators: []validator.String{
					stringvalidator.RegexMatches(gcpServiceAccountEmailRegexp,
						"must be a service account email, e.g. name@project.iam.gserviceaccount.com"),
				},
			},
			keyServiceAccountOwnershipToken: schema.StringAttribute{
				Required:  true,
				Sensitive: true,
				WriteOnly: true,
				Description: "Google OIDC identity token proving ownership of the service account. The audience " +
					"must be `https://rubrik.com/exocompute/pcr/gcp`, the `email` claim must be present, and the " +
					"token must be less than one hour old. The token is verified by RSC and then discarded, it is " +
					"never stored. Requires Terraform v1.11.0 or later.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyURL: schema.StringAttribute{
				Required: true,
				Description: "URL of the customer owned Artifact Registry repository, on the form " +
					"`<location>-docker.pkg.dev/<project>/<repository>`.",
				Validators: []validator.String{
					stringvalidator.RegexMatches(gcpArtifactRegistryURLRegexp,
						"must be an Artifact Registry repository URL, e.g. us-docker.pkg.dev/my-project/my-repo"),
				},
			},
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use `rubrik_gcp_private_container_registry` instead."
	}
}

func (r *gcpPrivateContainerRegistryResource) Configure(ctx context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	tflog.Trace(ctx, "gcpPrivateContainerRegistryResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *gcpPrivateContainerRegistryResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "gcpPrivateContainerRegistryResource.Create")

	var plan gcpPrivateContainerRegistryResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}
	// Write-only values are always null in the plan, read the token from the
	// configuration.
	var token types.String
	res.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(keyServiceAccountOwnershipToken), &token)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}
	id, err := uuid.Parse(plan.CloudAccountID.ValueString())
	if err != nil {
		res.Diagnostics.AddAttributeError(path.Root(keyCloudAccountID), "Invalid cloud account ID", err.Error())
		return
	}

	err = pcr.Wrap(polarisClient).SetGCPRegistry(ctx, id, plan.ServiceAccountEmail.ValueString(),
		secret.String(token.ValueString()), plan.URL.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Failed to set private container registry", err.Error())
		return
	}

	plan.ID = types.StringValue(id.String())
	res.Diagnostics.Append(res.State.Set(ctx, plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	// Read back the registry to make sure the state reflects what RSC stored.
	state, found := r.readRegistry(ctx, polarisClient, plan, &res.Diagnostics)
	if res.Diagnostics.HasError() {
		return
	}
	if !found {
		res.Diagnostics.AddError("Private container registry not found",
			"RSC did not return the private container registry after it was set")
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, state)...)
}

func (r *gcpPrivateContainerRegistryResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "gcpPrivateContainerRegistryResource.Read")

	var state gcpPrivateContainerRegistryResourceModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	state, found := r.readRegistry(ctx, polarisClient, state, &res.Diagnostics)
	if res.Diagnostics.HasError() {
		return
	}
	if !found {
		res.State.RemoveResource(ctx)
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, state)...)
}

func (r *gcpPrivateContainerRegistryResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "gcpPrivateContainerRegistryResource.Update")

	var plan gcpPrivateContainerRegistryResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}
	var token types.String
	res.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(keyServiceAccountOwnershipToken), &token)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}
	id, err := uuid.Parse(plan.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid resource ID", err.Error())
		return
	}

	err = pcr.Wrap(polarisClient).SetGCPRegistry(ctx, id, plan.ServiceAccountEmail.ValueString(),
		secret.String(token.ValueString()), plan.URL.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Failed to set private container registry", err.Error())
		return
	}

	state, found := r.readRegistry(ctx, polarisClient, plan, &res.Diagnostics)
	if res.Diagnostics.HasError() {
		return
	}
	if !found {
		res.Diagnostics.AddError("Private container registry not found",
			"RSC did not return the private container registry after it was set")
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, state)...)
}

func (r *gcpPrivateContainerRegistryResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "gcpPrivateContainerRegistryResource.Delete")

	var state gcpPrivateContainerRegistryResourceModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
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

func (r *gcpPrivateContainerRegistryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "gcpPrivateContainerRegistryResource.ImportState")

	id, err := uuid.Parse(req.ID)
	if err != nil {
		res.Diagnostics.AddError("Invalid import ID", "expected an RSC cloud account ID (UUID): "+err.Error())
		return
	}

	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyID), id.String())...)
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyCloudAccountID), id.String())...)
}

// readRegistry reads the private container registry for the cloud account in
// the model from RSC and returns the model updated with the values read. RSC
// returns an empty registry URL, and no error, when there is no registry
// registered for the cloud account. In that case false is returned.
func (r *gcpPrivateContainerRegistryResource) readRegistry(ctx context.Context, polarisClient *polaris.Client, model gcpPrivateContainerRegistryResourceModel, diags *diag.Diagnostics) (gcpPrivateContainerRegistryResourceModel, bool) {
	id, err := uuid.Parse(model.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid resource ID", err.Error())
		return model, false
	}

	registry, err := pcr.Wrap(polarisClient).GCPRegistry(ctx, id)
	if err != nil {
		diags.AddError("Failed to read private container registry", err.Error())
		return model, false
	}
	if registry.PCRDetails.RegistryURL == "" {
		return model, false
	}

	model.CloudAccountID = types.StringValue(id.String())
	model.ServiceAccountEmail = types.StringValue(registry.PCRDetails.ImagePullDetails.ServiceAccountEmail)
	model.URL = types.StringValue(registry.PCRDetails.RegistryURL)
	return model, true
}
