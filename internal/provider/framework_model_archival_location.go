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
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	gqlarchival "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/archival"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/regions/azure"
)

// azureArchivalLocationCustomerKeyType is the element type of the
// customer_managed_key block.
var azureArchivalLocationCustomerKeyType = types.ObjectType{AttrTypes: map[string]attr.Type{
	keyName:      types.StringType,
	keyRegion:    types.StringType,
	keyVaultName: types.StringType,
}}

type azureArchivalLocationCustomerKeyModel struct {
	Name      types.String `tfsdk:"name"`
	Region    types.String `tfsdk:"region"`
	VaultName types.String `tfsdk:"vault_name"`
}

// fromAzureCustomerKeys converts the SDK customer keys into the
// customer_managed_key blocks. It returns an empty set when there are no
// customer keys.
func fromAzureCustomerKeys(ctx context.Context, keys []gqlarchival.AzureCustomerKey) (types.Set, diag.Diagnostics) {
	customerKeys := make([]azureArchivalLocationCustomerKeyModel, 0, len(keys))
	for _, key := range keys {
		customerKeys = append(customerKeys, azureArchivalLocationCustomerKeyModel{
			Name:      types.StringValue(key.Name),
			Region:    types.StringValue(key.Region.Name()),
			VaultName: types.StringValue(key.VaultName),
		})
	}

	return types.SetValueFrom(ctx, azureArchivalLocationCustomerKeyType, customerKeys)
}

// toAzureCustomerKeys converts the customer_managed_key blocks into the SDK
// customer key parameter. It returns nil when no blocks are given.
func toAzureCustomerKeys(ctx context.Context, set types.Set) ([]gqlarchival.AzureCustomerKey, diag.Diagnostics) {
	var diags diag.Diagnostics
	if set.IsNull() || set.IsUnknown() {
		return nil, diags
	}

	var keys []azureArchivalLocationCustomerKeyModel
	diags.Append(set.ElementsAs(ctx, &keys, false)...)
	if diags.HasError() {
		return nil, diags
	}

	var customerKeys []gqlarchival.AzureCustomerKey
	for _, key := range keys {
		customerKeys = append(customerKeys, gqlarchival.AzureCustomerKey{
			Name:      key.Name.ValueString(),
			VaultName: key.VaultName.ValueString(),
			Region:    azure.RegionFromName(key.Region.ValueString()).ToRegionEnum(),
		})
	}

	return customerKeys, diags
}

// azureKeyVaultKeyType is the element type of the key_vault_key block.
var azureKeyVaultKeyType = types.ObjectType{AttrTypes: map[string]attr.Type{
	keyKeyManagerID: types.StringType,
	keyKeyName:      types.StringType,
	keyKeyVersion:   types.StringType,
}}

type azureKeyVaultKeyModel struct {
	KeyManagerID types.String `tfsdk:"key_manager_id"`
	KeyName      types.String `tfsdk:"key_name"`
	KeyVersion   types.String `tfsdk:"key_version"`
}

// toAzureKeyVaultKey converts the key_vault_key block into the SDK Key Vault
// key parameter. It returns nil when the block is left out.
func toAzureKeyVaultKey(ctx context.Context, list types.List) (*gqlarchival.AzureKeyVaultKey, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		return nil, diags
	}

	var keys []azureKeyVaultKeyModel
	diags.Append(list.ElementsAs(ctx, &keys, false)...)
	if diags.HasError() || len(keys) == 0 {
		return nil, diags
	}

	keyManagerID, err := uuid.Parse(keys[0].KeyManagerID.ValueString())
	if err != nil {
		diags.AddAttributeError(path.Root(keyKeyVaultKey), "Invalid key manager ID", err.Error())
		return nil, diags
	}

	return &gqlarchival.AzureKeyVaultKey{
		KeyVaultID: keyManagerID,
		KeyName:    keys[0].KeyName.ValueString(),
		KeyVersion: keys[0].KeyVersion.ValueString(),
	}, diags
}

// fromAzureKeyVaultKey converts the Azure Key Vault key of the encryption
// details into the key_vault_key block. It returns an empty list when the
// archival group isn't encrypted with an Azure Key Vault key.
func fromAzureKeyVaultKey(ctx context.Context, details gqlarchival.ArchivalGroupEncryptionDetails) (types.List, diag.Diagnostics) {
	keys := make([]azureKeyVaultKeyModel, 0, 1)
	if details.KMSKey != nil && details.KMSKey.Key != nil && details.KMSKey.Key.AzureKey != nil {
		keys = append(keys, azureKeyVaultKeyModel{
			KeyManagerID: types.StringValue(details.KMSKey.KeyManagerID),
			KeyName:      types.StringValue(details.KMSKey.Key.AzureKey.KeyName),
			KeyVersion:   types.StringValue(details.KMSKey.Key.AzureKey.KeyVersion),
		})
	}

	return types.ListValueFrom(ctx, azureKeyVaultKeyType, keys)
}
