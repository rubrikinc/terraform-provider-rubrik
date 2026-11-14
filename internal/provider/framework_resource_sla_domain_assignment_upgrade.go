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

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	gqlsla "github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/sla"
)

// slaDomainAssignmentModelV0 is the version 0 state. It only had the SLA domain
// ID and the object IDs.
type slaDomainAssignmentModelV0 struct {
	ID          types.String `tfsdk:"id"`
	ObjectIDs   types.Set    `tfsdk:"object_ids"`
	SLADomainID types.String `tfsdk:"sla_domain_id"`
}

// slaDomainAssignmentSchemaV0 returns the schema of the version 0 state of the
// polaris_sla_domain_assignment resource. Only the types and whether an
// attribute is required, optional or computed matter, since the schema is only
// used to decode the prior state.
func slaDomainAssignmentSchemaV0() *schema.Schema {
	return &schema.Schema{
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed: true,
			},
			keyObjectIDs: schema.SetAttribute{
				ElementType: types.StringType,
				Required:    true,
			},
			keySLADomainID: schema.StringAttribute{
				Required: true,
			},
		},
	}
}

// upgradeSLADomainAssignmentStateV0 upgrades the version 0 state to the current
// state. Version 0 state only supported protectWithSlaId, so the new
// attributes get the values the SDKv2 state upgrader gave them.
func upgradeSLADomainAssignmentStateV0(prior slaDomainAssignmentModelV0) slaDomainAssignmentModel {
	return slaDomainAssignmentModel{
		ID:                               prior.ID,
		AssignmentType:                   types.StringValue(string(gqlsla.ProtectWithSLA)),
		ObjectIDs:                        prior.ObjectIDs,
		SLADomainID:                      prior.SLADomainID,
		ApplyChangesToExistingSnapshots:  types.BoolValue(true),
		ApplyChangesToNonPolicySnapshots: types.BoolValue(false),
		ExistingSnapshotRetention:        types.StringNull(),
		Workload:                         types.StringNull(),
	}
}

func (r *slaDomainAssignmentResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	tflog.Trace(ctx, "slaDomainAssignmentResource.UpgradeState")

	return map[int64]resource.StateUpgrader{
		0: r.upgradeStateV0(),
	}
}

// upgradeStateV0 upgrades version 0 state to version 1, which added the
// assignment_type, apply_changes_to_existing_snapshots and
// apply_changes_to_non_policy_snapshots attributes.
func (r *slaDomainAssignmentResource) upgradeStateV0() resource.StateUpgrader {
	return resource.StateUpgrader{
		PriorSchema: slaDomainAssignmentSchemaV0(),
		StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, res *resource.UpgradeStateResponse) {
			tflog.Trace(ctx, "slaDomainAssignmentResource.upgradeStateV0")

			var prior slaDomainAssignmentModelV0
			res.Diagnostics.Append(req.State.Get(ctx, &prior)...)
			if res.Diagnostics.HasError() {
				return
			}

			state := upgradeSLADomainAssignmentStateV0(prior)
			res.Diagnostics.Append(res.State.Set(ctx, &state)...)
		},
	}
}
