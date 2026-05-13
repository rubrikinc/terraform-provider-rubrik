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

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func (r *awsCnpAccountResource) MoveState(ctx context.Context) []resource.StateMover {
	tflog.Trace(ctx, "awsCnpAccountResource.MoveState")

	return []resource.StateMover{
		r.moveStateV0(),
	}
}

// moveStateV0 moves v0 state from the polaris_aws_cnp_account resource in the
// rubrikinc/polaris provider to the rubrik_aws_cnp_account resource.
func (r *awsCnpAccountResource) moveStateV0() resource.StateMover {
	return resource.StateMover{
		SourceSchema: &schema.Schema{
			Attributes: map[string]schema.Attribute{
				keyID: schema.StringAttribute{
					Computed: true,
				},
				keyCloud: schema.StringAttribute{
					Optional: true,
					Computed: true,
				},
				keyDeleteSnapshotsOnDestroy: schema.BoolAttribute{
					Optional: true,
					Computed: true,
				},
				keyExternalID: schema.StringAttribute{
					Optional: true,
				},
				keyName: schema.StringAttribute{
					Optional: true,
					Computed: true,
				},
				keyNativeID: schema.StringAttribute{
					Required: true,
				},
				keyRoleChainingAccountID: schema.StringAttribute{
					Optional: true,
				},
				keyRegions: schema.SetAttribute{
					ElementType: types.StringType,
					Required:    true,
				},
				keyTrustPolicies: schema.SetNestedAttribute{
					Computed: true,
					NestedObject: schema.NestedAttributeObject{
						Attributes: map[string]schema.Attribute{
							keyRoleKey: schema.StringAttribute{
								Computed: true,
							},
							keyPolicy: schema.StringAttribute{
								Computed: true,
							},
						},
					},
				},
			},
			Blocks: map[string]schema.Block{
				keyFeature: schema.SetNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyName: schema.StringAttribute{
								Required: true,
							},
							keyPermissionGroups: schema.SetAttribute{
								ElementType: types.StringType,
								Required:    true,
							},
						},
					},
				},
			},
		},
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "awsCnpAccountResource.moveStateV0")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolaris+"_"+keyAWSCNPAccount {
				return
			}
			if req.SourceSchemaVersion != 0 {
				return
			}

			var state awsCnpAccountModel
			res.Diagnostics.Append(req.SourceState.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}
