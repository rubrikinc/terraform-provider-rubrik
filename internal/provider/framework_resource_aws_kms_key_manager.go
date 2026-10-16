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
	gqlaws "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/aws"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/kms"
)

const resourceAWSKMSKeyManagerDescription = `
The ´rubrik_aws_kms_key_manager´ resource is used to register an AWS KMS key
manager in RSC.

A key manager is a stored connection profile giving RSC access to a customer
owned key management service. RSC takes the credentials from the cloud account,
so no credentials are stored. Its ID is the value archival locations refer to
the key manager by, alongside the ID of the key itself, which comes from AWS.

-> **Note:** The AWS account must be onboarded to RSC with the
   ´CLOUD_NATIVE_UEM_KEY_MANAGEMENT´ feature, and the feature must be connected.

-> **Note:** RSC only supports updating the ´name´, the ´description´ and the
   region endpoints. Changing any other argument replaces the key manager. RSC
   refuses to delete a key manager which still encrypts an archival location.
`

var (
	_ resource.Resource                = &awsKMSKeyManagerResource{}
	_ resource.ResourceWithIdentity    = &awsKMSKeyManagerResource{}
	_ resource.ResourceWithImportState = &awsKMSKeyManagerResource{}
)

type awsKMSKeyManagerResource struct {
	client *client
	prefix string
}

type awsResourceKMSKeyManagerModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Description       types.String `tfsdk:"description"`
	CloudAccountID    types.String `tfsdk:"cloud_account_id"`
	Region            types.String `tfsdk:"region"`
	STSRegionEndpoint types.String `tfsdk:"sts_region_endpoint"`
	KMSRegionEndpoint types.String `tfsdk:"kms_region_endpoint"`
}

type awsKMSKeyManagerIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

func newAWSKMSKeyManagerResource() resource.Resource {
	return &awsKMSKeyManagerResource{prefix: keyRubrik}
}

func newPolarisAWSKMSKeyManagerResource() resource.Resource {
	return &awsKMSKeyManagerResource{prefix: keyPolaris}
}

func (r *awsKMSKeyManagerResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "awsKMSKeyManagerResource.Metadata")

	res.TypeName = r.prefix + "_" + keyAWSKMSKeyManager
}

func (r *awsKMSKeyManagerResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "awsKMSKeyManagerResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceAWSKMSKeyManagerDescription),
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
				Description: "RSC cloud account ID of the AWS account holding the KMS keys. Changing this forces " +
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
				Description: "AWS region of the KMS keys, e.g. `us-east-2`. Changing this forces a new resource " +
					"to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keySTSRegionEndpoint: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "AWS STS regional endpoint RSC uses to assume the role of the cloud account.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyKMSRegionEndpoint: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "AWS KMS regional endpoint RSC uses to reach the key management service.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_aws_kms_key_manager` resource instead."
	}
}

func (r *awsKMSKeyManagerResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "awsKMSKeyManagerResource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Key manager ID (UUID).",
			},
		},
	}
}

func (r *awsKMSKeyManagerResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "awsKMSKeyManagerResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *awsKMSKeyManagerResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "awsKMSKeyManagerResource.Create")

	var plan awsResourceKMSKeyManagerModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	region := gqlaws.RegionFromName(plan.Region.ValueString())
	if region == gqlaws.RegionUnknown {
		res.Diagnostics.AddAttributeError(path.Root(keyRegion), "Invalid region",
			"The region is not a known AWS region name: "+plan.Region.ValueString())
		return
	}

	keyManagerID, err := kms.Wrap(polarisClient).CreateAWSKeyManager(ctx, gqlkms.CreateAWSKeyManagerParams{
		CloudAccountID:    plan.CloudAccountID.ValueString(),
		Name:              plan.Name.ValueString(),
		Region:            region,
		Description:       plan.Description.ValueString(),
		STSRegionEndpoint: plan.STSRegionEndpoint.ValueString(),
		KMSRegionEndpoint: plan.KMSRegionEndpoint.ValueString(),
	})
	if err != nil {
		res.Diagnostics.AddError("Failed to create AWS KMS key manager", err.Error())
		return
	}

	// The key manager exists in RSC from here on. Saving its ID before reading
	// it back makes Terraform keep track of it if the read fails.
	res.Diagnostics.Append(res.State.SetAttribute(ctx, path.Root(keyID), keyManagerID.String())...)
	res.Diagnostics.Append(res.Identity.SetAttribute(ctx, path.Root(keyID), keyManagerID.String())...)
	if res.Diagnostics.HasError() {
		return
	}

	keyManager, err := kms.Wrap(polarisClient).AWSKeyManagerByID(ctx, keyManagerID)
	if err != nil {
		res.Diagnostics.AddError("Failed to read AWS KMS key manager", err.Error())
		return
	}

	updateAWSKMSKeyManagerModel(&plan, keyManager)

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := awsKMSKeyManagerIdentityModel{ID: plan.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *awsKMSKeyManagerResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "awsKMSKeyManagerResource.Read")

	var state awsResourceKMSKeyManagerModel
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

	keyManager, err := kms.Wrap(polarisClient).AWSKeyManagerByID(ctx, keyManagerID)
	if errors.Is(err, graphql.ErrNotFound) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read AWS KMS key manager", err.Error())
		return
	}

	updateAWSKMSKeyManagerModel(&state, keyManager)

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := awsKMSKeyManagerIdentityModel{ID: state.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *awsKMSKeyManagerResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "awsKMSKeyManagerResource.Update")

	var plan awsResourceKMSKeyManagerModel
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

	err = kms.Wrap(polarisClient).UpdateAWSKeyManager(ctx, keyManagerID, gqlkms.UpdateAWSKeyManagerParams{
		Name:              plan.Name.ValueString(),
		Description:       plan.Description.ValueString(),
		STSRegionEndpoint: plan.STSRegionEndpoint.ValueString(),
		KMSRegionEndpoint: plan.KMSRegionEndpoint.ValueString(),
	})
	if err != nil {
		res.Diagnostics.AddError("Failed to update AWS KMS key manager", err.Error())
		return
	}

	keyManager, err := kms.Wrap(polarisClient).AWSKeyManagerByID(ctx, keyManagerID)
	if err != nil {
		res.Diagnostics.AddError("Failed to read AWS KMS key manager", err.Error())
		return
	}

	updateAWSKMSKeyManagerModel(&plan, keyManager)

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
}

func (r *awsKMSKeyManagerResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "awsKMSKeyManagerResource.Delete")

	var state awsResourceKMSKeyManagerModel
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
		res.Diagnostics.AddError("Failed to delete AWS KMS key manager", err.Error())
		return
	}
}

func (r *awsKMSKeyManagerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "awsKMSKeyManagerResource.ImportState")

	resource.ImportStatePassthroughWithIdentity(ctx, path.Root(keyID), path.Root(keyID), req, res)
}

// updateAWSKMSKeyManagerModel updates the model with the values read from RSC.
func updateAWSKMSKeyManagerModel(m *awsResourceKMSKeyManagerModel, keyManager gqlkms.KeyManager) {
	m.ID = types.StringValue(keyManager.KeyVaultInfo.ID.String())
	m.Name = types.StringValue(keyManager.Name)
	m.Description = types.StringValue(keyManager.Description)
	m.CloudAccountID = types.StringValue(keyManager.KeyVaultInfo.CloudAccountID)
	m.Region = types.StringValue(keyManager.KeyVaultInfo.AWSRegion.Name())
	m.STSRegionEndpoint = types.StringValue(keyManager.KeyVaultInfo.STSRegionEndpoint)
	m.KMSRegionEndpoint = types.StringValue(keyManager.KeyVaultInfo.KMSRegionEndpoint)
}
