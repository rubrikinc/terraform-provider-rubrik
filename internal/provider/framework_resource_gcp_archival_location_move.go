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

func (r *gcpArchivalLocationResource) MoveState(ctx context.Context) []resource.StateMover {
	tflog.Trace(ctx, "gcpArchivalLocationResource.MoveState")

	return []resource.StateMover{
		r.moveStateV0(ctx),
	}
}

// moveStateV0 moves v0 state from the polaris_gcp_archival_location resource,
// in the rubrikinc/polaris provider or in this provider before the migration
// to the Plugin Framework, to the rubrik_gcp_archival_location resource. The
// SourceSchema is frozen at the SDKv2 v0 schema. Attributes the source state
// lacks, kms_key and timeouts in the polaris provider, are read as null.
func (r *gcpArchivalLocationResource) moveStateV0(ctx context.Context) resource.StateMover {
	return resource.StateMover{
		SourceSchema: &schema.Schema{
			Attributes: map[string]schema.Attribute{
				keyID:               schema.StringAttribute{Computed: true},
				keyCloudAccountID:   schema.StringAttribute{Required: true},
				keyBucketLabels:     schema.MapAttribute{ElementType: types.StringType, Optional: true},
				keyBucketPrefix:     schema.StringAttribute{Required: true},
				keyConnectionStatus: schema.StringAttribute{Computed: true},
				keyLocationTemplate: schema.StringAttribute{Computed: true},
				keyName:             schema.StringAttribute{Required: true},
				keyRegion:           schema.StringAttribute{Optional: true},
				keyStorageClass:     schema.StringAttribute{Optional: true},
			},
			Blocks: map[string]schema.Block{
				keyCustomerManagedKey: schema.SetNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyName:     schema.StringAttribute{Required: true},
							keyRegion:   schema.StringAttribute{Required: true},
							keyRingName: schema.StringAttribute{Required: true},
						},
					},
				},
				keyKMSKey: schema.ListNestedBlock{
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
			tflog.Trace(ctx, "gcpArchivalLocationResource.moveStateV0")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolaris+"_"+keyGCPArchivalLocation {
				return
			}
			if req.SourceSchemaVersion != 0 {
				return
			}

			var state gcpArchivalLocationResourceModel
			res.Diagnostics.Append(req.SourceState.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			// The SDKv2 resource stored no labels as an empty map and no region
			// as an empty string, Read stores both as null.
			if len(state.BucketLabels.Elements()) == 0 {
				state.BucketLabels = types.MapNull(types.StringType)
			}
			if state.Region.ValueString() == "" {
				state.Region = types.StringNull()
			}
			// Terraform holds an absent block as an empty collection. The
			// polaris provider has no kms_key block, which is then read as null.
			if state.CustomerManagedKey.IsNull() {
				state.CustomerManagedKey = types.SetValueMust(gcpArchivalLocationCustomerManagedKeyType, nil)
			}
			if state.KMSKey.IsNull() {
				state.KMSKey = types.ListValueMust(gcpArchivalLocationKMSKeyType, nil)
			}

			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}
