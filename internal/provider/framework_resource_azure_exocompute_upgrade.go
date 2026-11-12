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

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// azureExocomputeModelV0 is the version 0 state of the resource.
type azureExocomputeModelV0 struct {
	ID             types.String `tfsdk:"id"`
	PolarisManaged types.Bool   `tfsdk:"polaris_managed"`
	Region         types.String `tfsdk:"region"`
	Subnet         types.String `tfsdk:"subnet"`
	SubscriptionID types.String `tfsdk:"subscription_id"`
}

// azureExocomputeNullTimeouts is a null timeouts block value, with the
// attribute types of the timeouts block of the resource.
var azureExocomputeNullTimeouts = timeouts.Value{Object: types.ObjectNull(map[string]attr.Type{
	"create": types.StringType,
	"update": types.StringType,
})}

func (r *azureExocomputeResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	tflog.Trace(ctx, "azureExocomputeResource.UpgradeState")

	return map[int64]resource.StateUpgrader{
		0: r.upgradeStateV0(),
	}
}

// upgradeStateV0 upgrades v0 state to v1. The polaris_managed field is
// removed, since Exocompute on Azure only supports RSC managed configurations.
// The fields added in v1 are set to their unset values, the next read fills in
// the values stored in RSC.
func (r *azureExocomputeResource) upgradeStateV0() resource.StateUpgrader {
	return resource.StateUpgrader{
		PriorSchema: &schema.Schema{
			Attributes: map[string]schema.Attribute{
				keyID:             schema.StringAttribute{Computed: true},
				keyPolarisManaged: schema.BoolAttribute{Optional: true},
				keyRegion:         schema.StringAttribute{Required: true},
				keySubnet:         schema.StringAttribute{Required: true},
				keySubscriptionID: schema.StringAttribute{Required: true},
			},
		},
		StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, res *resource.UpgradeStateResponse) {
			tflog.Trace(ctx, "azureExocomputeResource.upgradeStateV0")

			var prior azureExocomputeModelV0
			res.Diagnostics.Append(req.State.Get(ctx, &prior)...)
			if res.Diagnostics.HasError() {
				return
			}

			state := azureExocomputeModel{
				ID:                    prior.ID,
				CloudAccountID:        types.StringNull(),
				HostCloudAccountID:    types.StringNull(),
				OptionalConfig:        types.ListValueMust(types.ObjectType{AttrTypes: azureExocomputeOptionalConfigAttrTypes}, []attr.Value{}),
				PodOverlayNetworkCIDR: types.StringValue(""),
				Region:                prior.Region,
				Subnet:                prior.Subnet,
				SubscriptionID:        prior.SubscriptionID,
				TriggerHealthCheck:    types.BoolNull(),
				Timeouts:              azureExocomputeNullTimeouts,
			}
			res.Diagnostics.Append(res.State.Set(ctx, &state)...)
		},
	}
}
