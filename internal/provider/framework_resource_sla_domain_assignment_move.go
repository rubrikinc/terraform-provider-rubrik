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

func (r *slaDomainAssignmentResource) MoveState(ctx context.Context) []resource.StateMover {
	tflog.Trace(ctx, "slaDomainAssignmentResource.MoveState")

	return []resource.StateMover{
		r.moveStateV0(),
		r.moveStateV1(),
	}
}

// moveStateV0 moves version 0 state from the polaris_sla_domain_assignment
// resource in the rubrikinc/polaris provider to the
// rubrik_sla_domain_assignment resource. The state is given the values the
// state upgrader gives it.
func (r *slaDomainAssignmentResource) moveStateV0() resource.StateMover {
	return resource.StateMover{
		SourceSchema: slaDomainAssignmentSchemaV0(),
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "slaDomainAssignmentResource.moveStateV0")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolaris+"_"+keySLADomainAssignment {
				return
			}
			if req.SourceSchemaVersion != 0 {
				return
			}

			var prior slaDomainAssignmentModelV0
			res.Diagnostics.Append(req.SourceState.Get(ctx, &prior)...)
			if res.Diagnostics.HasError() {
				return
			}

			state := upgradeSLADomainAssignmentStateV0(prior)
			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}

// moveStateV1 moves version 1 state from the polaris_sla_domain_assignment
// resource in the rubrikinc/polaris provider to the
// rubrik_sla_domain_assignment resource.
func (r *slaDomainAssignmentResource) moveStateV1() resource.StateMover {
	return resource.StateMover{
		SourceSchema: &schema.Schema{
			Attributes: map[string]schema.Attribute{
				keyID: schema.StringAttribute{
					Computed: true,
				},
				keyAssignmentType: schema.StringAttribute{
					Optional: true,
				},
				keyObjectIDs: schema.SetAttribute{
					ElementType: types.StringType,
					Required:    true,
				},
				keySLADomainID: schema.StringAttribute{
					Optional: true,
				},
				keyApplyChangesToExistingSnapshots: schema.BoolAttribute{
					Optional: true,
				},
				keyApplyChangesToNonPolicySnapshots: schema.BoolAttribute{
					Optional: true,
				},
				keyExistingSnapshotRetention: schema.StringAttribute{
					Optional: true,
				},
				keyWorkload: schema.StringAttribute{
					Optional: true,
				},
			},
		},
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "slaDomainAssignmentResource.moveStateV1")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolaris+"_"+keySLADomainAssignment {
				return
			}
			if req.SourceSchemaVersion != 1 {
				return
			}

			var state slaDomainAssignmentModel
			res.Diagnostics.Append(req.SourceState.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}
