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
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/archival"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql"
	gqlarchival "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/archival"
)

const dataSourceAzureArchivalLocationDescription = `
The ´rubrik_azure_archival_location´ data source is used to access information
about an Azure archival location. An archival location is looked up using either
the ID or the name.
`

var _ datasource.DataSource = &azureArchivalLocationDataSource{}

type azureArchivalLocationDataSource struct {
	client *client
	prefix string
}

type azureArchivalLocationModel struct {
	ID                       types.String `tfsdk:"id"`
	ArchivalLocationID       types.String `tfsdk:"archival_location_id"`
	CloudAccountID           types.String `tfsdk:"cloud_account_id"`
	ConnectionStatus         types.String `tfsdk:"connection_status"`
	ContainerName            types.String `tfsdk:"container_name"`
	CustomerManagedKey       types.Set    `tfsdk:"customer_managed_key"`
	KeyVaultKey              types.List   `tfsdk:"key_vault_key"`
	LocationTemplate         types.String `tfsdk:"location_template"`
	Name                     types.String `tfsdk:"name"`
	NetworkAccessType        types.String `tfsdk:"network_access_type"`
	Redundancy               types.String `tfsdk:"redundancy"`
	StorageAccountNamePrefix types.String `tfsdk:"storage_account_name_prefix"`
	StorageAccountRegion     types.String `tfsdk:"storage_account_region"`
	StorageAccountTags       types.Map    `tfsdk:"storage_account_tags"`
	StorageTier              types.String `tfsdk:"storage_tier"`
}

func newAzureArchivalLocationDataSource() datasource.DataSource {
	return &azureArchivalLocationDataSource{prefix: keyRubrik}
}

func newPolarisAzureArchivalLocationDataSource() datasource.DataSource {
	return &azureArchivalLocationDataSource{prefix: keyPolaris}
}

func (d *azureArchivalLocationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	tflog.Trace(ctx, "azureArchivalLocationDataSource.Metadata")

	res.TypeName = d.prefix + "_" + keyAzureArchivalLocation
}

func (d *azureArchivalLocationDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	tflog.Trace(ctx, "azureArchivalLocationDataSource.Schema")

	res.Schema = schema.Schema{
		Description: description(dataSourceAzureArchivalLocationDescription),
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Cloud native archival location ID (UUID).",
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.MatchRoot(keyArchivalLocationID), path.MatchRoot(keyName)),
					isUUID(),
				},
			},
			keyArchivalLocationID: schema.StringAttribute{
				Optional:           true,
				Computed:           true,
				Description:        "Cloud native archival location ID (UUID). **Deprecated:** use `id` instead.",
				DeprecationMessage: "Use `id` instead.",
				Validators: []validator.String{
					isUUID(),
				},
			},
			keyCloudAccountID: schema.StringAttribute{
				Computed:    true,
				Description: "RSC cloud account ID (UUID).",
			},
			keyConnectionStatus: schema.StringAttribute{
				Computed:    true,
				Description: "Connection status of the cloud native archival location.",
			},
			keyContainerName: schema.StringAttribute{
				Computed:    true,
				Description: "Azure storage container name.",
			},
			keyCustomerManagedKey: schema.SetNestedAttribute{
				Computed: true,
				Description: "Customer managed storage encryption. For `SPECIFIC_REGION`, a customer managed key " +
					"for the specific region will be returned. For `SOURCE_REGION`, a customer managed key for each " +
					"source region will be returned, for other regions, data will be encrypted using platform " +
					"managed keys. Empty when the archival location is encrypted with a key vault key.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyName: schema.StringAttribute{
							Computed:    true,
							Description: "Key name.",
						},
						keyRegion: schema.StringAttribute{
							Computed:    true,
							Description: "The region in which the key will be used.",
						},
						keyVaultName: schema.StringAttribute{
							Computed:    true,
							Description: "Key vault name.",
						},
					},
				},
			},
			keyKeyVaultKey: schema.ListNestedAttribute{
				Computed: true,
				Description: "Azure Key Vault key used for client-side encryption of the archived data. Empty " +
					"when the archival location isn't encrypted with a key from a key manager.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						keyKeyManagerID: schema.StringAttribute{
							Computed:    true,
							Description: "RSC key manager ID (UUID).",
						},
						keyKeyName: schema.StringAttribute{
							Computed:    true,
							Description: "Key Vault key name.",
						},
						keyKeyVersion: schema.StringAttribute{
							Computed:    true,
							Description: "Key Vault key version.",
						},
					},
				},
			},
			keyLocationTemplate: schema.StringAttribute{
				Computed: true,
				Description: "RSC location template. If a storage account region was specified, it will be " +
					"`SPECIFIC_REGION`, otherwise `SOURCE_REGION`.",
			},
			keyName: schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Cloud native archival location name.",
				Validators: []validator.String{
					isNotWhiteSpace(),
				},
			},
			keyNetworkAccessType: schema.StringAttribute{
				Computed: true,
				Description: "Azure storage account network access type. Possible values are `PRIVATE`, `PUBLIC` and " +
					"`SELECTED_NETWORKS`.",
			},
			keyRedundancy: schema.StringAttribute{
				Computed: true,
				Description: "Azure storage redundancy. Possible values are `GRS`, `GZRS`, `LRS`, `RA_GRS`, " +
					"`RA_GZRS` and `ZRS`. Default value is `LRS`.",
			},
			keyStorageAccountNamePrefix: schema.StringAttribute{
				Computed: true,
				Description: "Azure storage account name prefix. For `SOURCE_REGION`, the prefix cannot be " +
					"longer than 16 characters. For `SPECIFIC_REGION`, the name cannot be longer than 24 " +
					"characters. The value can only consist of numbers and lower case letters.",
			},
			keyStorageAccountRegion: schema.StringAttribute{
				Computed: true,
				Description: "Azure region to store the snapshots in (`SPECIFIC_REGION`). If not specified, the " +
					"snapshots will be stored in the same region as the workload (`SOURCE_REGION`).",
			},
			keyStorageAccountTags: schema.MapAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Azure storage account tags. Each tag will be added to the storage account created by " +
					"RSC.",
			},
			keyStorageTier: schema.StringAttribute{
				Computed:    true,
				Description: "Azure storage tier. Possible values are `COOL` and `HOT`. Default value is `COOL`.",
			},
		},
	}

	if d.prefix == keyPolaris {
		res.Schema.DeprecationMessage = "use the `rubrik_azure_archival_location` data source instead."
	}
}

func (d *azureArchivalLocationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	tflog.Trace(ctx, "azureArchivalLocationDataSource.Configure")

	if req.ProviderData == nil {
		return
	}
	d.client = req.ProviderData.(*client)
}

func (d *azureArchivalLocationDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	tflog.Trace(ctx, "azureArchivalLocationDataSource.Read")

	var config azureArchivalLocationModel
	res.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if res.Diagnostics.HasError() {
		return
	}

	polarisClient, err := d.client.polaris()
	if err != nil {
		res.Diagnostics.AddError("RSC client error", err.Error())
		return
	}

	// Read the archival location using either the ID or the name.
	var targetMapping gqlarchival.AzureTargetMapping
	targetMappingID := config.ID.ValueString()
	if targetMappingID == "" {
		targetMappingID = config.ArchivalLocationID.ValueString()
	}
	if targetMappingID != "" {
		id, err := uuid.Parse(targetMappingID)
		if err != nil {
			res.Diagnostics.AddError("Invalid archival location ID", err.Error())
			return
		}
		targetMapping, err = archival.Wrap(polarisClient).AzureTargetMappingByID(ctx, id)
		if err != nil {
			res.Diagnostics.AddError("Failed to read Azure archival location", err.Error())
			return
		}
	} else {
		targetMapping, err = archival.Wrap(polarisClient).AzureTargetMappingByName(ctx, config.Name.ValueString())
		if err != nil {
			res.Diagnostics.AddError("Failed to read Azure archival location", err.Error())
			return
		}
	}

	targetTemplate := targetMapping.TargetTemplate
	cloudNativeCompanion := targetTemplate.CloudNativeCompanion

	tagValues := make(map[string]attr.Value, len(cloudNativeCompanion.StorageAccountTags))
	for _, tag := range cloudNativeCompanion.StorageAccountTags {
		tagValues[tag.Key] = types.StringValue(tag.Value)
	}
	storageAccountTags, diags := types.MapValue(types.StringType, tagValues)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	// RSC only lists the groups encrypted with a key from a key manager.
	// A group which isn't listed has no such key.
	details, err := archival.Wrap(polarisClient).ArchivalGroupEncryptionDetailsByID(ctx, targetMapping.ID)
	if err != nil && !errors.Is(err, graphql.ErrNotFound) {
		res.Diagnostics.AddError("Failed to read Azure archival location encryption details", err.Error())
		return
	}
	keyVaultKey, diags := fromAzureKeyVaultKey(ctx, details)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	// RSC keeps customer managed keys given together with a key vault key,
	// even though they aren't used, so they aren't returned for a location
	// with a key vault key.
	cmkInfo := cloudNativeCompanion.CMKInfo
	if len(keyVaultKey.Elements()) > 0 {
		cmkInfo = nil
	}
	customerKeys, diags := fromAzureCustomerKeys(ctx, cmkInfo)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	state := azureArchivalLocationModel{
		ID:                       types.StringValue(targetMapping.ID.String()),
		ArchivalLocationID:       types.StringValue(targetMapping.ID.String()),
		CloudAccountID:           types.StringValue(targetTemplate.CloudAccount.ID.String()),
		ConnectionStatus:         types.StringValue(targetMapping.ConnectionStatus.Status),
		ContainerName:            types.StringValue(targetTemplate.ContainerNamePrefix),
		CustomerManagedKey:       customerKeys,
		KeyVaultKey:              keyVaultKey,
		LocationTemplate:         types.StringValue(cloudNativeCompanion.LocTemplate),
		Name:                     types.StringValue(targetMapping.Name),
		NetworkAccessType:        types.StringValue(cloudNativeCompanion.NetworkAccessType),
		Redundancy:               types.StringValue(cloudNativeCompanion.Redundancy),
		StorageAccountNamePrefix: types.StringValue(targetTemplate.StorageAccountName),
		StorageAccountRegion:     types.StringValue(cloudNativeCompanion.StorageAccountRegion.Name()),
		StorageAccountTags:       storageAccountTags,
		StorageTier:              types.StringValue(cloudNativeCompanion.StorageTier),
	}

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}
