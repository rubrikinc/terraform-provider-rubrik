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
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
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

const resourceGCPArchivalLocationDescription = `
The ´rubrik_gcp_archival_location´ resource creates an RSC archival location
for cloud native workloads. This resource requires that the GCP project has been
onboarded with the ´CLOUD_NATIVE_ARCHIVAL´ feature.

When creating an archival location, the region where the snapshots are stored
needs to be specified:
  * ´SOURCE_REGION´ - Store snapshots in the same region to minimize data
    transfer charges. This is the default behaviour when the ´region´ field is
    not specified.
  * ´SPECIFIC_REGION´ - Storing snapshots in another region can increase total
    data transfer charges. The ´region´ field specifies the region.

Custom storage encryption is enabled by specifying one or more customer managed
key blocks. For ´SPECIFIC_REGION´, a customer managed key block for the specific
region must be specified. For ´SOURCE_REGION´, a customer managed key block for
each source region should be specified, source regions not having a customer
managed key block will have its data encrypted with platform managed keys.

Client-side encryption is enabled by specifying the ´kms_key´ block, with a Cloud
KMS key in a key manager registered in RSC. Register the key manager with the
´rubrik_gcp_key_manager´ resource, or look up an existing one with the
´rubrik_gcp_key_manager´ data source. The ´kms_key´ block conflicts with the
customer managed key blocks. Exocompute performs the encryption, so Exocompute
must be configured for the key manager's cloud account in the key manager's
region. RSC does not check the key manager's region against the archival
location's region.

Changing the key name or the key version of the ´kms_key´ block rekeys the
archival location in place, without re-encrypting the archived data. The rekey
can take several minutes, increase ´timeouts.update´ if it takes longer than the
default of 20 minutes. Changing the key manager, or adding or removing the
block, replaces the archival location.

-> **Note:** When using ´SOURCE_REGION´ the GCP bucket isn't created until the
   first protected object is archived.
`

// defaultGCPArchivalLocationRekeyTimeout is the default time to wait for a
// rekey of a GCP archival location to complete.
const defaultGCPArchivalLocationRekeyTimeout = 20 * time.Minute

var (
	_ resource.Resource                   = &gcpArchivalLocationResource{}
	_ resource.ResourceWithIdentity       = &gcpArchivalLocationResource{}
	_ resource.ResourceWithImportState    = &gcpArchivalLocationResource{}
	_ resource.ResourceWithMoveState      = &gcpArchivalLocationResource{}
	_ resource.ResourceWithValidateConfig = &gcpArchivalLocationResource{}
)

type gcpArchivalLocationResource struct {
	client *client
	prefix string
}

type gcpArchivalLocationResourceModel struct {
	ID                 types.String   `tfsdk:"id"`
	CloudAccountID     types.String   `tfsdk:"cloud_account_id"`
	BucketLabels       types.Map      `tfsdk:"bucket_labels"`
	BucketPrefix       types.String   `tfsdk:"bucket_prefix"`
	ConnectionStatus   types.String   `tfsdk:"connection_status"`
	CustomerManagedKey types.Set      `tfsdk:"customer_managed_key"`
	KMSKey             types.List     `tfsdk:"kms_key"`
	LocationTemplate   types.String   `tfsdk:"location_template"`
	Name               types.String   `tfsdk:"name"`
	Region             types.String   `tfsdk:"region"`
	StorageClass       types.String   `tfsdk:"storage_class"`
	Timeouts           timeouts.Value `tfsdk:"timeouts"`
}

type gcpArchivalLocationCustomerManagedKeyModel struct {
	Name     types.String `tfsdk:"name"`
	Region   types.String `tfsdk:"region"`
	RingName types.String `tfsdk:"ring_name"`
}

type gcpArchivalLocationKMSKeyModel struct {
	KeyManagerID types.String `tfsdk:"key_manager_id"`
	KeyName      types.String `tfsdk:"key_name"`
	KeyVersion   types.String `tfsdk:"key_version"`
}

// gcpArchivalLocationCustomerManagedKeyType is the element type of the
// customer_managed_key block.
var gcpArchivalLocationCustomerManagedKeyType = types.ObjectType{AttrTypes: map[string]attr.Type{
	keyName:     types.StringType,
	keyRegion:   types.StringType,
	keyRingName: types.StringType,
}}

// gcpArchivalLocationKMSKeyType is the element type of the kms_key block.
var gcpArchivalLocationKMSKeyType = types.ObjectType{AttrTypes: map[string]attr.Type{
	keyKeyManagerID: types.StringType,
	keyKeyName:      types.StringType,
	keyKeyVersion:   types.StringType,
}}

type gcpArchivalLocationIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

func newGcpArchivalLocationResource() resource.Resource {
	return &gcpArchivalLocationResource{prefix: keyRubrik}
}

func newPolarisGcpArchivalLocationResource() resource.Resource {
	return &gcpArchivalLocationResource{prefix: keyPolaris}
}

func (r *gcpArchivalLocationResource) Metadata(ctx context.Context, _ resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "gcpArchivalLocationResource.Metadata")

	res.TypeName = r.prefix + "_" + keyGCPArchivalLocation
}

func (r *gcpArchivalLocationResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "gcpArchivalLocationResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceGCPArchivalLocationDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:    true,
				Description: "Cloud native archival location ID (UUID).",
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
			keyBucketLabels: schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "GCP bucket labels. Each label will be added to the GCP bucket created by RSC.",
			},
			keyBucketPrefix: schema.StringAttribute{
				Required: true,
				Description: "GCP bucket prefix. The prefix cannot be longer than 19 characters. Note that " +
					"`rubrik-` will always be prepended to the prefix. Changing this forces a new resource to " +
					"be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 19),
				},
			},
			keyConnectionStatus: schema.StringAttribute{
				Computed:    true,
				Description: "Connection status of the cloud native archival location.",
			},
			keyLocationTemplate: schema.StringAttribute{
				Computed: true,
				Description: "RSC location template. If a region was specified, it will be `SPECIFIC_REGION`, " +
					"otherwise `SOURCE_REGION`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyName: schema.StringAttribute{
				Required:    true,
				Description: "Name of the cloud native archival location.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyRegion: schema.StringAttribute{
				Optional: true,
				Description: "GCP region to store the snapshots in (`SPECIFIC_REGION`). If not specified, the " +
					"snapshots will be stored in the same region as the workload (`SOURCE_REGION`). Changing " +
					"this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(gqlgcp.AllRegionNames()...),
				},
			},
			keyStorageClass: schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("STANDARD"),
				Description: "GCP bucket storage class. Possible values are `ARCHIVE`, `COLDLINE`, `NEARLINE`, " +
					"`STANDARD` and `DURABLE_REDUCED_AVAILABILITY`. Default value is `STANDARD`.",
				Validators: []validator.String{
					stringvalidator.OneOf("ARCHIVE", "COLDLINE", "NEARLINE", "STANDARD",
						"DURABLE_REDUCED_AVAILABILITY"),
				},
			},
		},
		Blocks: map[string]schema.Block{
			keyCustomerManagedKey: schema.SetNestedBlock{
				Description: "Customer managed storage encryption. For `SPECIFIC_REGION`, a customer managed " +
					"key block for the specific region must be specified. For `SOURCE_REGION`, a customer " +
					"managed key block for each source region should be specified, source regions not having a " +
					"customer managed key block will have its data encrypted with platform managed keys. " +
					"Conflicts with `kms_key`.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyName: schema.StringAttribute{
							Required:    true,
							Description: "Key name.",
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
						keyRegion: schema.StringAttribute{
							Required:    true,
							Description: "The region in which the key will be used.",
							Validators: []validator.String{
								stringvalidator.OneOf(gqlgcp.AllRegionNames()...),
							},
						},
						keyRingName: schema.StringAttribute{
							Required:    true,
							Description: "Key ring name.",
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
					},
				},
			},
			// A single-element list block rather than a SingleNestedBlock, to
			// keep the SDKv2 state and configuration shape.
			keyKMSKey: schema.ListNestedBlock{
				Description: "GCP Cloud KMS key used for client-side encryption of the archived data. Conflicts " +
					"with `customer_managed_key`. May be specified at most once. Changing the key name or the " +
					"key version rekeys the archival location in place. Changing the key manager, or adding or " +
					"removing the block, forces a new resource to be created.",
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
							Description: "RSC key manager ID (UUID).",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
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
			// A block, not an attribute, so configurations written for the
			// SDKv2 resource keep working.
			keyTimeouts: timeouts.Block(ctx, timeouts.Opts{
				Update: true,
			}),
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use `rubrik_gcp_archival_location` instead."
	}
}

func (r *gcpArchivalLocationResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "gcpArchivalLocationResource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Cloud native archival location ID (UUID).",
			},
		},
	}
}

func (r *gcpArchivalLocationResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, res *resource.ValidateConfigResponse) {
	tflog.Trace(ctx, "gcpArchivalLocationResource.ValidateConfig")

	var cmk types.Set
	res.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(keyCustomerManagedKey), &cmk)...)
	var kmsKey types.List
	res.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(keyKMSKey), &kmsKey)...)
	if res.Diagnostics.HasError() {
		return
	}

	// A KMS key selects client-side encryption and a customer managed key
	// selects server-side encryption. An absent block may be null or empty,
	// so the elements are counted.
	if len(cmk.Elements()) > 0 && len(kmsKey.Elements()) > 0 {
		res.Diagnostics.AddAttributeError(path.Root(keyKMSKey), "Conflicting blocks",
			"kms_key conflicts with customer_managed_key, only one of them can be specified.")
	}
}

func (r *gcpArchivalLocationResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "gcpArchivalLocationResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *gcpArchivalLocationResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "gcpArchivalLocationResource.Create")

	var plan gcpArchivalLocationResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := r.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	cloudAccountID, err := uuid.Parse(plan.CloudAccountID.ValueString())
	if err != nil {
		res.Diagnostics.AddAttributeError(path.Root(keyCloudAccountID), "Invalid cloud account ID", err.Error())
		return
	}

	var labels map[string]string
	res.Diagnostics.Append(plan.BucketLabels.ElementsAs(ctx, &labels, false)...)
	var cmks []gcpArchivalLocationCustomerManagedKeyModel
	res.Diagnostics.Append(plan.CustomerManagedKey.ElementsAs(ctx, &cmks, false)...)
	var kmsKeys []gcpArchivalLocationKMSKeyModel
	res.Diagnostics.Append(plan.KMSKey.ElementsAs(ctx, &kmsKeys, false)...)
	if res.Diagnostics.HasError() {
		return
	}

	createParams := gqlarchival.CreateGCPStorageSettingParams{
		CloudAccountID: cloudAccountID,
		Name:           plan.Name.ValueString(),
		BucketPrefix:   plan.BucketPrefix.ValueString(),
		Region:         gqlgcp.RegionFromName(plan.Region.ValueString()).ToRegionEnum(),
		StorageClass:   plan.StorageClass.ValueString() + "_GCP",
	}
	if len(labels) > 0 {
		tags := core.Tags{TagList: make([]core.Tag, 0, len(labels))}
		for key, value := range labels {
			tags.TagList = append(tags.TagList, core.Tag{Key: key, Value: value})
		}
		createParams.BucketLabels = &tags
	}
	for _, cmk := range cmks {
		createParams.CMKInfo = append(createParams.CMKInfo, gqlarchival.GCPCustomerKey{
			Name:     cmk.Name.ValueString(),
			RingName: cmk.RingName.ValueString(),
			Region:   gqlgcp.RegionFromName(cmk.Region.ValueString()).ToRegionEnum(),
		})
	}
	if len(kmsKeys) > 0 {
		keyManagerID, err := uuid.Parse(kmsKeys[0].KeyManagerID.ValueString())
		if err != nil {
			res.Diagnostics.AddAttributeError(path.Root(keyKMSKey), "Invalid key manager ID", err.Error())
			return
		}
		createParams.KMSKey = &gqlarchival.GCPKMSKey{
			KeyManagerID: keyManagerID,
			KeyName:      kmsKeys[0].KeyName.ValueString(),
			KeyVersion:   kmsKeys[0].KeyVersion.ValueString(),
		}
	}

	id, err := archival.Wrap(polarisClient).CreateGCPStorageSetting(ctx, createParams)
	if err != nil {
		res.Diagnostics.AddError("Failed to create GCP archival location", err.Error())
		return
	}

	// Save the ID to state and identity before the read-back so Terraform can
	// track the archival location even if the read fails.
	plan.ID = types.StringValue(id.String())
	if plan.ConnectionStatus.IsUnknown() {
		plan.ConnectionStatus = types.StringNull()
	}
	if plan.LocationTemplate.IsUnknown() {
		plan.LocationTemplate = types.StringNull()
	}
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.Identity.Set(ctx, gcpArchivalLocationIdentityModel{ID: plan.ID})...)
	if res.Diagnostics.HasError() {
		return
	}

	targetMapping, err := archival.Wrap(polarisClient).GCPTargetMappingByID(ctx, id)
	if err != nil {
		res.Diagnostics.AddWarning("Failed to read GCP archival location after create",
			fmt.Sprintf("The archival location was created successfully but the computed fields could not be "+
				"populated: %s", err.Error()))
		return
	}
	// Only the computed values are read back, the configured values must equal
	// the plan.
	plan.ConnectionStatus = types.StringValue(targetMapping.ConnectionStatus.Status)
	plan.LocationTemplate = types.StringValue(targetMapping.TargetTemplate.LocTemplate)
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
}

func (r *gcpArchivalLocationResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "gcpArchivalLocationResource.Read")

	var state gcpArchivalLocationResourceModel
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

	targetMapping, err := archival.Wrap(polarisClient).GCPTargetMappingByID(ctx, id)
	if errors.Is(err, graphql.ErrNotFound) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read GCP archival location", err.Error())
		return
	}
	targetTemplate := targetMapping.TargetTemplate

	labels := make(map[string]string, len(targetTemplate.BucketLabels))
	for _, tag := range targetTemplate.BucketLabels {
		labels[tag.Key] = tag.Value
	}
	labelMap, diags := types.MapValueFrom(ctx, types.StringType, labels)
	res.Diagnostics.Append(diags...)

	cmks := make([]gcpArchivalLocationCustomerManagedKeyModel, 0, len(targetTemplate.CMKInfo))
	for _, key := range targetTemplate.CMKInfo {
		cmks = append(cmks, gcpArchivalLocationCustomerManagedKeyModel{
			Name:     types.StringValue(key.Name),
			Region:   types.StringValue(key.Region.Name()),
			RingName: types.StringValue(key.RingName),
		})
	}
	cmkSet, diags := types.SetValueFrom(ctx, gcpArchivalLocationCustomerManagedKeyType, cmks)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	// The SDKv2 resource stored no labels as an empty map and no region as an
	// empty string. Both are null here, matching a configuration leaving them
	// out, so state moved or upgraded from SDKv2 doesn't plan a change, or
	// for the region, a replacement. No labels only become null when the prior
	// value was null, so a configured empty map is kept. The region validator
	// rejects an empty string, so the region can't be configured as one.
	if len(labels) > 0 || !state.BucketLabels.IsNull() {
		state.BucketLabels = labelMap
	}
	state.Region = types.StringNull()
	if region := targetTemplate.Region.Name(); region != "" {
		state.Region = types.StringValue(region)
	}
	state.CustomerManagedKey = cmkSet

	state.BucketPrefix = types.StringValue(strings.TrimPrefix(targetTemplate.BucketPrefix, implicitPrefix))
	state.CloudAccountID = types.StringValue(targetTemplate.CloudAccount.ID.String())
	state.ConnectionStatus = types.StringValue(targetMapping.ConnectionStatus.Status)
	state.LocationTemplate = types.StringValue(targetTemplate.LocTemplate)
	state.Name = types.StringValue(targetMapping.Name)
	state.StorageClass = types.StringValue(strings.TrimSuffix(targetTemplate.StorageClass, "_GCP"))

	// RSC only lists the groups encrypted with a key from a key manager. A group
	// which isn't listed has no such key, and the kms_key block is left as it is.
	details, err := archival.Wrap(polarisClient).ArchivalGroupEncryptionDetailsByID(ctx, id)
	if err != nil && !errors.Is(err, graphql.ErrNotFound) {
		res.Diagnostics.AddError("Failed to read GCP archival location encryption details", err.Error())
		return
	}
	if err == nil && details.KMSKey != nil && details.KMSKey.Key != nil && details.KMSKey.Key.GCPKMSKey != nil {
		list, diags := types.ListValueFrom(ctx, gcpArchivalLocationKMSKeyType, []gcpArchivalLocationKMSKeyModel{{
			KeyManagerID: types.StringValue(details.KMSKey.KeyManagerID),
			KeyName:      types.StringValue(details.KMSKey.Key.GCPKMSKey.KeyName),
			KeyVersion:   types.StringValue(details.KMSKey.Key.GCPKMSKey.KeyVersion),
		}})
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return
		}
		state.KMSKey = list
	}
	// After an import the block is null. Terraform holds an absent block as
	// an empty list.
	if state.KMSKey.IsNull() {
		state.KMSKey = types.ListValueMust(gcpArchivalLocationKMSKeyType, nil)
	}

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.Identity.Set(ctx, gcpArchivalLocationIdentityModel{ID: state.ID})...)
}

func (r *gcpArchivalLocationResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "gcpArchivalLocationResource.Update")

	var plan gcpArchivalLocationResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	var state gcpArchivalLocationResourceModel
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

	var labels map[string]string
	res.Diagnostics.Append(plan.BucketLabels.ElementsAs(ctx, &labels, false)...)
	var cmks []gcpArchivalLocationCustomerManagedKeyModel
	res.Diagnostics.Append(plan.CustomerManagedKey.ElementsAs(ctx, &cmks, false)...)
	var kmsKeys []gcpArchivalLocationKMSKeyModel
	res.Diagnostics.Append(plan.KMSKey.ElementsAs(ctx, &kmsKeys, false)...)
	if res.Diagnostics.HasError() {
		return
	}

	// The rekey runs before the update, a failed rekey then leaves the other
	// arguments unchanged. Adding or removing the key, or changing the key
	// manager, replaces the archival location, so a difference here is always
	// a change to the key name or version.
	if !plan.KMSKey.Equal(state.KMSKey) && len(kmsKeys) > 0 {
		keyManagerID, err := uuid.Parse(kmsKeys[0].KeyManagerID.ValueString())
		if err != nil {
			res.Diagnostics.AddAttributeError(path.Root(keyKMSKey), "Invalid key manager ID", err.Error())
			return
		}

		timeout, diags := plan.Timeouts.Update(ctx, defaultGCPArchivalLocationRekeyTimeout)
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return
		}
		rekeyCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		err = archival.Wrap(polarisClient).RekeyMasterKey(rekeyCtx, id, gqlarchival.RekeyArchivalGroupMasterKeyParams{
			GCPKMSKey: &gqlarchival.GCPKMSKey{
				KeyManagerID: keyManagerID,
				KeyName:      kmsKeys[0].KeyName.ValueString(),
				KeyVersion:   kmsKeys[0].KeyVersion.ValueString(),
			},
		})
		if err != nil {
			res.Diagnostics.AddError("Failed to rekey GCP archival location", err.Error())
			return
		}
	}

	// The API doesn't support updating all fields. The labels are always sent,
	// an empty list removes them.
	updateParams := gqlarchival.UpdateGCPStorageSettingParams{
		Name:         plan.Name.ValueString(),
		StorageClass: plan.StorageClass.ValueString() + "_GCP",
		BucketLabels: core.Tags{TagList: make([]core.Tag, 0, len(labels))},
	}
	for key, value := range labels {
		updateParams.BucketLabels.TagList = append(updateParams.BucketLabels.TagList, core.Tag{Key: key, Value: value})
	}
	for _, cmk := range cmks {
		updateParams.CMKInfo = append(updateParams.CMKInfo, gqlarchival.GCPCustomerKey{
			Name:     cmk.Name.ValueString(),
			RingName: cmk.RingName.ValueString(),
			Region:   gqlgcp.RegionFromName(cmk.Region.ValueString()).ToRegionEnum(),
		})
	}
	if err := archival.Wrap(polarisClient).UpdateGCPStorageSetting(ctx, id, updateParams); err != nil {
		res.Diagnostics.AddError("Failed to update GCP archival location", err.Error())
		return
	}

	// The SDKv2 resource didn't read back after an update, the computed values
	// are kept from state and refreshed by the next Read.
	plan.ID = state.ID
	plan.ConnectionStatus = state.ConnectionStatus
	plan.LocationTemplate = state.LocationTemplate

	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.Identity.Set(ctx, gcpArchivalLocationIdentityModel{ID: plan.ID})...)
}

func (r *gcpArchivalLocationResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "gcpArchivalLocationResource.Delete")

	var state gcpArchivalLocationResourceModel
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
	if err != nil && !errors.Is(err, graphql.ErrNotFound) {
		res.Diagnostics.AddError("Failed to delete GCP archival location", err.Error())
		return
	}
}

func (r *gcpArchivalLocationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "gcpArchivalLocationResource.ImportState")

	resource.ImportStatePassthroughWithIdentity(ctx, path.Root(keyID), path.Root(keyID), req, res)
}
