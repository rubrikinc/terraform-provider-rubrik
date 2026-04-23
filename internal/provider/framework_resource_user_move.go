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
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/access"
	gqlaccess "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/access"
)

func (r *userResource) MoveState(ctx context.Context) []resource.StateMover {
	tflog.Trace(ctx, "userResource.MoveState")

	return []resource.StateMover{
		r.moveStateV0(),
		r.moveStateV1(),
	}
}

// moveStateV0 moves v0 state from the polaris_user resource in the
// rubrikinc/polaris provider to the rubrik_user resource. The v0 state used the
// user email address as the resource ID.
func (r *userResource) moveStateV0() resource.StateMover {
	return resource.StateMover{
		SourceSchema: &schema.Schema{
			Attributes: map[string]schema.Attribute{
				keyID: schema.StringAttribute{
					Computed:    true,
					Description: "User email address.",
				},
				keyEmail: schema.StringAttribute{
					Required:    true,
					Description: "User email address.",
				},
				keyIsAccountOwner: schema.BoolAttribute{
					Computed:    true,
					Description: "True if the user is the account owner.",
				},
				keyRoleIDs: schema.SetAttribute{
					ElementType: types.StringType,
					Required:    true,
					Description: "Roles assigned to the user (UUIDs).",
				},
				keyStatus: schema.StringAttribute{
					Computed:    true,
					Description: "User status.",
				},
			},
		},
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "userResource.moveStateV0")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolaris+"_"+keyUser {
				return
			}
			if req.SourceSchemaVersion != 0 {
				return
			}

			var prior userResourceModelV0
			res.Diagnostics.Append(req.SourceState.Get(ctx, &prior)...)
			if res.Diagnostics.HasError() {
				return
			}

			email := prior.Email.ValueString()
			if id := prior.ID.ValueString(); id != email {
				res.Diagnostics.AddError("State move failed",
					fmt.Sprintf("unexpected mismatch between user ID and email address: %s != %s", id, email))
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

			var state userResourceModel
			state.ID = types.StringValue(user.ID)
			state.Domain = types.StringValue(string(user.Domain))
			state.Email = prior.Email
			state.IsAccountOwner = prior.IsAccountOwner
			state.RoleIDs = prior.RoleIDs
			state.Status = prior.Status
			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}

// moveStateV1 moves v1 state from the polaris_user resource in the
// rubrikinc/polaris provider to the rubrik_user resource.
func (r *userResource) moveStateV1() resource.StateMover {
	return resource.StateMover{
		SourceSchema: &schema.Schema{
			Attributes: map[string]schema.Attribute{
				keyID: schema.StringAttribute{
					Computed:    true,
					Description: "User ID (UUID).",
				},
				keyDomain: schema.StringAttribute{
					Computed:    true,
					Description: "User domain.",
				},
				keyEmail: schema.StringAttribute{
					Required:    true,
					Description: "User email address.",
				},
				keyIsAccountOwner: schema.BoolAttribute{
					Computed:    true,
					Description: "True if the user is the account owner.",
				},
				keyRoleIDs: schema.SetAttribute{
					ElementType: types.StringType,
					Required:    true,
					Description: "Roles assigned to the user (UUIDs).",
				},
				keyStatus: schema.StringAttribute{
					Computed:    true,
					Description: "User status.",
				},
			},
		},
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "userResource.moveStateV1")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolaris+"_"+keyUser {
				return
			}
			if req.SourceSchemaVersion != 1 {
				return
			}

			var state userResourceModel
			res.Diagnostics.Append(req.SourceState.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}
