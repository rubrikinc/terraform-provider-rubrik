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
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func (r *azureArchivalLocationResource) MoveState(ctx context.Context) []resource.StateMover {
	tflog.Trace(ctx, "azureArchivalLocationResource.MoveState")

	return []resource.StateMover{
		r.moveStateV0(ctx),
	}
}

// moveStateV0 moves v0 state from the polaris_azure_archival_location resource
// in the rubrikinc/polaris provider, or from the deprecated alias in this
// provider, to the rubrik_azure_archival_location resource. The SourceSchema is
// an inline literal frozen at the SDKv2 v0 schema of this provider. The polaris
// provider's v0 schema lacks the key_vault_key and timeouts blocks, and in older
// versions the network_access_type attribute, which the framework then decodes
// as null.
func (r *azureArchivalLocationResource) moveStateV0(ctx context.Context) resource.StateMover {
	return resource.StateMover{
		SourceSchema: &schema.Schema{
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
		},
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "azureArchivalLocationResource.moveStateV0")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolaris+"_"+keyAzureArchivalLocation {
				return
			}
			if req.SourceSchemaVersion != 0 {
				return
			}

			var state azureArchivalLocationResourceModel
			res.Diagnostics.Append(req.SourceState.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			// SDKv2 stores the region of a source region archival location as
			// the empty string, the framework resource stores it as null.
			if state.StorageAccountRegion.ValueString() == "" {
				state.StorageAccountRegion = types.StringNull()
			}

			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}
