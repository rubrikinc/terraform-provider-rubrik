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

func (r *awsAccountManagedResource) MoveState(ctx context.Context) []resource.StateMover {
	tflog.Trace(ctx, "awsAccountManagedResource.MoveState")

	return []resource.StateMover{
		r.moveStateV0(),
	}
}

// moveStateV0 moves v0 state from the polaris_aws_account_managed resource in
// the rubrikinc/polaris provider to the rubrik_aws_account_managed resource.
func (r *awsAccountManagedResource) moveStateV0() resource.StateMover {
	return resource.StateMover{
		SourceSchema: &schema.Schema{
			Attributes: map[string]schema.Attribute{
				keyID: schema.StringAttribute{
					Computed: true,
				},
				keyNativeID: schema.StringAttribute{
					Required: true,
				},
				keyName: schema.StringAttribute{
					Optional: true,
					Computed: true,
				},
				keyCloud: schema.StringAttribute{
					Optional: true,
					Computed: true,
				},
				keyFeatures: schema.SetAttribute{
					ElementType: types.StringType,
					Optional:    true,
					Computed:    true,
				},
				keyRegions: schema.SetAttribute{
					ElementType: types.StringType,
					Optional:    true,
					Computed:    true,
				},
				keyCloudFormationURL: schema.StringAttribute{
					Computed: true,
				},
				keyTemplateURL: schema.StringAttribute{
					Computed: true,
				},
				keyStackName: schema.StringAttribute{
					Computed: true,
				},
				keyPermissionsVersion: schema.StringAttribute{
					Computed: true,
				},
			},
		},
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "awsAccountManagedResource.moveStateV0")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolaris+"_"+keyAWSAccountManaged {
				return
			}
			if req.SourceSchemaVersion != 0 {
				return
			}

			var state awsAccountManagedModel
			res.Diagnostics.Append(req.SourceState.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}
