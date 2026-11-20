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

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func (r *azureArchivalLocationResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	tflog.Trace(ctx, "azureArchivalLocationResource.UpgradeState")

	return map[int64]resource.StateUpgrader{
		0: r.upgradeStateV0(ctx),
	}
}

// upgradeStateV0 upgrades v0 state to v1. The resource stored an unset storage
// account region as an empty string. It's stored as null, matching a
// configuration that leaves it out.
func (r *azureArchivalLocationResource) upgradeStateV0(ctx context.Context) resource.StateUpgrader {
	return resource.StateUpgrader{
		PriorSchema: azureArchivalLocationSchemaV0(ctx),
		StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, res *resource.UpgradeStateResponse) {
			tflog.Trace(ctx, "azureArchivalLocationResource.upgradeStateV0")

			var state azureArchivalLocationResourceModel
			res.Diagnostics.Append(req.State.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			if state.StorageAccountRegion.ValueString() == "" {
				state.StorageAccountRegion = types.StringNull()
			}
			res.Diagnostics.Append(res.State.Set(ctx, &state)...)
		},
	}
}

// azureArchivalLocationSchemaV0 returns the SDKv2 v0 schema of the resource in
// this provider. The polaris provider's v0 schema lacks the key_vault_key and
// timeouts blocks, and in older versions the network_access_type attribute,
// which the framework then decodes as null.
func azureArchivalLocationSchemaV0(ctx context.Context) *schema.Schema {
	return &schema.Schema{
		Attributes: map[string]schema.Attribute{
			keyID:                       schema.StringAttribute{Computed: true},
			keyCloudAccountID:           schema.StringAttribute{Required: true},
			keyConnectionStatus:         schema.StringAttribute{Computed: true},
			keyContainerName:            schema.StringAttribute{Computed: true},
			keyLocationTemplate:         schema.StringAttribute{Computed: true},
			keyName:                     schema.StringAttribute{Required: true},
			keyNetworkAccessType:        schema.StringAttribute{Optional: true, Computed: true},
			keyRedundancy:               schema.StringAttribute{Optional: true},
			keyStorageAccountNamePrefix: schema.StringAttribute{Required: true},
			keyStorageAccountRegion:     schema.StringAttribute{Optional: true},
			keyStorageAccountTags:       schema.MapAttribute{ElementType: types.StringType, Optional: true},
			keyStorageTier:              schema.StringAttribute{Optional: true},
		},
		Blocks: map[string]schema.Block{
			keyCustomerManagedKey: schema.SetNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyName:      schema.StringAttribute{Required: true},
						keyRegion:    schema.StringAttribute{Required: true},
						keyVaultName: schema.StringAttribute{Required: true},
					},
				},
			},
			keyKeyVaultKey: schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyKeyManagerID: schema.StringAttribute{Required: true},
						keyKeyName:      schema.StringAttribute{Required: true},
						keyKeyVersion:   schema.StringAttribute{Required: true},
					},
				},
			},
			keyTimeouts: timeouts.Block(ctx, timeouts.Opts{Update: true}),
		},
	}
}
