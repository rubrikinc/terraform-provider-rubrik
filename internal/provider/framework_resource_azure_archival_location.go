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
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
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
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/azure"
)

const resourceAzureArchivalLocationDescription = `
The ´rubrik_azure_archival_location´ resource creates an RSC archival location
for cloud-native workloads. This resource requires that the Azure subscription
has been onboarded with the ´cloud_native_archival´ feature.

When creating an archival location, the region where the snapshots are stored
needs to be specified:
  * ´SOURCE_REGION´ - Store snapshots in the same region to minimize data
    transfer charges. This is the default behaviour when the
    ´storage_account_region´ field is not specified.
  * ´SPECIFIC_REGION´ - Storing snapshots in another region can increase total
    data transfer charges. The ´storage_account_region´ field specifies the
    region.

Custom storage encryption is enabled by specifying one or more customer managed
key blocks. For ´SPECIFIC_REGION´, a customer managed key block for the specific
region must be specified. For ´SOURCE_REGION´, a customer managed key block for
each source region should be specified, source regions not having a customer
managed key block will have its data encrypted with platform managed keys.

Client-side encryption is enabled by specifying the ´key_vault_key´ block, with
a key in an Azure Key Vault registered as a key manager in RSC. Register the key
manager with the ´rubrik_azure_key_manager´ resource, or look up an existing
one with the ´rubrik_azure_key_manager´ data source. The ´key_vault_key´
block conflicts with the customer managed key blocks.

The key vault key has the following requirements, which RSC does not check when
the archival location is created:
  * The key vault must be registered as a key manager for a subscription
    onboarded with the ´cloud_native_uem_key_management´ feature, see the
    ´rubrik_azure_subscription´ resource.
  * The service principal of the key manager's subscription must be assigned the
    Key Vault Crypto User role on the key vault.
  * Exocompute performs the encryption, so the key manager's subscription must
    be mapped to Exocompute in the key vault's region.

Changing the key name or the key version of the ´key_vault_key´ block rekeys the
archival location in place, without re-encrypting the archived data. The rekey
can take several minutes, increase ´timeouts.update´ if it takes longer than the
default of 20 minutes. Changing the key manager, or adding or removing the
block, replaces the archival location.

-> **Note:** When using ´SOURCE_REGION´ the Azure storage account isn't created
   until the first protected object is archived.
`

// defaultAzureArchivalLocationUpdateTimeout is the default time to wait for an
// update of an Azure archival location, including a rekey, to complete.
const defaultAzureArchivalLocationUpdateTimeout = 20 * time.Minute

var (
	_ resource.Resource                     = &azureArchivalLocationResource{}
	_ resource.ResourceWithConfigValidators = &azureArchivalLocationResource{}
	_ resource.ResourceWithValidateConfig   = &azureArchivalLocationResource{}
	_ resource.ResourceWithIdentity         = &azureArchivalLocationResource{}
	_ resource.ResourceWithImportState      = &azureArchivalLocationResource{}
	_ resource.ResourceWithMoveState        = &azureArchivalLocationResource{}
)

type azureArchivalLocationResource struct {
	client *client
	prefix string
}

type azureArchivalLocationResourceModel struct {
	ID                       types.String   `tfsdk:"id"`
	CloudAccountID           types.String   `tfsdk:"cloud_account_id"`
	ConnectionStatus         types.String   `tfsdk:"connection_status"`
	ContainerName            types.String   `tfsdk:"container_name"`
	CustomerManagedKey       types.Set      `tfsdk:"customer_managed_key"`
	KeyVaultKey              types.List     `tfsdk:"key_vault_key"`
	LocationTemplate         types.String   `tfsdk:"location_template"`
	Name                     types.String   `tfsdk:"name"`
	NetworkAccessType        types.String   `tfsdk:"network_access_type"`
	Redundancy               types.String   `tfsdk:"redundancy"`
	StorageAccountNamePrefix types.String   `tfsdk:"storage_account_name_prefix"`
	StorageAccountRegion     types.String   `tfsdk:"storage_account_region"`
	StorageAccountTags       types.Map      `tfsdk:"storage_account_tags"`
	StorageTier              types.String   `tfsdk:"storage_tier"`
	Timeouts                 timeouts.Value `tfsdk:"timeouts"`
}

type azureArchivalLocationIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

func newAzureArchivalLocationResource() resource.Resource {
	return &azureArchivalLocationResource{prefix: keyRubrik}
}

func newPolarisAzureArchivalLocationResource() resource.Resource {
	return &azureArchivalLocationResource{prefix: keyPolaris}
}

func (r *azureArchivalLocationResource) Metadata(ctx context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	tflog.Trace(ctx, "azureArchivalLocationResource.Metadata")

	res.TypeName = r.prefix + "_" + keyAzureArchivalLocation
}

func (r *azureArchivalLocationResource) Schema(ctx context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	tflog.Trace(ctx, "azureArchivalLocationResource.Schema")

	res.Schema = schema.Schema{
		Description: description(resourceAzureArchivalLocationDescription),
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
			keyConnectionStatus: schema.StringAttribute{
				Computed:    true,
				Description: "Connection status of the cloud native archival location.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyContainerName: schema.StringAttribute{
				Computed:    true,
				Description: "Azure storage container name.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyLocationTemplate: schema.StringAttribute{
				Computed: true,
				Description: "RSC location template. If a storage account region was specified, it will be " +
					"`SPECIFIC_REGION`, otherwise `SOURCE_REGION`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			keyName: schema.StringAttribute{
				Required:    true,
				Description: "Cloud native archival location name.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyNetworkAccessType: schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Azure storage account network access type. Possible values are `PRIVATE`, `PUBLIC` and " +
					"`SELECTED_NETWORKS`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("PRIVATE", "PUBLIC", "SELECTED_NETWORKS"),
				},
			},
			keyRedundancy: schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("LRS"),
				Description: "Azure storage redundancy. Possible values are `GRS`, `GZRS`, `LRS`, `RA_GRS`, " +
					"`RA_GZRS` and `ZRS`. Default value is `LRS`. Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("GRS", "GZRS", "LRS", "RA_GRS", "RA_GZRS", "ZRS"),
				},
			},
			keyStorageAccountNamePrefix: schema.StringAttribute{
				Required: true,
				Description: "Azure storage account name prefix. When `storage_account_region` is not specified " +
					"(`SOURCE_REGION`), the prefix cannot be longer than 16 characters. When `storage_account_region` " +
					"is specified (`SPECIFIC_REGION`), the name cannot be longer than 24 characters. The value can " +
					"only consist of numbers and lower case letters. Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 24),
					stringvalidator.RegexMatches(regexp.MustCompile("^[a-z0-9]*$"),
						"storage account name may only contain numbers and lowercase letters"),
				},
			},
			keyStorageAccountRegion: schema.StringAttribute{
				Optional: true,
				Description: "Azure region to store the snapshots in. If not specified, the snapshots will be stored " +
					"in the same region as the workload. Changing this forces a new resource to be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(azure.RegionEnumRegionNames()...),
				},
			},
			keyStorageAccountTags: schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Azure storage account tags. Each tag will be added to the storage account created by " +
					"RSC.",
			},
			keyStorageTier: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("COOL"),
				Description: "Azure storage tier. Possible values are `COOL` and `HOT`. Default value is `COOL`.",
				Validators: []validator.String{
					stringvalidator.OneOf("COOL", "HOT"),
				},
			},
		},
		Blocks: map[string]schema.Block{
			keyCustomerManagedKey: schema.SetNestedBlock{
				Description: "Customer managed storage encryption. For `SPECIFIC_REGION`, a customer managed key " +
					"block for the specific region must be specified. For `SOURCE_REGION`, a customer managed key " +
					"block for each source region should be specified, source regions not having a customer managed " +
					"key block will have its data encrypted with platform managed keys. Conflicts with " +
					"`key_vault_key`.",
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
								stringvalidator.OneOf(azure.RegionEnumRegionNames()...),
							},
						},
						keyVaultName: schema.StringAttribute{
							Required:    true,
							Description: "Key vault name.",
							Validators: []validator.String{
								isNotWhiteSpace(),
							},
						},
					},
				},
			},
			// A single-element list block, as in SDKv2. Only adding or removing
			// the block replaces the archival location, changing the key name
			// or the key version rekeys it in place.
			keyKeyVaultKey: schema.ListNestedBlock{
				Description: "Azure Key Vault key used for client-side encryption of the archived data. Conflicts " +
					"with `customer_managed_key`. May be specified at most once. Changing the key name or the key " +
					"version rekeys the archival location in place. Changing the key manager, or adding or removing " +
					"the block, forces a new resource to be created.",
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
			keyTimeouts: timeouts.Block(ctx, timeouts.Opts{
				Update: true,
				UpdateDescription: fmt.Sprintf("How long to wait for an update of the archival location, "+
					"including a rekey, to complete. Default is `%s`.", defaultAzureArchivalLocationUpdateTimeout),
			}),
		},
	}

	if r.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use `rubrik_azure_archival_location` instead."
	}
}

func (r *azureArchivalLocationResource) IdentitySchema(ctx context.Context, _ resource.IdentitySchemaRequest, res *resource.IdentitySchemaResponse) {
	tflog.Trace(ctx, "azureArchivalLocationResource.IdentitySchema")

	res.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			keyID: identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Cloud native archival location ID (UUID).",
			},
		},
	}
}

func (r *azureArchivalLocationResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	tflog.Trace(ctx, "azureArchivalLocationResource.ConfigValidators")

	return []resource.ConfigValidator{
		resourcevalidator.Conflicting(
			path.MatchRoot(keyCustomerManagedKey),
			path.MatchRoot(keyKeyVaultKey),
		),
	}
}

func (r *azureArchivalLocationResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, res *resource.ValidateConfigResponse) {
	tflog.Trace(ctx, "azureArchivalLocationResource.ValidateConfig")

	var prefix, region types.String
	res.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(keyStorageAccountNamePrefix), &prefix)...)
	res.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(keyStorageAccountRegion), &region)...)
	if res.Diagnostics.HasError() {
		return
	}
	if prefix.IsUnknown() || region.IsUnknown() {
		return
	}

	name := prefix.ValueString()
	if region.ValueString() != "" {
		// SPECIFIC_REGION: the name is used directly as the Azure storage
		// account name, so it must be between 3 and 24 characters.
		if len(name) < 3 {
			res.Diagnostics.AddAttributeError(path.Root(keyStorageAccountNamePrefix), "Invalid storage account name prefix",
				"storage_account_name_prefix must be at least 3 characters when storage_account_region is specified")
		}
	} else {
		// SOURCE_REGION: an 8-character random UID is appended to the prefix,
		// so it must not be longer than 16 characters.
		if len(name) > 16 {
			res.Diagnostics.AddAttributeError(path.Root(keyStorageAccountNamePrefix), "Invalid storage account name prefix",
				"storage_account_name_prefix must not be longer than 16 characters when storage_account_region is not specified")
		}
	}
}

func (r *azureArchivalLocationResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	tflog.Trace(ctx, "azureArchivalLocationResource.Configure")

	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client)
}

func (r *azureArchivalLocationResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	tflog.Trace(ctx, "azureArchivalLocationResource.Create")

	var plan azureArchivalLocationResourceModel
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

	// Tags are only sent when there are any.
	var storageAccountTags *core.Tags
	if len(plan.StorageAccountTags.Elements()) > 0 {
		var tags map[string]string
		res.Diagnostics.Append(plan.StorageAccountTags.ElementsAs(ctx, &tags, false)...)
		if res.Diagnostics.HasError() {
			return
		}
		tagList := make([]core.Tag, 0, len(tags))
		for key, value := range tags {
			tagList = append(tagList, core.Tag{Key: key, Value: value})
		}
		storageAccountTags = &core.Tags{TagList: tagList}
	}

	customerKeys, diags := toAzureCustomerKeys(ctx, plan.CustomerManagedKey)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	keyVaultKey, diags := toAzureKeyVaultKey(ctx, plan.KeyVaultKey)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	id, err := archival.Wrap(polarisClient).CreateAzureStorageSetting(ctx, gqlarchival.CreateAzureStorageSettingParams{
		CloudAccountID:       cloudAccountID,
		Name:                 plan.Name.ValueString(),
		NetworkAccessType:    plan.NetworkAccessType.ValueString(),
		Redundancy:           plan.Redundancy.ValueString(),
		StorageTier:          plan.StorageTier.ValueString(),
		StorageAccountName:   plan.StorageAccountNamePrefix.ValueString(),
		StorageAccountRegion: azure.RegionFromName(plan.StorageAccountRegion.ValueString()).ToRegionEnum(),
		StorageAccountTags:   storageAccountTags,
		CMKInfo:              customerKeys,
		KeyVaultKey:          keyVaultKey,
	})
	if err != nil {
		res.Diagnostics.AddError("Failed to create Azure archival location", err.Error())
		return
	}

	// Save the ID to state and identity before the read-back so Terraform can
	// track the archival location even if the read fails.
	plan.ID = types.StringValue(id.String())
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := azureArchivalLocationIdentityModel{ID: plan.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
	if res.Diagnostics.HasError() {
		return
	}

	// The tags, the customer managed keys and the key vault key are kept as
	// planned. RSC doesn't store tags given on create, the next refresh shows
	// them as a difference which the following apply corrects.
	targetMapping, err := archival.Wrap(polarisClient).AzureTargetMappingByID(ctx, id)
	if err != nil {
		res.Diagnostics.AddWarning("Failed to read Azure archival location after create",
			fmt.Sprintf("The archival location was created successfully but the computed fields could not be "+
				"populated: %s", err.Error()))
		updateAzureArchivalLocationModel(&plan, nil)
	} else {
		updateAzureArchivalLocationModel(&plan, &targetMapping)
	}
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
}

func (r *azureArchivalLocationResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	tflog.Trace(ctx, "azureArchivalLocationResource.Read")

	var state azureArchivalLocationResourceModel
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

	targetMapping, err := archival.Wrap(polarisClient).AzureTargetMappingByID(ctx, id)
	if errors.Is(err, graphql.ErrNotFound) {
		res.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		res.Diagnostics.AddError("Failed to read Azure archival location", err.Error())
		return
	}

	updateAzureArchivalLocationModel(&state, &targetMapping)
	cloudNativeCompanion := targetMapping.TargetTemplate.CloudNativeCompanion

	// Without tags in RSC, a state without tags is kept as it is, so that
	// both a configuration without tags and an empty map plan cleanly.
	if len(cloudNativeCompanion.StorageAccountTags) > 0 {
		tagValues := make(map[string]attr.Value, len(cloudNativeCompanion.StorageAccountTags))
		for _, tag := range cloudNativeCompanion.StorageAccountTags {
			tagValues[tag.Key] = types.StringValue(tag.Value)
		}
		tags, diags := types.MapValue(types.StringType, tagValues)
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return
		}
		state.StorageAccountTags = tags
	} else if len(state.StorageAccountTags.Elements()) > 0 {
		state.StorageAccountTags = types.MapNull(types.StringType)
	}

	// RSC only lists the groups encrypted with a key from a key manager. A group
	// which isn't listed has no such key, and the key_vault_key block is left as
	// it is.
	details, err := archival.Wrap(polarisClient).ArchivalGroupEncryptionDetailsByID(ctx, id)
	if err != nil && !errors.Is(err, graphql.ErrNotFound) {
		res.Diagnostics.AddError("Failed to read Azure archival location encryption details", err.Error())
		return
	}
	keyVaultKey, diags := fromAzureKeyVaultKey(ctx, details)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	if len(keyVaultKey.Elements()) > 0 {
		state.KeyVaultKey = keyVaultKey
	}

	// RSC keeps customer managed keys given together with a key vault key, even
	// though they aren't used, so they are only read for a location without a
	// key vault key.
	if len(state.KeyVaultKey.Elements()) == 0 {
		customerManagedKey, diags := fromAzureCustomerKeys(ctx, cloudNativeCompanion.CMKInfo)
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return
		}
		state.CustomerManagedKey = customerManagedKey
	}

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := azureArchivalLocationIdentityModel{ID: state.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *azureArchivalLocationResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	tflog.Trace(ctx, "azureArchivalLocationResource.Update")

	var plan azureArchivalLocationResourceModel
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	var state azureArchivalLocationResourceModel
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

	// The timeout covers the whole update, the rekey and the update of the
	// other arguments.
	timeout, diags := plan.Timeouts.Update(ctx, defaultAzureArchivalLocationUpdateTimeout)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// The rekey runs before the update, a failed rekey then leaves the other
	// arguments unchanged. Adding or removing the key replaces the archival
	// location, so a difference here is always a change to the existing key.
	if !plan.KeyVaultKey.Equal(state.KeyVaultKey) {
		keyVaultKey, diags := toAzureKeyVaultKey(ctx, plan.KeyVaultKey)
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return
		}
		if keyVaultKey != nil {
			err := archival.Wrap(polarisClient).RekeyMasterKey(ctx, id, gqlarchival.RekeyArchivalGroupMasterKeyParams{
				AzureKeyVaultKey: keyVaultKey,
			})
			if err != nil {
				res.Diagnostics.AddError("Failed to rekey Azure archival location", err.Error())
				return
			}
		}
	}

	// Tags are always sent, an empty list removes them.
	var tags map[string]string
	if !plan.StorageAccountTags.IsNull() && !plan.StorageAccountTags.IsUnknown() {
		res.Diagnostics.Append(plan.StorageAccountTags.ElementsAs(ctx, &tags, false)...)
		if res.Diagnostics.HasError() {
			return
		}
	}
	tagList := make([]core.Tag, 0, len(tags))
	for key, value := range tags {
		tagList = append(tagList, core.Tag{Key: key, Value: value})
	}

	customerKeys, diags := toAzureCustomerKeys(ctx, plan.CustomerManagedKey)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	// Note, the API doesn't support updating all fields.
	err = archival.Wrap(polarisClient).UpdateAzureStorageSetting(ctx, id, gqlarchival.UpdateAzureStorageSettingParams{
		Name:               plan.Name.ValueString(),
		NetworkAccessType:  plan.NetworkAccessType.ValueString(),
		StorageTier:        plan.StorageTier.ValueString(),
		StorageAccountTags: core.Tags{TagList: tagList},
		CMKInfo:            customerKeys,
	})
	if err != nil {
		res.Diagnostics.AddError("Failed to update Azure archival location", err.Error())
		return
	}

	// The update isn't read back, the next refresh reads it.
	plan.ID = state.ID
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	identity := azureArchivalLocationIdentityModel{ID: plan.ID}
	res.Diagnostics.Append(res.Identity.Set(ctx, identity)...)
}

func (r *azureArchivalLocationResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	tflog.Trace(ctx, "azureArchivalLocationResource.Delete")

	var state azureArchivalLocationResourceModel
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
		res.Diagnostics.AddError("Failed to delete Azure archival location", err.Error())
		return
	}
}

func (r *azureArchivalLocationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	tflog.Trace(ctx, "azureArchivalLocationResource.ImportState")

	resource.ImportStatePassthroughWithIdentity(ctx, path.Root(keyID), path.Root(keyID), req, res)
}

// updateAzureArchivalLocationModel updates the model with the values read back
// from RSC. A nil target mapping means the read-back failed, computed values
// still unknown are then set to null so the model can be saved to state. Read
// fills them in on the next refresh. The tags, the customer managed keys and
// the key vault key are left untouched, the caller handles them.
func updateAzureArchivalLocationModel(m *azureArchivalLocationResourceModel, targetMapping *gqlarchival.AzureTargetMapping) {
	if targetMapping == nil {
		// Optional + Computed.
		if m.NetworkAccessType.IsUnknown() {
			m.NetworkAccessType = types.StringNull()
		}

		// Computed.
		if m.ConnectionStatus.IsUnknown() {
			m.ConnectionStatus = types.StringNull()
		}
		if m.ContainerName.IsUnknown() {
			m.ContainerName = types.StringNull()
		}
		if m.LocationTemplate.IsUnknown() {
			m.LocationTemplate = types.StringNull()
		}

		return
	}

	targetTemplate := targetMapping.TargetTemplate
	cloudNativeCompanion := targetTemplate.CloudNativeCompanion
	m.CloudAccountID = types.StringValue(targetTemplate.CloudAccount.ID.String())
	m.ConnectionStatus = types.StringValue(targetMapping.ConnectionStatus.Status)
	m.ContainerName = types.StringValue(targetTemplate.ContainerNamePrefix)
	m.LocationTemplate = types.StringValue(cloudNativeCompanion.LocTemplate)
	m.Name = types.StringValue(targetMapping.Name)
	m.NetworkAccessType = types.StringValue(cloudNativeCompanion.NetworkAccessType)
	m.Redundancy = types.StringValue(cloudNativeCompanion.Redundancy)
	m.StorageAccountNamePrefix = types.StringValue(targetTemplate.StorageAccountName)
	m.StorageTier = types.StringValue(cloudNativeCompanion.StorageTier)

	// A source region archival location has no region of its own, RSC returns
	// the unknown region, whose name is the empty string.
	if name := cloudNativeCompanion.StorageAccountRegion.Name(); name != "" {
		m.StorageAccountRegion = types.StringValue(name)
	} else {
		m.StorageAccountRegion = types.StringNull()
	}
}
