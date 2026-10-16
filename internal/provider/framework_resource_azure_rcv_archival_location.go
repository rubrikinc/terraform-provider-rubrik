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
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
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
	gqlazure "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/azure"
)

const resourceAzureRCVArchivalLocationDescription = `
The ´rubrik_azure_rcv_archival_location´ resource is used to create and manage
RCV Azure archival locations in RSC.

Rubrik Cloud Vault, RCV, is Rubrik hosted archival storage. The cloud account
and the storage container are owned by Rubrik, so no cloud account ID is needed,
only a region and a tier.

-> **Note:** Azure is the only RCV cloud accepting a ´source_workload_cloud´,
   letting an Azure hosted vault archive AWS or GCP workloads. When left out,
   RSC derives it from the destination cloud.

-> **Note:** Encryption defaults to a key RCV creates and manages. The
   ´key_vault_key´ block selects client-side encryption with an Azure Key Vault
   key instead, and the ´rsa_key´ block selects client-side encryption with an
   RSA key you supply. The two blocks are mutually exclusive.

Changing the key name, the key version or the key vault of the ´key_vault_key´
block rekeys the archival location in place. The rekey moves the archival
location to the new key without re-encrypting the archived data. It can take
several minutes, increase ´timeouts.update´ if it takes longer than the default
of 20 minutes.

-> **Note:** RSC only supports updating the ´name´, the ´tier´ and the
   ´redundancy´, and rekeying the ´key_vault_key´. Changing any other argument,
   or adding or removing the ´key_vault_key´ block, replaces the archival
   location.
`

// defaultAzureRCVRekeyTimeout is the default time to wait for a rekey of an RCV
// Azure archival location to complete.
const defaultAzureRCVRekeyTimeout = 20 * time.Minute

var (
	_ resource.Resource                     = &azureRCVArchivalLocationResource{}
	_ resource.ResourceWithConfigValidators = &azureRCVArchivalLocationResource{}
	_ resource.ResourceWithIdentity         = &azureRCVArchivalLocationResource{}
	_ resource.ResourceWithImportState      = &azureRCVArchivalLocationResource{}
)

type azureRCVArchivalLocationResource struct {
	client *client
	prefix string
}

type azureRCVArchivalLocationModel struct {
	ID                  types.String   `tfsdk:"id"`
	Name                types.String   `tfsdk:"name"`
	Region              types.String   `tfsdk:"region"`
	Tier                types.String   `tfsdk:"tier"`
	Redundancy          types.String   `tfsdk:"redundancy"`
	SourceWorkloadCloud types.String   `tfsdk:"source_workload_cloud"`
	UpdateChildVaults   types.Bool     `tfsdk:"update_child_vaults"`
	RSAKey              types.List     `tfsdk:"rsa_key"`
	LockDurationDays    types.Int64    `tfsdk:"lock_duration_days"`
	ConnectionStatus    types.String   `tfsdk:"connection_status"`
	LocationTemplate    types.String   `tfsdk:"location_template"`
	EncryptionType      types.String   `tfsdk:"encryption_type"`
	KeyVaultKey         types.List     `tfsdk:"key_vault_key"`
	Timeouts            timeouts.Value `tfsdk:"timeouts"`
}

type azureRCVKeyVaultKeyModel struct {
	KeyVaultID types.String `tfsdk:"key_vault_id"`
	KeyName    types.String `tfsdk:"key_name"`
	KeyVersion types.String `tfsdk:"key_version"`
}

// azureRCVKeyVaultKeyType is the element type of the key_vault_key block,
// needed to build a null list when the block has no value to hold.
var azureRCVKeyVaultKeyType = types.ObjectType{AttrTypes: map[string]attr.Type{
	keyKeyVaultID: types.StringType,
	keyKeyName:    types.StringType,
	keyKeyVersion: types.StringType,
}}

type azureRCVRSAKeyModel struct {
	Key     types.String `tfsdk:"key"`
	Version types.String `tfsdk:"version"`
}

// azureRCVRSAKeyType is the element type of the rsa_key block, needed to build
// a null list when the block has no value to hold.
var azureRCVRSAKeyType = types.ObjectType{AttrTypes: map[string]attr.Type{
	keyKey:     types.StringType,
	keyVersion: types.StringType,
}}

type azureRCVArchivalLocationIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

func newAzureRCVArchivalLocationResource() resource.Resource {
	return &azureRCVArchivalLocationResource{prefix: keyRubrik}
}

func newPolarisAzureRCVArchivalLocationResource() resource.Resource {
	return &azureRCVArchivalLocationResource{prefix: keyPolaris}
}

func (r *azureRCVArchivalLocationResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "azureRCVArchivalLocationResource.Metadata")

	res.TypeName = r.prefix + "_" + keyAzureRCVArchivalLocation
}

func (r *azureRCVArchivalLocationResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "azureRCVArchivalLocationResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceAzureRCVArchivalLocationDescription),
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
				Required: true,
				Description: "Azure region hosting the vault, e.g. `eastus2`. Changing this forces a new " +
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
				Description: fmt.Sprintf("Storage tier. %s.", possibleValues(gqlarchival.RCVTierValues())),
				Validators: []validator.String{
					stringvalidator.OneOf(enumValues(gqlarchival.RCVTierValues())...),
				},
			},
			keyRedundancy: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: fmt.Sprintf("Storage redundancy. %s.", possibleValues(gqlarchival.RCVRedundancyValues())),
				Validators: []validator.String{
					stringvalidator.OneOf(enumValues(gqlarchival.RCVRedundancyValues())...),
				},
			},
			keySourceWorkloadCloud: schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: fmt.Sprintf("Cloud the archived workloads originate from, letting an Azure "+
					"hosted vault archive AWS or GCP workloads. %s. When left out, RSC derives it from "+
					"the destination cloud. Changing this forces a new resource to be created.",
					possibleValues(gqlarchival.SourceWorkloadCloudValues())),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(enumValues(gqlarchival.SourceWorkloadCloudValues())...),
				},
			},
			keyUpdateChildVaults: schema.BoolAttribute{
				Optional: true,
				Description: "Propagate updates to the child vaults of the archival location. Only used " +
					"when updating an existing archival location.",
			},
			keyLockDurationDays: schema.Int64Attribute{
				Computed:    true,
				Description: "Number of days the archived data is locked, from the immutability settings.",
			},
			keyConnectionStatus: schema.StringAttribute{
				Computed:    true,
				Description: "Connection status of the archival location.",
			},
			keyLocationTemplate: schema.StringAttribute{
				Computed:    true,
				Description: "Location template, `SPECIFIC_REGION` or `SOURCE_REGION`.",
			},
			keyEncryptionType: schema.StringAttribute{
				Computed: true,
				Description: fmt.Sprintf("Encryption mode RSC applied to the archival location. %s.",
					possibleValues(gqlarchival.RCVTargetEncryptionTypeValues(core.CloudVendorAzure))),
			},
			keyTimeouts: timeouts.Attributes(ctx, timeouts.Opts{
				Update: true,
				UpdateDescription: fmt.Sprintf("How long to wait for a rekey of the archival location to "+
					"complete. Default is `%s`.", defaultAzureRCVRekeyTimeout),
			}),
		},
		Blocks: map[string]schema.Block{
			// A single-element list block rather than a SingleNestedBlock. The
			// two are written identically in a configuration, but a
			// SingleNestedBlock cannot be left out when its attributes are
			// required: the framework still demands a value for each of them
			// and rejects the configuration.
			keyKeyVaultKey: schema.ListNestedBlock{
				Description: "Azure Key Vault key used for client-side encryption of the archived data. " +
					"Conflicts with `rsa_key`. When neither is given, RCV creates and manages the " +
					"encryption key. May be specified at most once. Changing the key name, the key version " +
					"or the key vault rekeys the archival location in place. Adding or removing the block " +
					"forces a new resource to be created.",
				PlanModifiers: []planmodifier.List{
					requiresReplaceOnAddOrRemove(),
				},
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyKeyVaultID: schema.StringAttribute{
							Required:    true,
							Description: "Key Vault ID (UUID).",
							Validators: []validator.String{
								isUUID(),
							},
						},
						keyKeyName: schema.StringAttribute{
							Required:    true,
							Description: "Key Vault key name.",
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
						keyKeyVersion: schema.StringAttribute{
							Required:    true,
							Description: "Key Vault key version.",
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
					},
				},
			},
			keyRSAKey: schema.ListNestedBlock{
				Description: "RSA key used for client-side encryption of the archived data. Conflicts " +
					"with `key_vault_key`. When neither is given, RCV creates and manages the encryption " +
					"key. May be specified at most once. Changing this forces a new resource to be created.",
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
							Description: "RSA key. Write-only, so it never reaches Terraform state. RSC " +
								"only accepts the key when the archival location is created, change " +
								"`version` to replace the archival location with one using a new key.",
							Validators: []validator.String{
								isNotWhiteSpace(),
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
		res.Schema.DeprecationMessage = "use `rubrik_azure_rcv_archival_location` instead."
	}
}

func (r *azureRCVArchivalLocationResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "azureRCVArchivalLocationResource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Archival location ID (UUID).",
			},
		},
	}
}

func (r *azureRCVArchivalLocationResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	tflog.Trace(ctx, "azureRCVArchivalLocationResource.ConfigValidators")

	return []resource.ConfigValidator{
		// A Key Vault key and an RSA key are different client-side encryption
		// modes.
		resourcevalidator.Conflicting(
			path.MatchRoot(keyKeyVaultKey),
			path.MatchRoot(keyRSAKey),
		),
	}
}

func (r *azureRCVArchivalLocationResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "azureRCVArchivalLocationResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *azureRCVArchivalLocationResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "azureRCVArchivalLocationResource.Create")

	var plan azureRCVArchivalLocationModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	// Write-only attributes are null in the plan, the config is the only place
	// holding the RSA key.
	var config azureRCVArchivalLocationModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	createParams := gqlarchival.CreateRCVAzureStorageSettingParams{
		Name:                plan.Name.ValueString(),
		Region:              gqlazure.RegionFromName(plan.Region.ValueString()).ToRegionEnum(),
		Tier:                gqlarchival.RCVTier(plan.Tier.ValueString()),
		SourceWorkloadCloud: gqlarchival.SourceWorkloadCloud(plan.SourceWorkloadCloud.ValueString()),
		Redundancy:          gqlarchival.RCVRedundancy(plan.Redundancy.ValueString()),
	}

	rsaKey, diags := toAzureRCVRSAKey(ctx, config.RSAKey)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	createParams.RSAKey = rsaKey

	keyVaultKey, diags := toAzureRCVKeyVaultKey(ctx, plan.KeyVaultKey)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	createParams.KeyVaultKey = keyVaultKey

	id, err := archival.Wrap(polarisClient).CreateRCVAzureStorageSetting(ctx, createParams)
	if err != nil {
		res.Diagnostics.AddError("Failed to create RCV Azure archival location", err.Error())
		return
	}

	targetMapping, err := archival.Wrap(polarisClient).RCVAzureTargetMappingByID(ctx, id)
	if err != nil {
		res.Diagnostics.AddError("Failed to read RCV Azure archival location", err.Error())
		return
	}

	plan.ID = types.StringValue(id.String())
	updateAzureRCVArchivalLocationModel(&plan, targetMapping)

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := azureRCVArchivalLocationIdentityModel{ID: plan.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *azureRCVArchivalLocationResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "azureRCVArchivalLocationResource.Read")

	var state azureRCVArchivalLocationModel
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

	targetMapping, err := archival.Wrap(polarisClient).RCVAzureTargetMappingByID(ctx, id)
	if errors.Is(err, graphql.ErrNotFound) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read RCV Azure archival location", err.Error())
		return
	}

	updateAzureRCVArchivalLocationModel(&state, targetMapping)

	// RSC only lists the groups encrypted with a key from a key manager. A group
	// which isn't listed has no such key, and the key_vault_key block is left as
	// it is.
	details, err := archival.Wrap(polarisClient).ArchivalGroupEncryptionDetailsByID(ctx, id)
	if err != nil && !errors.Is(err, graphql.ErrNotFound) {
		res.Diagnostics.AddError("Failed to read RCV Azure archival location encryption details", err.Error())
		return
	}
	if err == nil && details.KMSKey != nil && details.KMSKey.Key != nil && details.KMSKey.Key.AzureKey != nil {
		keyVaultKey := []azureRCVKeyVaultKeyModel{{
			KeyVaultID: types.StringValue(details.KMSKey.KeyManagerID),
			KeyName:    types.StringValue(details.KMSKey.Key.AzureKey.KeyName),
			KeyVersion: types.StringValue(details.KMSKey.Key.AzureKey.KeyVersion),
		}}
		list, diags := types.ListValueFrom(ctx, azureRCVKeyVaultKeyType, keyVaultKey)
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return
		}
		state.KeyVaultKey = list
	}

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := azureRCVArchivalLocationIdentityModel{ID: state.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *azureRCVArchivalLocationResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "azureRCVArchivalLocationResource.Update")

	var plan azureRCVArchivalLocationModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	var state azureRCVArchivalLocationModel
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

	// The rekey runs before the update, a failed rekey then leaves the name, the
	// tier and the redundancy unchanged. Adding or removing the key replaces the
	// archival location, so a difference here is always a change to the existing
	// key.
	if !plan.KeyVaultKey.Equal(state.KeyVaultKey) {
		keyVaultKey, diags := toAzureRCVKeyVaultKey(ctx, plan.KeyVaultKey)
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return
		}

		if keyVaultKey != nil {
			timeout, diags := plan.Timeouts.Update(ctx, defaultAzureRCVRekeyTimeout)
			res.Diagnostics.Append(diags...)
			if res.Diagnostics.HasError() {
				return
			}
			rekeyCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			err := archival.Wrap(polarisClient).RekeyRCVAzureMasterKey(rekeyCtx, id, gqlarchival.RekeyArchivalGroupMasterKeyParams{
				AzureKeyVaultKey: keyVaultKey,
			})
			if err != nil {
				res.Diagnostics.AddError("Failed to rekey RCV Azure archival location", err.Error())
				return
			}
		}
	}

	updateParams := gqlarchival.UpdateRCVAzureStorageSettingParams{
		Name:       plan.Name.ValueString(),
		Tier:       gqlarchival.RCVTier(plan.Tier.ValueString()),
		Redundancy: gqlarchival.RCVRedundancy(plan.Redundancy.ValueString()),
	}
	if !plan.UpdateChildVaults.IsNull() && !plan.UpdateChildVaults.IsUnknown() {
		updateChildVaults := plan.UpdateChildVaults.ValueBool()
		updateParams.UpdateChildVaults = &updateChildVaults
	}

	if err := archival.Wrap(polarisClient).UpdateRCVAzureStorageSetting(ctx, id, updateParams); err != nil {
		res.Diagnostics.AddError("Failed to update RCV Azure archival location", err.Error())
		return
	}

	targetMapping, err := archival.Wrap(polarisClient).RCVAzureTargetMappingByID(ctx, id)
	if err != nil {
		res.Diagnostics.AddError("Failed to read RCV Azure archival location", err.Error())
		return
	}

	plan.ID = state.ID
	updateAzureRCVArchivalLocationModel(&plan, targetMapping)

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := azureRCVArchivalLocationIdentityModel{ID: plan.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *azureRCVArchivalLocationResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "azureRCVArchivalLocationResource.Delete")

	var state azureRCVArchivalLocationModel
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
		res.Diagnostics.AddError("Failed to delete RCV Azure archival location", err.Error())
		return
	}
}

func (r *azureRCVArchivalLocationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "azureRCVArchivalLocationResource.ImportState")

	resource.ImportStatePassthroughWithIdentity(ctx, path.Root(keyID), path.Root(keyID), req, res)
}

// toAzureRCVKeyVaultKey converts the key_vault_key block into the SDK Key Vault
// key parameter. It returns nil when the block is left out, letting RCV create
// and manage the encryption key.
func toAzureRCVKeyVaultKey(ctx context.Context, list types.List) (*gqlarchival.AzureKeyVaultKey, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		return nil, diags
	}

	var keys []azureRCVKeyVaultKeyModel
	diags.Append(list.ElementsAs(ctx, &keys, false)...)
	if diags.HasError() || len(keys) == 0 {
		return nil, diags
	}

	keyVaultID, err := uuid.Parse(keys[0].KeyVaultID.ValueString())
	if err != nil {
		diags.AddAttributeError(path.Root(keyKeyVaultKey), "Invalid key vault ID", err.Error())
		return nil, diags
	}

	return &gqlarchival.AzureKeyVaultKey{
		KeyVaultID: keyVaultID,
		KeyName:    keys[0].KeyName.ValueString(),
		KeyVersion: keys[0].KeyVersion.ValueString(),
	}, diags
}

// toAzureRCVRSAKey returns the key from the rsa_key block, or the empty string
// when the block is left out. The key is write-only, so the list must come from
// the config rather than the plan.
func toAzureRCVRSAKey(ctx context.Context, list types.List) (secret.String, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		return "", diags
	}

	var keys []azureRCVRSAKeyModel
	diags.Append(list.ElementsAs(ctx, &keys, false)...)
	if diags.HasError() || len(keys) == 0 {
		return "", diags
	}

	return secret.String(keys[0].Key.ValueString()), diags
}

// updateAzureRCVArchivalLocationModel updates the model with the values read
// back from RSC. The RSA key is left untouched, the RSC API does not return it.
// The Key Vault key is left untouched too, Read gets it from the encryption
// details of the archival group.
func updateAzureRCVArchivalLocationModel(m *azureRCVArchivalLocationModel, targetMapping gqlarchival.RCVAzureTargetMapping) {
	m.Name = types.StringValue(targetMapping.Name)
	m.Tier = types.StringValue(string(targetMapping.TargetTemplate.Tier))
	m.Redundancy = types.StringValue(string(targetMapping.TargetTemplate.Redundancy))
	m.SourceWorkloadCloud = types.StringValue(string(targetMapping.TargetTemplate.SourceWorkloadCloud))
	m.ConnectionStatus = types.StringValue(targetMapping.ConnectionStatus.Status)
	m.LocationTemplate = types.StringValue(string(targetMapping.TargetTemplate.LocTemplate))
	m.EncryptionType = types.StringValue(string(targetMapping.TargetTemplate.EncryptionType))
	m.LockDurationDays = types.Int64Value(targetMapping.TargetTemplate.ImmutabilitySettings.LockDurationDays)

	if name := targetMapping.TargetTemplate.Region.Name(); name != "" {
		m.Region = types.StringValue(name)
	} else {
		m.Region = types.StringNull()
	}
}

// newAzureRCVArchivalLocationModel builds a complete model from a target
// mapping. The RSA key is null, the RSC API does not return it. The Key Vault
// key is null, the list resource sets it from the encryption details. So is
// update_child_vaults, which only exists for updates. The timeouts are the
// correctly typed null value, derived from the resource schema.
func newAzureRCVArchivalLocationModel(targetMapping gqlarchival.RCVAzureTargetMapping, nullTimeouts timeouts.Value) azureRCVArchivalLocationModel {
	m := azureRCVArchivalLocationModel{
		ID:                types.StringValue(targetMapping.ID.String()),
		UpdateChildVaults: types.BoolNull(),
		RSAKey:            types.ListNull(azureRCVRSAKeyType),
		KeyVaultKey:       types.ListNull(azureRCVKeyVaultKeyType),
		Timeouts:          nullTimeouts,
	}
	updateAzureRCVArchivalLocationModel(&m, targetMapping)

	return m
}
