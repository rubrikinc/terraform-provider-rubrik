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

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/gcp"
)

func (r *gcpServiceAccountResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	tflog.Trace(ctx, "gcpServiceAccountResource.UpgradeState")

	return map[int64]resource.StateUpgrader{
		0: r.upgradeStateV0(),
	}
}

// upgradeStateV0 upgrades v0 state, where the resource ID was the service
// account name, to v1, where the resource ID is the SHA-256 sum of the service
// account name in RSC. The PriorSchema is the SDKv2 v0 schema.
func (r *gcpServiceAccountResource) upgradeStateV0() resource.StateUpgrader {
	return resource.StateUpgrader{
		PriorSchema: &schema.Schema{
			Attributes: map[string]schema.Attribute{
				keyID: schema.StringAttribute{
					Computed: true,
				},
				keyCredentials: schema.StringAttribute{
					Required:    true,
					Description: "Path to GCP service account key file.",
				},
				keyName: schema.StringAttribute{
					Optional: true,
					Computed: true,
					Description: "Service account name in Polaris. If not given the name of the service account " +
						"key file is used.",
				},
				keyPermissionsHash: schema.StringAttribute{
					Optional:    true,
					Description: "Signals that the permissions has been updated.",
				},
			},
		},
		StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, res *resource.UpgradeStateResponse) {
			tflog.Trace(ctx, "gcpServiceAccountResource.upgradeStateV0")

			var state gcpServiceAccountModel
			res.Diagnostics.Append(req.State.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			polarisClient, err := r.client.polaris()
			if err != nil {
				res.Diagnostics.AddError("RSC client error", err.Error())
				return
			}

			name, err := gcp.Wrap(polarisClient).ServiceAccount(ctx)
			if err != nil {
				res.Diagnostics.AddError("Failed to read GCP default service account", err.Error())
				return
			}

			state.ID = types.StringValue(fmt.Sprintf("%x", sha256.Sum256([]byte(name))))
			res.Diagnostics.Append(res.State.Set(ctx, &state)...)
		},
	}
}
