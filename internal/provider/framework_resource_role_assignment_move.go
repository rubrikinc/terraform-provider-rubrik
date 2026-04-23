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
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/access"
	gqlaccess "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/access"
)

func (r *roleAssignmentResource) MoveState(ctx context.Context) []resource.StateMover {
	tflog.Trace(ctx, "roleAssignmentResource.MoveState")

	return []resource.StateMover{
		r.moveStateV0(),
		r.moveStateV1(),
	}
}

// moveStateV0 moves v0 state from the polaris_role_assignment resource in the
// rubrikinc/polaris provider to the rubrik_role_assignment resource. The v0
// state used a SHA-256 hash of the user email and role ID as the resource ID.
func (r *roleAssignmentResource) moveStateV0() resource.StateMover {
	return resource.StateMover{
		SourceSchema: &schema.Schema{
			Attributes: map[string]schema.Attribute{
				keyID: schema.StringAttribute{
					Computed:    true,
					Description: "SHA-256 hash of the user email and the role ID.",
				},
				keyRoleID: schema.StringAttribute{
					Required:    true,
					Description: "Role ID (UUID).",
				},
				keyUserEmail: schema.StringAttribute{
					Required:    true,
					Description: "User email address.",
				},
			},
		},
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "roleAssignmentResource.moveStateV0")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolaris+"_"+keyRoleAssignment {
				return
			}
			if req.SourceSchemaVersion != 0 {
				return
			}

			var prior roleAssignmentModelV0
			res.Diagnostics.Append(req.SourceState.Get(ctx, &prior)...)
			if res.Diagnostics.HasError() {
				return
			}

			email := prior.UserEmail.ValueString()
			roleID := prior.RoleID.ValueString()
			expectedID := fmt.Sprintf("%x", sha256.Sum256([]byte(email+roleID)))
			if prior.ID.ValueString() != expectedID {
				res.Diagnostics.AddError("State move failed",
					fmt.Sprintf("unexpected resource id: %s", prior.ID.ValueString()))
				return
			}

			polarisClient, err := r.client.polaris()
			if err != nil {
				res.Diagnostics.AddError("RSC client error", err.Error())
				return
			}

			user, err := access.Wrap(polarisClient).UserByEmail(ctx, email, gqlaccess.DomainLocal)
			if err != nil {
				res.Diagnostics.AddError("Failed to look up user by email", err.Error())
				return
			}

			var state roleAssignmentModel
			state.ID = types.StringValue(user.ID)
			state.RoleID = prior.RoleID
			state.RoleIDs = types.SetNull(types.StringType)
			state.SSOGroupID = types.StringNull()
			state.UserEmail = prior.UserEmail
			state.UserID = types.StringNull()
			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}

// moveStateV1 moves v1 state from the polaris_role_assignment resource in the
// rubrikinc/polaris provider to the rubrik_role_assignment resource.
func (r *roleAssignmentResource) moveStateV1() resource.StateMover {
	return resource.StateMover{
		SourceSchema: &schema.Schema{
			Attributes: map[string]schema.Attribute{
				keyID: schema.StringAttribute{
					Computed:    true,
					Description: "User or SSO group ID.",
				},
				keyRoleID: schema.StringAttribute{
					Optional:    true,
					Description: "Role ID (UUID).",
				},
				keyRoleIDs: schema.SetAttribute{
					ElementType: types.StringType,
					Optional:    true,
					Description: "Role IDs (UUID).",
				},
				keySSOGroupID: schema.StringAttribute{
					Optional:    true,
					Description: "SSO group ID.",
				},
				keyUserEmail: schema.StringAttribute{
					Optional:    true,
					Description: "User email address.",
				},
				keyUserID: schema.StringAttribute{
					Optional:    true,
					Description: "User ID.",
				},
			},
		},
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "roleAssignmentResource.moveStateV1")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolaris+"_"+keyRoleAssignment {
				return
			}
			if req.SourceSchemaVersion != 1 {
				return
			}

			var state roleAssignmentModel
			res.Diagnostics.Append(req.SourceState.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}
