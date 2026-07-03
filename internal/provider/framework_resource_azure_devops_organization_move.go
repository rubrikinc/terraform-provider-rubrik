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

func (r *azureDevOpsOrganizationResource) MoveState(ctx context.Context) []resource.StateMover {
	tflog.Trace(ctx, "azureDevOpsOrganizationResource.MoveState")

	return []resource.StateMover{
		r.moveStateV0(),
	}
}

// moveStateV0 moves v0 state from the polaris_azure_devops_organization resource
// in the rubrikinc/polaris provider to the rubrik_azure_devops_organization
// resource. It also handles the deprecated polaris_azure_devops_organization
// alias in the rubrikinc/rubrik provider.
func (r *azureDevOpsOrganizationResource) moveStateV0() resource.StateMover {
	return resource.StateMover{
		SourceSchema: &schema.Schema{
			Attributes: map[string]schema.Attribute{
				keyID: schema.StringAttribute{
					Computed: true,
				},
				keyNativeID: schema.StringAttribute{
					Required: true,
				},
				keyTenantDomain: schema.StringAttribute{
					Required: true,
				},
				keyCloud: schema.StringAttribute{
					Optional: true,
					Computed: true,
				},
				keyExocomputeHostType: schema.StringAttribute{
					Required: true,
				},
				keyStorageType: schema.StringAttribute{
					Required: true,
				},
				keyArchivalLocationID: schema.StringAttribute{
					Optional: true,
				},
				keyExocomputeHostCloudAccountID: schema.StringAttribute{
					Optional: true,
				},
				keyExocomputeRegion: schema.StringAttribute{
					Optional: true,
				},
				keyDeleteSnapshotsOnDestroy: schema.BoolAttribute{
					Optional: true,
					Computed: true,
				},
				keyConnectionStatus: schema.StringAttribute{
					Computed: true,
				},
				keyProjectCount: schema.Int64Attribute{
					Computed: true,
				},
				keyRepoCount: schema.Int64Attribute{
					Computed: true,
				},
				keyLastRefreshTime: schema.StringAttribute{
					Computed: true,
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
								Optional:    true,
								ElementType: types.StringType,
							},
						},
					},
				},
			},
		},
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "azureDevOpsOrganizationResource.moveStateV0")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolaris+"_"+keyAzureDevOpsOrganization {
				return
			}
			if req.SourceSchemaVersion != 0 {
				return
			}

			var state azureDevOpsOrganizationModel
			res.Diagnostics.Append(req.SourceState.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}
