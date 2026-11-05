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
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func (r *gcpProjectResource) MoveState(ctx context.Context) []resource.StateMover {
	tflog.Trace(ctx, "gcpProjectResource.MoveState")

	return []resource.StateMover{
		r.moveStateV0(),
		r.moveStateV1(),
		r.moveStateV2(),
	}
}

// gcpProjectMoveGuard reports whether the move request is for the
// polaris_gcp_project resource of the given schema version.
func gcpProjectMoveGuard(req resource.MoveStateRequest, version int64) bool {
	return (strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") ||
		strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik")) &&
		req.SourceTypeName == keyPolaris+"_"+keyGCPProject &&
		req.SourceSchemaVersion == version
}

// moveStateV0 moves v0 state, written by polaris provider versions before
// v0.2.0, to the rubrik_gcp_project resource. The v0 ID is the RSC cloud
// account ID and the GCP project ID separated by a colon.
//
// Unlike the state upgrader, the mover can't look the project up in RSC,
// Terraform doesn't configure the resource before moving state. The GCP
// project ID part of the ID is checked against the project in the state
// instead, and Read fills in the Cloud Native Protection block and the
// features.
func (r *gcpProjectResource) moveStateV0() resource.StateMover {
	return resource.StateMover{
		SourceSchema: gcpProjectSchemaV0,
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "gcpProjectResource.moveStateV0")

			if !gcpProjectMoveGuard(req, 0) {
				return
			}

			var prior gcpProjectResourceModelV0
			res.Diagnostics.Append(req.SourceState.Get(ctx, &prior)...)
			if res.Diagnostics.HasError() {
				return
			}

			parts := strings.Split(prior.ID.ValueString(), ":")
			if len(parts) != 2 {
				res.Diagnostics.AddError("State move failed", "invalid id format for v0 resource state")
				return
			}
			if project := prior.Project.ValueString(); project != "" && project != parts[1] {
				res.Diagnostics.AddError("State move failed",
					fmt.Sprintf("v0 id refers to GCP project %q but the state holds GCP project %q", parts[1],
						project))
				return
			}
			prior.ID = types.StringValue(parts[0])

			state, diags := gcpProjectStateFromV1(ctx, prior, nil)
			res.Diagnostics.Append(diags...)
			if res.Diagnostics.HasError() {
				return
			}
			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}

// moveStateV1 moves v1 state, written by polaris provider versions before
// v0.3.0, to the rubrik_gcp_project resource. As for v0, the mover can't look
// up the Cloud Native Protection status in RSC, Read fills it in.
func (r *gcpProjectResource) moveStateV1() resource.StateMover {
	return resource.StateMover{
		SourceSchema: gcpProjectSchemaV0,
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "gcpProjectResource.moveStateV1")

			if !gcpProjectMoveGuard(req, 1) {
				return
			}

			var prior gcpProjectResourceModelV0
			res.Diagnostics.Append(req.SourceState.Get(ctx, &prior)...)
			if res.Diagnostics.HasError() {
				return
			}

			state, diags := gcpProjectStateFromV1(ctx, prior, nil)
			res.Diagnostics.Append(diags...)
			if res.Diagnostics.HasError() {
				return
			}
			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}

// moveStateV2 moves v2 state to the rubrik_gcp_project resource. The SDKv2
// resource stored an unset string as an empty string, which is stored as null,
// matching a configuration leaving it out.
func (r *gcpProjectResource) moveStateV2() resource.StateMover {
	return resource.StateMover{
		SourceSchema: &schema.Schema{
			Attributes: map[string]schema.Attribute{
				keyID:                       schema.StringAttribute{Computed: true},
				keyCredentials:              schema.StringAttribute{Optional: true},
				keyDeleteSnapshotsOnDestroy: schema.BoolAttribute{Optional: true},
				keyOrganizationName:         schema.StringAttribute{Optional: true},
				keyPermissionsHash:          schema.StringAttribute{Optional: true},
				keyProject:                  schema.StringAttribute{Required: true},
				keyProjectName:              schema.StringAttribute{Required: true},
				keyProjectNumber:            schema.StringAttribute{Required: true},
			},
			Blocks: map[string]schema.Block{
				keyCloudNativeProtection: schema.ListNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyStatus: schema.StringAttribute{Optional: true, Computed: true},
						},
					},
				},
				keyFeature: schema.SetNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyName:             schema.StringAttribute{Required: true},
							keyPermissionGroups: schema.SetAttribute{ElementType: types.StringType, Required: true},
							keyPermissions:      schema.StringAttribute{Optional: true},
							keyStatus:           schema.StringAttribute{Computed: true},
						},
					},
				},
			},
		},
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "gcpProjectResource.moveStateV2")

			if !gcpProjectMoveGuard(req, 2) {
				return
			}

			var state gcpProjectResourceModel
			res.Diagnostics.Append(req.SourceState.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			for _, v := range []*types.String{&state.Credentials, &state.OrganizationName, &state.PermissionsHash} {
				if v.ValueString() == "" {
					*v = types.StringNull()
				}
			}
			if state.DeleteSnapshotsOnDestroy.IsNull() {
				state.DeleteSnapshotsOnDestroy = types.BoolValue(false)
			}

			features := []gcpProjectFeatureModel{}
			res.Diagnostics.Append(state.Feature.ElementsAs(ctx, &features, false)...)
			if res.Diagnostics.HasError() {
				return
			}
			for i := range features {
				if features[i].Permissions.ValueString() == "" {
					features[i].Permissions = types.StringNull()
				}
			}
			set, diags := types.SetValueFrom(ctx, gcpProjectFeatureType, features)
			res.Diagnostics.Append(diags...)
			if res.Diagnostics.HasError() {
				return
			}
			state.Feature = set

			// The SDKv2 resource populated both blocks, only the block in use
			// is kept. With features, the Cloud Native Protection block was
			// read from the features and is dropped.
			if len(state.Feature.Elements()) > 0 || state.CloudNativeProtection.IsNull() {
				state.CloudNativeProtection = types.ListValueMust(gcpProjectCloudNativeProtectionType, nil)
			}

			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}
