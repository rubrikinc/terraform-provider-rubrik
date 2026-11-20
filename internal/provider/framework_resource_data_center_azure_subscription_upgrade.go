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
)

func (r *dataCenterAzureSubscriptionResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	tflog.Trace(ctx, "dataCenterAzureSubscriptionResource.UpgradeState")

	return map[int64]resource.StateUpgrader{
		0: r.upgradeStateV0(),
	}
}

// upgradeStateV0 upgrades v0 state to v1. The resource stored an unset
// description as an empty string. It's stored as null, matching a
// configuration that leaves it out.
func (r *dataCenterAzureSubscriptionResource) upgradeStateV0() resource.StateUpgrader {
	return resource.StateUpgrader{
		PriorSchema: dataCenterAzureSubscriptionSchemaV0,
		StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, res *resource.UpgradeStateResponse) {
			tflog.Trace(ctx, "dataCenterAzureSubscriptionResource.upgradeStateV0")

			var state dataCenterAzureSubscriptionResourceModel
			res.Diagnostics.Append(req.State.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			if state.Description.ValueString() == "" {
				state.Description = types.StringNull()
			}
			res.Diagnostics.Append(res.State.Set(ctx, &state)...)
		},
	}
}

// dataCenterAzureSubscriptionSchemaV0 is the schema of the resource in the v0
// state.
var dataCenterAzureSubscriptionSchemaV0 = &schema.Schema{
	Attributes: map[string]schema.Attribute{
		keyID:             schema.StringAttribute{Computed: true},
		keyDescription:    schema.StringAttribute{Optional: true},
		keyName:           schema.StringAttribute{Required: true},
		keySubscriptionID: schema.StringAttribute{Required: true},
		keyTenantID:       schema.StringAttribute{Computed: true},
	},
}
