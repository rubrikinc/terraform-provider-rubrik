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
	"errors"

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
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	gqlkms "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/kms"
	gqlgcp "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/gcp"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/kms"
)

const resourceGCPKMSKeyManagerDescription = `
The ´rubrik_gcp_kms_key_manager´ resource is used to register a GCP Cloud KMS
key manager in RSC.

A key manager is a stored connection profile giving RSC access to a customer
owned key management service. RSC takes the credentials from the cloud account,
so no credentials are stored. Its ID is the value archival locations refer to
the key manager by, alongside the name and version of the key itself, which come
from GCP.

-> **Note:** The GCP project must be onboarded to RSC with the
   ´CLOUD_NATIVE_UEM_KEY_MANAGEMENT´ feature, and the feature must be connected.

-> **Note:** RSC only supports updating the ´name´ and the ´description´.
   Changing any other argument replaces the key manager. RSC refuses to delete a
   key manager which still encrypts an archival location.
`

var (
	_ resource.Resource                = &gcpKMSKeyManagerResource{}
	_ resource.ResourceWithIdentity    = &gcpKMSKeyManagerResource{}
	_ resource.ResourceWithImportState = &gcpKMSKeyManagerResource{}
)

type gcpKMSKeyManagerResource struct {
	client *client
	prefix string
}

type gcpResourceKMSKeyManagerModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	CloudAccountID types.String `tfsdk:"cloud_account_id"`
	Region         types.String `tfsdk:"region"`
	KeyRing        types.String `tfsdk:"key_ring"`
}

type gcpKMSKeyManagerIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

func newGCPKMSKeyManagerResource() resource.Resource {
	return &gcpKMSKeyManagerResource{prefix: keyRubrik}
}

func newPolarisGCPKMSKeyManagerResource() resource.Resource {
	return &gcpKMSKeyManagerResource{prefix: keyPolaris}
}

func (r *gcpKMSKeyManagerResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "gcpKMSKeyManagerResource.Metadata")

	res.TypeName = r.prefix + "_" + keyGCPKMSKeyManager
}

func (r *gcpKMSKeyManagerResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "gcpKMSKeyManagerResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceGCPKMSKeyManagerDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "RSC key manager ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyName: schema.StringAttribute{
				Required:    true,
				Description: "Key manager name. Key manager names are unique across all key managers.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyDescription: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Key manager description.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyCloudAccountID: schema.StringAttribute{
				Required: true,
				Description: "RSC cloud account ID of the GCP project holding the key ring. Changing this forces " +
					"a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyRegion: schema.StringAttribute{
				Required: true,
				Description: "Cloud KMS location of the key ring, e.g. `us-west1`. Changing this forces a new " +
					"resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyKeyRing: schema.StringAttribute{
				Required: true,
				Description: "Short name of the Cloud KMS key ring, not the fully qualified resource path. " +
					"Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_gcp_kms_key_manager` resource instead."
	}
}

func (r *gcpKMSKeyManagerResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "gcpKMSKeyManagerResource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Key manager ID (UUID).",
			},
		},
	}
}

func (r *gcpKMSKeyManagerResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "gcpKMSKeyManagerResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *gcpKMSKeyManagerResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "gcpKMSKeyManagerResource.Create")

	var plan gcpResourceKMSKeyManagerModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	region := gqlgcp.RegionFromName(plan.Region.ValueString())
	if region == gqlgcp.RegionUnknown || region.ToKMSLocationEnumPtr() == nil {
		res.Diagnostics.AddAttributeError(path.Root(keyRegion), "Invalid region",
			"The region is not a known GCP Cloud KMS location: "+plan.Region.ValueString())
		return
	}

	keyManagerID, err := kms.Wrap(polarisClient).CreateGCPKeyManager(ctx, gqlkms.CreateGCPKeyManagerParams{
		CloudAccountID: plan.CloudAccountID.ValueString(),
		Name:           plan.Name.ValueString(),
		KeyRing:        plan.KeyRing.ValueString(),
		Location:       region.ToKMSLocationEnum(),
		Description:    plan.Description.ValueString(),
	})
	if err != nil {
		res.Diagnostics.AddError("Failed to create GCP key manager", err.Error())
		return
	}

	// The key manager exists in RSC from here on. Saving its ID before reading
	// it back makes Terraform keep track of it if the read fails.
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyID), keyManagerID.String())...)
	res.Diagnostics.Append(res.Identity.SetAttribute(ctx, path.Root(keyID), keyManagerID.String())...)
	if res.Diagnostics.HasError() {
		return
	}

	keyManager, err := kms.Wrap(polarisClient).GCPKeyManagerByID(ctx, keyManagerID)
	if err != nil {
		res.Diagnostics.AddError("Failed to read GCP key manager", err.Error())
		return
	}

	updateGCPKMSKeyManagerModel(&plan, keyManager)

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := gcpKMSKeyManagerIdentityModel{ID: plan.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *gcpKMSKeyManagerResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "gcpKMSKeyManagerResource.Read")

	var state gcpResourceKMSKeyManagerModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	keyManagerID, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid key manager ID", err.Error())
		return
	}

	keyManager, err := kms.Wrap(polarisClient).GCPKeyManagerByID(ctx, keyManagerID)
	if errors.Is(err, graphql.ErrNotFound) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read GCP key manager", err.Error())
		return
	}

	updateGCPKMSKeyManagerModel(&state, keyManager)

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := gcpKMSKeyManagerIdentityModel{ID: state.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *gcpKMSKeyManagerResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "gcpKMSKeyManagerResource.Update")

	var plan gcpResourceKMSKeyManagerModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	keyManagerID, err := uuid.Parse(plan.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid key manager ID", err.Error())
		return
	}

	err = kms.Wrap(polarisClient).UpdateGCPKeyManager(ctx, keyManagerID, gqlkms.UpdateGCPKeyManagerParams{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
	})
	if err != nil {
		res.Diagnostics.AddError("Failed to update GCP key manager", err.Error())
		return
	}

	keyManager, err := kms.Wrap(polarisClient).GCPKeyManagerByID(ctx, keyManagerID)
	if err != nil {
		res.Diagnostics.AddError("Failed to read GCP key manager", err.Error())
		return
	}

	updateGCPKMSKeyManagerModel(&plan, keyManager)

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
}

func (r *gcpKMSKeyManagerResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "gcpKMSKeyManagerResource.Delete")

	var state gcpResourceKMSKeyManagerModel
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	keyManagerID, err := uuid.Parse(state.ID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("Invalid key manager ID", err.Error())
		return
	}

	err = kms.Wrap(polarisClient).DeleteKeyManager(ctx, keyManagerID)
	if errors.Is(err, graphql.ErrNotFound) {
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to delete GCP key manager", err.Error())
		return
	}
}

func (r *gcpKMSKeyManagerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "gcpKMSKeyManagerResource.ImportState")

	resource.ImportStatePassthroughWithIdentity(ctx, path.Root(keyID), path.Root(keyID), req, res)
}

// updateGCPKMSKeyManagerModel updates the model with the values read from RSC.
func updateGCPKMSKeyManagerModel(m *gcpResourceKMSKeyManagerModel, keyManager gqlkms.KeyManager) {
	m.ID = types.StringValue(keyManager.KeyVaultInfo.ID.String())
	m.Name = types.StringValue(keyManager.Name)
	m.Description = types.StringValue(keyManager.Description)
	m.CloudAccountID = types.StringValue(keyManager.KeyVaultInfo.CloudAccountID)
	m.Region = types.StringValue(keyManager.KeyVaultInfo.GCPLocation.Name())
	m.KeyRing = types.StringValue(keyManager.KeyVaultInfo.KeyRing)
}
