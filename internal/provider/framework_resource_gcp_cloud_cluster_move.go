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

func (r *gcpCloudClusterResource) MoveState(ctx context.Context) []resource.StateMover {
	tflog.Trace(ctx, "gcpCloudClusterResource.MoveState")

	return []resource.StateMover{
		r.moveStateV0(ctx),
	}
}

// moveStateV0 moves v0 state from the polaris_gcp_cloud_cluster resource in the
// rubrikinc/polaris provider to the rubrik_gcp_cloud_cluster resource. The
// SourceSchema is an inline literal frozen at the polaris V0 schema so it
// remains correct regardless of future changes to the rubrik resource schema.
func (r *gcpCloudClusterResource) moveStateV0(ctx context.Context) resource.StateMover {
	return resource.StateMover{
		SourceSchema: &schema.Schema{
			Attributes: map[string]schema.Attribute{
				keyID:             schema.StringAttribute{Computed: true},
				keyCloudAccountID: schema.StringAttribute{Required: true},
				keyRegion:         schema.StringAttribute{Required: true},
				keyZone:           schema.StringAttribute{Optional: true, Computed: true},
				keyAzResilient:    schema.BoolAttribute{Optional: true, Computed: true},
			},
			Blocks: map[string]schema.Block{
				keyClusterConfig: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyClusterName:                 schema.StringAttribute{Required: true},
							keyAdminEmail:                  schema.StringAttribute{Optional: true},
							keyAdminPassword:               schema.StringAttribute{Optional: true},
							keyNumNodes:                    schema.Int64Attribute{Required: true},
							keyDNSNameServers:              schema.SetAttribute{ElementType: types.StringType, Required: true},
							keyDNSSearchDomains:            schema.SetAttribute{ElementType: types.StringType, Optional: true, Computed: true},
							keyNTPServers:                  schema.SetAttribute{ElementType: types.StringType, Required: true},
							keyBucketName:                  schema.StringAttribute{Required: true},
							keyKeepClusterOnFailure:        schema.BoolAttribute{Required: true},
							keyForceClusterDeleteOnDestroy: schema.BoolAttribute{Optional: true, Computed: true},
							keyTimezone:                    schema.StringAttribute{Optional: true, Computed: true},
							keyLocation:                    schema.StringAttribute{Optional: true, Computed: true},
						},
					},
				},
				keyVMConfig: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyCDMVersion:       schema.StringAttribute{Required: true},
							keyCDMProduct:       schema.StringAttribute{Computed: true},
							keyInstanceType:     schema.StringAttribute{Required: true},
							keyNetwork:          schema.StringAttribute{Required: true},
							keySubnet:           schema.StringAttribute{Optional: true},
							keyHostProject:      schema.StringAttribute{Optional: true},
							keyServiceAccounts:  schema.SetAttribute{ElementType: types.StringType, Required: true},
							keyDeleteProtection: schema.BoolAttribute{Optional: true, Computed: true},
						},
						Blocks: map[string]schema.Block{
							keySubnetAzConfigs: schema.ListNestedBlock{
								NestedObject: schema.NestedBlockObject{
									Attributes: map[string]schema.Attribute{
										keyAvailabilityZone: schema.StringAttribute{Required: true},
										keySubnet:           schema.StringAttribute{Required: true},
									},
								},
							},
						},
					},
				},
				keyTimeouts: timeouts.Block(ctx, timeouts.Opts{Create: true}),
			},
		},
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "gcpCloudClusterResource.moveStateV0")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolaris+"_"+keyGcpCloudCluster {
				return
			}
			if req.SourceSchemaVersion != 0 {
				return
			}

			var state gcpCloudClusterModel
			res.Diagnostics.Append(req.SourceState.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}
