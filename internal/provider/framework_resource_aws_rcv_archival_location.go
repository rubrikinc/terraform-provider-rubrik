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
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/archival"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	gqlarchival "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/archival"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core/secret"
	gqlaws "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/aws"
)

const resourceAWSRCVArchivalLocationDescription = `
The ´rubrik_aws_rcv_archival_location´ resource is used to create and manage RCV
AWS archival locations in RSC.

Rubrik Cloud Vault, RCV, is Rubrik hosted archival storage. The cloud account
and the storage container are owned by Rubrik, so no cloud account ID is needed,
only a region and a tier.

-> **Note:** When ´region´ is left out, the archival location follows the region
   of the workload being archived, and ´location_template´ becomes
   ´SOURCE_REGION´ instead of ´SPECIFIC_REGION´.

-> **Note:** RSC only supports updating the ´name´ and the ´tier´. Changing any
   other argument replaces the archival location.

-> **Note:** Encryption defaults to a key RCV creates and manages. The ´kms_key´
   block selects client-side encryption with an AWS KMS key instead, and the
   ´customer_provided_key´ block selects server-side encryption with an AES key
   you supply. The two blocks are mutually exclusive.
`

var (
	_ resource.Resource                     = &awsRCVArchivalLocationResource{}
	_ resource.ResourceWithConfigValidators = &awsRCVArchivalLocationResource{}
	_ resource.ResourceWithIdentity         = &awsRCVArchivalLocationResource{}
	_ resource.ResourceWithImportState      = &awsRCVArchivalLocationResource{}
)

type awsRCVArchivalLocationResource struct {
	client *client
	prefix string
}

type awsRCVArchivalLocationModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Region           types.String `tfsdk:"region"`
	Tier             types.String `tfsdk:"tier"`
	Redundancy       types.String `tfsdk:"redundancy"`
	ConnectionStatus types.String `tfsdk:"connection_status"`
	LocationTemplate types.String `tfsdk:"location_template"`
	EncryptionType   types.String `tfsdk:"encryption_type"`
	KMSKey           types.List   `tfsdk:"kms_key"`

	CustomerProvidedKey types.List `tfsdk:"customer_provided_key"`
}

type awsRCVKMSKeyModel struct {
	KeyManagerID types.String `tfsdk:"key_manager_id"`
	KeyID        types.String `tfsdk:"key_id"`
}

// awsRCVKMSKeyType is the element type of the kms_key block, needed to build a
// null list when the block has no value to hold.
var awsRCVKMSKeyType = types.ObjectType{AttrTypes: map[string]attr.Type{
	keyKeyManagerID: types.StringType,
	keyKeyID:        types.StringType,
}}

type awsRCVCustomerProvidedKeyModel struct {
	Key     types.String `tfsdk:"key"`
	Version types.String `tfsdk:"version"`
}

// awsRCVCustomerProvidedKeyType is the element type of the
// customer_provided_key block, needed to build a null list when the block has
// no value to hold.
var awsRCVCustomerProvidedKeyType = types.ObjectType{AttrTypes: map[string]attr.Type{
	keyKey:     types.StringType,
	keyVersion: types.StringType,
}}

type awsRCVArchivalLocationIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

func newAWSRCVArchivalLocationResource() resource.Resource {
	return &awsRCVArchivalLocationResource{prefix: keyRubrik}
}

func newPolarisAWSRCVArchivalLocationResource() resource.Resource {
	return &awsRCVArchivalLocationResource{prefix: keyPolaris}
}

func (r *awsRCVArchivalLocationResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "awsRCVArchivalLocationResource.Metadata")

	res.TypeName = r.prefix + "_" + keyAWSRCVArchivalLocation
}

func (r *awsRCVArchivalLocationResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "awsRCVArchivalLocationResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceAWSRCVArchivalLocationDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "Archival location ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyName: schema.StringAttribute{
				Required:    true,
				Description: "Archival location name.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyRegion: schema.StringAttribute{
				Optional: true,
				Description: "AWS region hosting the vault, e.g. `us-east-1`. When left out, the archival " +
					"location follows the region of the workload being archived. Changing this forces a new " +
					"resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyTier: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: fmt.Sprintf("Storage tier. %s.", possibleValues(gqlarchival.RCVTierValues(core.CloudVendorAWS))),
				Validators: []validator.String{
					stringvalidator.OneOf(enumValues(gqlarchival.RCVTierValues(core.CloudVendorAWS))...),
				},
			},
			keyRedundancy: schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: fmt.Sprintf("Storage redundancy. %s. Changing this forces a new resource to "+
					"be created, the RSC API does not support updating it.",
					possibleValues(gqlarchival.RCVRedundancyValues())),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(enumValues(gqlarchival.RCVRedundancyValues())...),
				},
			},
			keyConnectionStatus: schema.StringAttribute{
				Computed:    true,
				Description: "Connection status of the archival location.",
			},
			keyEncryptionType: schema.StringAttribute{
				Computed: true,
				Description: fmt.Sprintf("Encryption mode RSC applied to the archival location. %s.",
					possibleValues(gqlarchival.RCVTargetEncryptionTypeValues(core.CloudVendorAWS))),
			},
			keyLocationTemplate: schema.StringAttribute{
				Computed: true,
				Description: "Location template. `SPECIFIC_REGION` when `region` is specified, otherwise " +
					"`SOURCE_REGION`.",
			},
		},
		Blocks: map[string]schema.Block{
			// A single-element list block rather than a SingleNestedBlock. The
			// two are written identically in a configuration, but a
			// SingleNestedBlock cannot be left out when its attributes are
			// required: the framework still demands a value for each of them
			// and rejects the configuration.
			keyKMSKey: schema.ListNestedBlock{
				Description: "AWS KMS key used for client-side encryption of the archived data. Conflicts " +
					"with `customer_provided_key`. When neither is given, RCV creates and manages the " +
					"encryption key. May be specified at most once. Changing this forces a new resource to " +
					"be created.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyKeyManagerID: schema.StringAttribute{
							Required:    true,
							Description: "KMS key manager ID (UUID).",
							Validators: []validator.String{
								isUUID(),
							},
						},
						keyKeyID: schema.StringAttribute{
							Required:    true,
							Description: "KMS key ID.",
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
					},
				},
			},
			keyCustomerProvidedKey: schema.ListNestedBlock{
				Description: "Customer provided AES key used for server-side encryption of the archived " +
					"data. Conflicts with `kms_key`. When neither is given, RCV creates and manages the " +
					"encryption key. May be specified at most once. Changing this forces a new resource " +
					"to be created.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyKey: schema.StringAttribute{
							Required:  true,
							Sensitive: true,
							WriteOnly: true,
							Description: "AES-256 key, base64 encoded. Generate one with " +
								"`openssl rand -base64 32`. Write-only, so it never reaches Terraform " +
								"state. RSC only accepts the key when the archival location is created, " +
								"change `version` to replace the archival location with one using a new " +
								"key.",
							Validators: []validator.String{
								isAESKey(),
							},
						},
						keyVersion: schema.StringAttribute{
							Required: true,
							Description: "Arbitrary value identifying the version of `key`. Write-only " +
								"arguments produce no difference in the plan on their own, so changing " +
								"this is what makes a new key take effect.",
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
					},
				},
			},
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use `rubrik_aws_rcv_archival_location` instead."
	}
}

func (r *awsRCVArchivalLocationResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "awsRCVArchivalLocationResource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Archival location ID (UUID).",
			},
		},
	}
}

func (r *awsRCVArchivalLocationResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	tflog.Trace(ctx, "awsRCVArchivalLocationResource.ConfigValidators")

	return []resource.ConfigValidator{
		// A KMS key selects client-side encryption and a customer provided key
		// selects server-side encryption.
		resourcevalidator.Conflicting(
			path.MatchRoot(keyKMSKey),
			path.MatchRoot(keyCustomerProvidedKey),
		),
	}
}

func (r *awsRCVArchivalLocationResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "awsRCVArchivalLocationResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *awsRCVArchivalLocationResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "awsRCVArchivalLocationResource.Create")

	var plan awsRCVArchivalLocationModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	// Write-only attributes are null in the plan, the config is the only place
	// holding the encryption key.
	var config awsRCVArchivalLocationModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	createParams := gqlarchival.CreateRCVAWSStorageSettingParams{
		Name:       plan.Name.ValueString(),
		Region:     gqlaws.RegionFromName(plan.Region.ValueString()).ToRCSRegionEnum(),
		Tier:       gqlarchival.RCVTier(plan.Tier.ValueString()),
		Redundancy: gqlarchival.RCVRedundancy(plan.Redundancy.ValueString()),
	}

	kmsKey, diags := toAWSRCVKMSKey(ctx, plan.KMSKey)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	createParams.KMSKey = kmsKey

	keyInDER, diags := toAWSRCVCustomerProvidedKey(ctx, config.CustomerProvidedKey)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	if keyInDER != "" {
		enableSSEWithCPK := true
		createParams.EnableSSEWithCPK = &enableSSEWithCPK
		createParams.EncryptionKeyInDER = keyInDER
	}

	id, err := archival.Wrap(polarisClient).CreateRCVAWSStorageSetting(ctx, createParams)
	if err != nil {
		res.Diagnostics.AddError("Failed to create RCV AWS archival location", err.Error())
		return
	}

	targetMapping, err := archival.Wrap(polarisClient).RCVAWSTargetMappingByID(ctx, id)
	if err != nil {
		res.Diagnostics.AddError("Failed to read RCV AWS archival location", err.Error())
		return
	}

	plan.ID = types.StringValue(id.String())
	updateAWSRCVArchivalLocationModel(&plan, targetMapping)

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := awsRCVArchivalLocationIdentityModel{ID: plan.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *awsRCVArchivalLocationResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "awsRCVArchivalLocationResource.Read")

	var state awsRCVArchivalLocationModel
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
		res.Diagnostics.AddError("Invalid archival location ID", err.Error())
		return
	}

	targetMapping, err := archival.Wrap(polarisClient).RCVAWSTargetMappingByID(ctx, id)
	if errors.Is(err, graphql.ErrNotFound) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read RCV AWS archival location", err.Error())
		return
	}

	updateAWSRCVArchivalLocationModel(&state, targetMapping)

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := awsRCVArchivalLocationIdentityModel{ID: state.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *awsRCVArchivalLocationResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "awsRCVArchivalLocationResource.Update")

	var plan awsRCVArchivalLocationModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	var state awsRCVArchivalLocationModel
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
		res.Diagnostics.AddError("Invalid archival location ID", err.Error())
		return
	}

	updateParams := gqlarchival.UpdateRCVAWSStorageSettingParams{
		Name: plan.Name.ValueString(),
		Tier: gqlarchival.RCVTier(plan.Tier.ValueString()),
	}
	if err := archival.Wrap(polarisClient).UpdateRCVAWSStorageSetting(ctx, id, updateParams); err != nil {
		res.Diagnostics.AddError("Failed to update RCV AWS archival location", err.Error())
		return
	}

	targetMapping, err := archival.Wrap(polarisClient).RCVAWSTargetMappingByID(ctx, id)
	if err != nil {
		res.Diagnostics.AddError("Failed to read RCV AWS archival location", err.Error())
		return
	}

	plan.ID = state.ID
	updateAWSRCVArchivalLocationModel(&plan, targetMapping)

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := awsRCVArchivalLocationIdentityModel{ID: plan.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *awsRCVArchivalLocationResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "awsRCVArchivalLocationResource.Delete")

	var state awsRCVArchivalLocationModel
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
		res.Diagnostics.AddError("Invalid archival location ID", err.Error())
		return
	}

	err = archival.Wrap(polarisClient).DeleteTargetMapping(ctx, id)
	if errors.Is(err, graphql.ErrNotFound) {
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to delete RCV AWS archival location", err.Error())
		return
	}
}

func (r *awsRCVArchivalLocationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "awsRCVArchivalLocationResource.ImportState")

	resource.ImportStatePassthroughWithIdentity(ctx, path.Root(keyID), path.Root(keyID), req, res)
}

// toAWSRCVKMSKey converts the kms_key block into the SDK KMS key parameter. It
// returns nil when the block is left out, letting RCV create and manage the
// encryption key.
func toAWSRCVKMSKey(ctx context.Context, list types.List) (*gqlarchival.AWSKMSKey, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		return nil, diags
	}

	var keys []awsRCVKMSKeyModel
	diags.Append(list.ElementsAs(ctx, &keys, false)...)
	if diags.HasError() || len(keys) == 0 {
		return nil, diags
	}

	keyManagerID, err := uuid.Parse(keys[0].KeyManagerID.ValueString())
	if err != nil {
		diags.AddAttributeError(path.Root(keyKMSKey), "Invalid key manager ID", err.Error())
		return nil, diags
	}

	return &gqlarchival.AWSKMSKey{
		KeyManagerID: keyManagerID,
		KeyID:        keys[0].KeyID.ValueString(),
	}, diags
}

// toAWSRCVCustomerProvidedKey returns the DER encoded key from the
// customer_provided_key block, or the empty string when the block is left out.
// The key is write-only, so the list must come from the config rather than the
// plan.
func toAWSRCVCustomerProvidedKey(ctx context.Context, list types.List) (secret.String, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		return "", diags
	}

	var keys []awsRCVCustomerProvidedKeyModel
	diags.Append(list.ElementsAs(ctx, &keys, false)...)
	if diags.HasError() || len(keys) == 0 {
		return "", diags
	}

	return secret.String(keys[0].Key.ValueString()), diags
}

// updateAWSRCVArchivalLocationModel updates the model with the values read back
// from RSC. The KMS key is left untouched, the RSC API does not return it.
func updateAWSRCVArchivalLocationModel(m *awsRCVArchivalLocationModel, targetMapping gqlarchival.RCVAWSTargetMapping) {
	m.Name = types.StringValue(targetMapping.Name)
	m.Tier = types.StringValue(string(targetMapping.TargetTemplate.Tier))
	m.Redundancy = types.StringValue(string(targetMapping.TargetTemplate.Redundancy))
	m.ConnectionStatus = types.StringValue(targetMapping.ConnectionStatus.Status)
	m.LocationTemplate = types.StringValue(string(targetMapping.TargetTemplate.LocTemplate))
	m.EncryptionType = types.StringValue(string(targetMapping.TargetTemplate.EncryptionType))

	// A source region archival location follows the region of the workload
	// being archived and has no region of its own.
	if name := targetMapping.TargetTemplate.Region.Name(); name != "" {
		m.Region = types.StringValue(name)
	} else {
		m.Region = types.StringNull()
	}
}

// newAWSRCVArchivalLocationModel builds a complete model from a target mapping.
// The KMS key is null, the RSC API does not return it.
func newAWSRCVArchivalLocationModel(targetMapping gqlarchival.RCVAWSTargetMapping) awsRCVArchivalLocationModel {
	m := awsRCVArchivalLocationModel{
		ID:                  types.StringValue(targetMapping.ID.String()),
		KMSKey:              types.ListNull(awsRCVKMSKeyType),
		CustomerProvidedKey: types.ListNull(awsRCVCustomerProvidedKeyType),
	}
	updateAWSRCVArchivalLocationModel(&m, targetMapping)

	return m
}
