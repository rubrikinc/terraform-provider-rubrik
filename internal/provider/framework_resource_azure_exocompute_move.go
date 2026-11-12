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
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func (r *azureExocomputeResource) MoveState(ctx context.Context) []resource.StateMover {
	tflog.Trace(ctx, "azureExocomputeResource.MoveState")

	return []resource.StateMover{
		r.moveStateV0(),
		r.moveStateV1(ctx),
	}
}

// moveStateV0 moves v0 state from the polaris_azure_exocompute resource in the
// rubrikinc/polaris provider to the rubrik_azure_exocompute resource. It does
// the same conversion as the v0 state upgrader.
func (r *azureExocomputeResource) moveStateV0() resource.StateMover {
	return resource.StateMover{
		SourceSchema: &schema.Schema{
			Attributes: map[string]schema.Attribute{
				keyID:             schema.StringAttribute{Computed: true},
				keyPolarisManaged: schema.BoolAttribute{Optional: true},
				keyRegion:         schema.StringAttribute{Required: true},
				keySubnet:         schema.StringAttribute{Required: true},
				keySubscriptionID: schema.StringAttribute{Required: true},
			},
		},
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "azureExocomputeResource.moveStateV0")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolaris+"_"+keyAzureExocompute {
				return
			}
			if req.SourceSchemaVersion != 0 {
				return
			}

			var source azureExocomputeModelV0
			res.Diagnostics.Append(req.SourceState.Get(ctx, &source)...)
			if res.Diagnostics.HasError() {
				return
			}

			state := azureExocomputeModel{
				ID:                    source.ID,
				CloudAccountID:        types.StringNull(),
				HostCloudAccountID:    types.StringNull(),
				OptionalConfig:        types.ListValueMust(types.ObjectType{AttrTypes: azureExocomputeOptionalConfigAttrTypes}, []attr.Value{}),
				PodOverlayNetworkCIDR: types.StringValue(""),
				Region:                source.Region,
				Subnet:                source.Subnet,
				SubscriptionID:        source.SubscriptionID,
				TriggerHealthCheck:    types.BoolNull(),
				Timeouts:              azureExocomputeNullTimeouts,
			}
			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}

// moveStateV1 moves v1 state from the polaris_azure_exocompute resource in the
// rubrikinc/polaris provider to the rubrik_azure_exocompute resource.
func (r *azureExocomputeResource) moveStateV1(ctx context.Context) resource.StateMover {
	return resource.StateMover{
		SourceSchema: azureExocomputeSourceSchemaV1(ctx),
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "azureExocomputeResource.moveStateV1")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolaris+"_"+keyAzureExocompute {
				return
			}
			if req.SourceSchemaVersion != 1 {
				return
			}

			var state azureExocomputeModel
			res.Diagnostics.Append(req.SourceState.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}

// azureExocomputeSourceSchemaV1 returns the source schema used when moving v1
// state from the polaris_azure_exocompute resource. The schema is frozen at the
// attribute set of the latest polaris release. State written by earlier polaris
// releases lacks the fields added since, and the Framework sets those to null
// when decoding the source state.
func azureExocomputeSourceSchemaV1(ctx context.Context) *schema.Schema {
	return &schema.Schema{
		Attributes: map[string]schema.Attribute{
			keyID:                    schema.StringAttribute{Computed: true},
			keyCloudAccountID:        schema.StringAttribute{Optional: true},
			keyHostCloudAccountID:    schema.StringAttribute{Optional: true},
			keyPodOverlayNetworkCIDR: schema.StringAttribute{Optional: true},
			keyRegion:                schema.StringAttribute{Optional: true},
			keySubnet:                schema.StringAttribute{Optional: true},
			keySubscriptionID:        schema.StringAttribute{Optional: true},
			keyTriggerHealthCheck:    schema.BoolAttribute{Optional: true},
		},
		Blocks: map[string]schema.Block{
			keyOptionalConfig: schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						keyAllowlistAdditionalIPs:         schema.SetAttribute{ElementType: types.StringType, Optional: true},
						keyAllowlistRubrikIPs:             schema.BoolAttribute{Optional: true},
						keyClusterAccess:                  schema.StringAttribute{Optional: true},
						keyClusterTier:                    schema.StringAttribute{Optional: true},
						keyDiskEncryptionAtHost:           schema.BoolAttribute{Optional: true},
						keyDiskEncryptionSetID:            schema.StringAttribute{Optional: true, Computed: true},
						keyHealthCheckVMNamePrefix:        schema.StringAttribute{Optional: true, Computed: true},
						keyMaxNodeCount:                   schema.StringAttribute{Optional: true},
						keyPostgresPrivateDNSZoneID:       schema.StringAttribute{Optional: true, Computed: true},
						keyPostgresSubnetID:               schema.StringAttribute{Optional: true, Computed: true},
						keyPrivateExocomputeDNSZoneID:     schema.StringAttribute{Optional: true},
						keyResourceGroupPrefix:            schema.StringAttribute{Optional: true},
						keySnapshotPrivateAccessDNSZoneID: schema.StringAttribute{Optional: true},
						keySQLPrivateDNSZoneID:            schema.StringAttribute{Optional: true, Computed: true},
						keyUserDefinedRouting:             schema.BoolAttribute{Optional: true},
					},
				},
			},
			keyTimeouts: timeouts.Block(ctx, timeouts.Opts{Create: true, Update: true}),
		},
	}
}
