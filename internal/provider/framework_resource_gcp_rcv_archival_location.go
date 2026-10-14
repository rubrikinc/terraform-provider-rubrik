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
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/archival"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	gqlarchival "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/archival"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
	gqlgcp "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/gcp"
)

const resourceGCPRCVArchivalLocationDescription = `
The ´rubrik_gcp_rcv_archival_location´ resource is used to create and manage RCV
GCP archival locations in RSC.

Rubrik Cloud Vault, RCV, is Rubrik hosted archival storage. The cloud account
and the storage container are owned by Rubrik, so no cloud account ID is needed,
only a region and a tier.

Encryption defaults to a key created and managed by RCV. The ´kms_key´ block
selects client-side encryption with a GCP Cloud KMS key instead. The key manager
must already be registered in RSC, see the ´rubrik_gcp_kms_key_manager´ data
source.

Changing the key name, the key version or the key manager of the ´kms_key´ block
rekeys the archival location in place. The rekey moves the archival location to
the new key without re-encrypting the archived data. It can take several
minutes, increase ´timeouts.update´ if it takes longer than the default of 20
minutes.

-> **Note:** RSC only supports updating the ´name´ and rekeying the ´kms_key´.
   Changing the ´region´ or the ´tier´, or adding or removing the ´kms_key´
   block, replaces the archival location.
`

// defaultGCPRCVRekeyTimeout is the default time to wait for a rekey of an RCV
// GCP archival location to complete.
const defaultGCPRCVRekeyTimeout = 20 * time.Minute

var (
	_ resource.Resource                = &gcpRCVArchivalLocationResource{}
	_ resource.ResourceWithIdentity    = &gcpRCVArchivalLocationResource{}
	_ resource.ResourceWithImportState = &gcpRCVArchivalLocationResource{}
)

type gcpRCVArchivalLocationResource struct {
	client *client
	prefix string
}

type gcpRCVArchivalLocationModel struct {
	ID               types.String   `tfsdk:"id"`
	Name             types.String   `tfsdk:"name"`
	Region           types.String   `tfsdk:"region"`
	Tier             types.String   `tfsdk:"tier"`
	ConnectionStatus types.String   `tfsdk:"connection_status"`
	LocationTemplate types.String   `tfsdk:"location_template"`
	EncryptionType   types.String   `tfsdk:"encryption_type"`
	KMSKey           types.List     `tfsdk:"kms_key"`
	Timeouts         timeouts.Value `tfsdk:"timeouts"`
}

type gcpRCVKMSKeyModel struct {
	KeyManagerID types.String `tfsdk:"key_manager_id"`
	KeyName      types.String `tfsdk:"key_name"`
	KeyVersion   types.String `tfsdk:"key_version"`
}

// gcpRCVKMSKeyType is the element type of the kms_key block, needed to build a
// null list when the block has no value to hold.
var gcpRCVKMSKeyType = types.ObjectType{AttrTypes: map[string]attr.Type{
	keyKeyManagerID: types.StringType,
	keyKeyName:      types.StringType,
	keyKeyVersion:   types.StringType,
}}

type gcpRCVArchivalLocationIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

func newGCPRCVArchivalLocationResource() resource.Resource {
	return &gcpRCVArchivalLocationResource{prefix: keyRubrik}
}

func newPolarisGCPRCVArchivalLocationResource() resource.Resource {
	return &gcpRCVArchivalLocationResource{prefix: keyPolaris}
}

func (r *gcpRCVArchivalLocationResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "gcpRCVArchivalLocationResource.Metadata")

	res.TypeName = r.prefix + "_" + keyGCPRCVArchivalLocation
}

func (r *gcpRCVArchivalLocationResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "gcpRCVArchivalLocationResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceGCPRCVArchivalLocationDescription),
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
				Description: "GCP region hosting the vault, e.g. `us-west1`. Changing this forces a new " +
					"resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyTier: schema.StringAttribute{
				Required: true,
				Description: fmt.Sprintf("Storage tier. %s. Changing this forces a new resource to be "+
					"created, the RSC API only supports updating the name.",
					possibleValues(gqlarchival.RCVTierValues())),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(enumValues(gqlarchival.RCVTierValues())...),
				},
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
					possibleValues(gqlarchival.RCVTargetEncryptionTypeValues(core.CloudVendorGCP))),
			},
			keyTimeouts: timeouts.Attributes(ctx, timeouts.Opts{
				Update: true,
				UpdateDescription: fmt.Sprintf("How long to wait for a rekey of the archival location to "+
					"complete. Default is `%s`.", defaultGCPRCVRekeyTimeout),
			}),
		},
		Blocks: map[string]schema.Block{
			// A single-element list block rather than a SingleNestedBlock. The
			// two are written identically in a configuration, but a
			// SingleNestedBlock cannot be left out when its attributes are
			// required: the framework still demands a value for each of them
			// and rejects the configuration.
			keyKMSKey: schema.ListNestedBlock{
				Description: "GCP Cloud KMS key used for client-side encryption of the archived data. When " +
					"not given, RCV creates and manages the encryption key. May be specified at most " +
					"once. Changing the key name, the key version or the key manager rekeys the archival " +
					"location in place. Adding or removing the block forces a new resource to be created.",
				PlanModifiers: []planmodifier.List{
					requiresReplaceOnAddOrRemove(),
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
						keyKeyName: schema.StringAttribute{
							Required:    true,
							Description: "Cloud KMS crypto key name.",
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
						keyKeyVersion: schema.StringAttribute{
							Required:    true,
							Description: "Cloud KMS crypto key version.",
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
		res.Schema.DeprecationMessage = "use `rubrik_gcp_rcv_archival_location` instead."
	}
}

func (r *gcpRCVArchivalLocationResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "gcpRCVArchivalLocationResource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Archival location ID (UUID).",
			},
		},
	}
}

func (r *gcpRCVArchivalLocationResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "gcpRCVArchivalLocationResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *gcpRCVArchivalLocationResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "gcpRCVArchivalLocationResource.Create")

	var plan gcpRCVArchivalLocationModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	createParams := gqlarchival.CreateRCVGCPStorageSettingParams{
		Name:   plan.Name.ValueString(),
		Region: gqlgcp.RegionFromName(plan.Region.ValueString()).ToRCSRegionEnum(),
		Tier:   gqlarchival.RCVTier(plan.Tier.ValueString()),
	}

	kmsKey, diags := toGCPRCVKMSKey(ctx, plan.KMSKey)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	createParams.KMSKey = kmsKey

	id, err := archival.Wrap(polarisClient).CreateRCVGCPStorageSetting(ctx, createParams)
	if err != nil {
		res.Diagnostics.AddError("Failed to create RCV GCP archival location", err.Error())
		return
	}

	targetMapping, err := archival.Wrap(polarisClient).RCVGCPTargetMappingByID(ctx, id)
	if err != nil {
		res.Diagnostics.AddError("Failed to read RCV GCP archival location", err.Error())
		return
	}

	plan.ID = types.StringValue(id.String())
	updateGCPRCVArchivalLocationModel(&plan, targetMapping)

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := gcpRCVArchivalLocationIdentityModel{ID: plan.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *gcpRCVArchivalLocationResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "gcpRCVArchivalLocationResource.Read")

	var state gcpRCVArchivalLocationModel
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

	targetMapping, err := archival.Wrap(polarisClient).RCVGCPTargetMappingByID(ctx, id)
	if errors.Is(err, graphql.ErrNotFound) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read RCV GCP archival location", err.Error())
		return
	}

	updateGCPRCVArchivalLocationModel(&state, targetMapping)

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := gcpRCVArchivalLocationIdentityModel{ID: state.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *gcpRCVArchivalLocationResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "gcpRCVArchivalLocationResource.Update")

	var plan gcpRCVArchivalLocationModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	var state gcpRCVArchivalLocationModel
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

	// The rekey runs before the rename, a failed rekey then leaves the name
	// unchanged. Adding or removing the key replaces the archival location, so
	// a difference here is always a change to the existing key.
	if !plan.KMSKey.Equal(state.KMSKey) {
		kmsKey, diags := toGCPRCVKMSKey(ctx, plan.KMSKey)
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return
		}

		if kmsKey != nil {
			timeout, diags := plan.Timeouts.Update(ctx, defaultGCPRCVRekeyTimeout)
			res.Diagnostics.Append(diags...)
			if res.Diagnostics.HasError() {
				return
			}
			rekeyCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			err := archival.Wrap(polarisClient).RekeyRCVGCPMasterKey(rekeyCtx, id, gqlarchival.RekeyArchivalGroupMasterKeyParams{
				KeyManagerID: kmsKey.KeyManagerID,
				GCPKMSKey:    *kmsKey,
			})
			if err != nil {
				res.Diagnostics.AddError("Failed to rekey RCV GCP archival location", err.Error())
				return
			}
		}
	}

	updateParams := gqlarchival.UpdateRCVGCPStorageSettingParams{
		Name: plan.Name.ValueString(),
	}
	if err := archival.Wrap(polarisClient).UpdateRCVGCPStorageSetting(ctx, id, updateParams); err != nil {
		res.Diagnostics.AddError("Failed to update RCV GCP archival location", err.Error())
		return
	}

	targetMapping, err := archival.Wrap(polarisClient).RCVGCPTargetMappingByID(ctx, id)
	if err != nil {
		res.Diagnostics.AddError("Failed to read RCV GCP archival location", err.Error())
		return
	}

	plan.ID = state.ID
	updateGCPRCVArchivalLocationModel(&plan, targetMapping)

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := gcpRCVArchivalLocationIdentityModel{ID: plan.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *gcpRCVArchivalLocationResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "gcpRCVArchivalLocationResource.Delete")

	var state gcpRCVArchivalLocationModel
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
		res.Diagnostics.AddError("Failed to delete RCV GCP archival location", err.Error())
		return
	}
}

func (r *gcpRCVArchivalLocationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "gcpRCVArchivalLocationResource.ImportState")

	resource.ImportStatePassthroughWithIdentity(ctx, path.Root(keyID), path.Root(keyID), req, res)
}

// toGCPRCVKMSKey converts the kms_key block into the SDK Cloud KMS key
// parameter. It returns nil when the block is left out, letting RCV create and
// manage the encryption key.
func toGCPRCVKMSKey(ctx context.Context, list types.List) (*gqlarchival.GCPKMSKey, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		return nil, diags
	}

	var keys []gcpRCVKMSKeyModel
	diags.Append(list.ElementsAs(ctx, &keys, false)...)
	if diags.HasError() || len(keys) == 0 {
		return nil, diags
	}

	keyManagerID, err := uuid.Parse(keys[0].KeyManagerID.ValueString())
	if err != nil {
		diags.AddAttributeError(path.Root(keyKMSKey), "Invalid key manager ID", err.Error())
		return nil, diags
	}

	return &gqlarchival.GCPKMSKey{
		KeyManagerID: keyManagerID,
		KeyName:      keys[0].KeyName.ValueString(),
		KeyVersion:   keys[0].KeyVersion.ValueString(),
	}, diags
}

// updateGCPRCVArchivalLocationModel updates the model with the values read back
// from RSC.
func updateGCPRCVArchivalLocationModel(m *gcpRCVArchivalLocationModel, targetMapping gqlarchival.RCVGCPTargetMapping) {
	m.Name = types.StringValue(targetMapping.Name)
	m.Tier = types.StringValue(string(targetMapping.TargetTemplate.Tier))
	m.ConnectionStatus = types.StringValue(targetMapping.ConnectionStatus.Status)
	m.LocationTemplate = types.StringValue(string(targetMapping.TargetTemplate.LocTemplate))
	m.EncryptionType = types.StringValue(string(targetMapping.TargetTemplate.EncryptionType))

	if name := targetMapping.TargetTemplate.Region.Name(); name != "" {
		m.Region = types.StringValue(name)
	} else {
		m.Region = types.StringNull()
	}
}

// newGCPRCVArchivalLocationModel builds a complete model from a target mapping.
// The Cloud KMS key is null, the RSC API does not return it. The timeouts are
// the correctly typed null value, derived from the resource schema.
func newGCPRCVArchivalLocationModel(targetMapping gqlarchival.RCVGCPTargetMapping, nullTimeouts timeouts.Value) gcpRCVArchivalLocationModel {
	m := gcpRCVArchivalLocationModel{
		ID:       types.StringValue(targetMapping.ID.String()),
		KMSKey:   types.ListNull(gcpRCVKMSKeyType),
		Timeouts: nullTimeouts,
	}
	updateGCPRCVArchivalLocationModel(&m, targetMapping)

	return m
}
