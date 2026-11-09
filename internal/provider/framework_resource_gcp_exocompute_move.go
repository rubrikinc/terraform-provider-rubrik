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

func (r *gcpExocomputeResource) MoveState(ctx context.Context) []resource.StateMover {
	tflog.Trace(ctx, "gcpExocomputeResource.MoveState")

	return []resource.StateMover{
		r.moveStateV0(),
	}
}

// moveStateV0 moves v0 state from the polaris_gcp_exocompute resource, in the
// rubrikinc/polaris provider or in this provider before the migration to the
// Plugin Framework, to the rubrik_gcp_exocompute resource. The SourceSchema is
// frozen at the SDKv2 v0 schema. Polaris provider versions before v1.11.0 have
// no host_project_id or secondary_range_name, which are then read as null.
func (r *gcpExocomputeResource) moveStateV0() resource.StateMover {
	return resource.StateMover{
		SourceSchema: &schema.Schema{
			Attributes: map[string]schema.Attribute{
				keyID:                 schema.StringAttribute{Computed: true},
				keyCloudAccountID:     schema.StringAttribute{Required: true},
				keyTriggerHealthCheck: schema.BoolAttribute{Optional: true},
			},
			Blocks: map[string]schema.Block{
				keyRegionalConfig: schema.SetNestedBlock{
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyHostProjectID:      schema.StringAttribute{Optional: true},
							keyRegion:             schema.StringAttribute{Optional: true},
							keySecondaryRangeName: schema.StringAttribute{Optional: true},
							keySubnetName:         schema.StringAttribute{Optional: true},
							keyVPCName:            schema.StringAttribute{Optional: true},
						},
					},
				},
			},
		},
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "gcpExocomputeResource.moveStateV0")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolaris+"_"+keyGCPExocompute {
				return
			}
			if req.SourceSchemaVersion != 0 {
				return
			}

			var state gcpExocomputeModel
			res.Diagnostics.Append(req.SourceState.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			var configs []gcpExocomputeRegionalConfigModel
			res.Diagnostics.Append(state.RegionalConfig.ElementsAs(ctx, &configs, false)...)
			if res.Diagnostics.HasError() {
				return
			}

			// The SDKv2 resource stored an unset string as an empty string,
			// Read stores it as null. A missing secondary range name is
			// stored as the default.
			for i := range configs {
				for _, v := range []*types.String{&configs[i].HostProjectID, &configs[i].Region,
					&configs[i].SubnetName, &configs[i].VPCName} {
					if v.ValueString() == "" {
						*v = types.StringNull()
					}
				}
				if configs[i].SecondaryRangeName.ValueString() == "" {
					configs[i].SecondaryRangeName = types.StringValue(gcpDefaultSecondaryRangeName)
				}
			}
			set, diags := types.SetValueFrom(ctx, gcpExocomputeRegionalConfigType, configs)
			res.Diagnostics.Append(diags...)
			if res.Diagnostics.HasError() {
				return
			}
			state.RegionalConfig = set

			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}
