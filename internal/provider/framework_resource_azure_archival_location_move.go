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
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func (r *azureArchivalLocationResource) MoveState(ctx context.Context) []resource.StateMover {
	tflog.Trace(ctx, "azureArchivalLocationResource.MoveState")

	return []resource.StateMover{
		r.moveStateV0(ctx),
		r.moveStateLatest(ctx),
	}
}

// azureArchivalLocationMoveGuard reports whether the move request is for state
// of the given schema version, from the polaris_azure_archival_location
// resource of the rubrikinc/rubrik or rubrikinc/polaris provider.
func azureArchivalLocationMoveGuard(req resource.MoveStateRequest, version int64) bool {
	if req.SourceTypeName != keyPolaris+"_"+keyAzureArchivalLocation || req.SourceSchemaVersion != version {
		return false
	}

	provider := req.SourceProviderAddress
	return strings.HasSuffix(provider, "rubrikinc/rubrik") || strings.HasSuffix(provider, "rubrikinc/polaris")
}

// moveStateV0 moves v0 state from the polaris_azure_archival_location resource
// in the rubrikinc/polaris provider, or from the deprecated alias in this
// provider, to the rubrik_azure_archival_location resource. It does the same
// conversion as the v0 state upgrader.
func (r *azureArchivalLocationResource) moveStateV0(ctx context.Context) resource.StateMover {
	return resource.StateMover{
		SourceSchema: azureArchivalLocationSchemaV0(ctx),
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "azureArchivalLocationResource.moveStateV0")

			if !azureArchivalLocationMoveGuard(req, 0) {
				return
			}

			var state azureArchivalLocationResourceModel
			res.Diagnostics.Append(req.SourceState.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			if state.StorageAccountRegion.ValueString() == "" {
				state.StorageAccountRegion = types.StringNull()
			}
			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}

// moveStateLatest moves state of the current schema version, written by the
// polaris_azure_archival_location alias of this provider, as it is. When the
// schema version is bumped, the previous version needs a mover of its own, with
// a copy of its schema, as for v0.
func (r *azureArchivalLocationResource) moveStateLatest(ctx context.Context) resource.StateMover {
	var schemaRes resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaRes)

	return resource.StateMover{
		SourceSchema: &schemaRes.Schema,
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "azureArchivalLocationResource.moveStateLatest")

			if !azureArchivalLocationMoveGuard(req, schemaRes.Schema.Version) {
				return
			}

			var state azureArchivalLocationResourceModel
			res.Diagnostics.Append(req.SourceState.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}
