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
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func (r *awsAccountResource) MoveState(ctx context.Context) []resource.StateMover {
	tflog.Trace(ctx, "awsAccountResource.MoveState")

	return []resource.StateMover{
		r.moveStateV0(),
		r.moveStateV1(),
		r.moveStateV2(),
		r.moveStateLatest(ctx),
	}
}

// awsAccountMoveGuard reports whether the move request is for state of the
// given schema version, from the polaris_aws_account resource of the
// rubrikinc/rubrik or rubrikinc/polaris provider.
func awsAccountMoveGuard(req resource.MoveStateRequest, version int64) bool {
	if req.SourceTypeName != keyPolaris+"_"+keyAWSAccount || req.SourceSchemaVersion != version {
		return false
	}

	provider := req.SourceProviderAddress
	return strings.HasSuffix(provider, "rubrikinc/rubrik") || strings.HasSuffix(provider, "rubrikinc/polaris")
}

// moveStateV0 moves v0 state from the polaris_aws_account resource. Unlike
// the state upgrader, the mover can't check the ID with RSC, the resource isn't
// configured before moving state.
func (r *awsAccountResource) moveStateV0() resource.StateMover {
	return resource.StateMover{
		SourceSchema: awsAccountSchemaV0,
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "awsAccountResource.moveStateV0")

			if !awsAccountMoveGuard(req, 0) {
				return
			}

			var prior awsAccountResourceModelV0
			res.Diagnostics.Append(req.SourceState.Get(ctx, &prior)...)
			if res.Diagnostics.HasError() {
				return
			}

			state, d := upgradeAccountStateV0(ctx, prior)
			res.Diagnostics.Append(d...)
			if res.Diagnostics.HasError() {
				return
			}

			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}

// moveStateV1 moves v1 state from the polaris_aws_account resource.
func (r *awsAccountResource) moveStateV1() resource.StateMover {
	return resource.StateMover{
		SourceSchema: awsAccountSchemaV1,
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "awsAccountResource.moveStateV1")

			if !awsAccountMoveGuard(req, 1) {
				return
			}

			var prior awsAccountResourceModelV1
			res.Diagnostics.Append(req.SourceState.Get(ctx, &prior)...)
			if res.Diagnostics.HasError() {
				return
			}

			state, d := upgradeAccountStateV1(ctx, prior)
			res.Diagnostics.Append(d...)
			if res.Diagnostics.HasError() {
				return
			}

			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}

// moveStateV2 moves v2 state, written by the SDKv2 polaris_aws_account
// resource.
func (r *awsAccountResource) moveStateV2() resource.StateMover {
	return resource.StateMover{
		SourceSchema: awsAccountSchemaV2,
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "awsAccountResource.moveStateV2")

			if !awsAccountMoveGuard(req, 2) {
				return
			}

			var prior awsAccountResourceModelV2
			res.Diagnostics.Append(req.SourceState.Get(ctx, &prior)...)
			if res.Diagnostics.HasError() {
				return
			}

			state, d := upgradeAccountStateV2(ctx, prior)
			res.Diagnostics.Append(d...)
			if res.Diagnostics.HasError() {
				return
			}

			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}

// moveStateLatest moves state of the current schema version, written by the
// polaris_aws_account alias of this provider, as it is. When the schema version
// is bumped, the previous version needs a mover of its own, with a copy of its
// schema, as for v0 to v2.
func (r *awsAccountResource) moveStateLatest(ctx context.Context) resource.StateMover {
	var schemaRes resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaRes)

	return resource.StateMover{
		SourceSchema: &schemaRes.Schema,
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "awsAccountResource.moveStateLatest")

			if !awsAccountMoveGuard(req, schemaRes.Schema.Version) {
				return
			}

			var state awsAccountResourceModel
			res.Diagnostics.Append(req.SourceState.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}
