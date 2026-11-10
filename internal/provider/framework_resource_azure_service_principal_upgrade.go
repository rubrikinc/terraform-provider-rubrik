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
	"encoding/json"
	"errors"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type azureServicePrincipalResourceModelV0 struct {
	ID           types.String `tfsdk:"id"`
	AppID        types.String `tfsdk:"app_id"`
	AppName      types.String `tfsdk:"app_name"`
	AppSecret    types.String `tfsdk:"app_secret"`
	Credentials  types.String `tfsdk:"credentials"`
	TenantDomain types.String `tfsdk:"tenant_domain"`
	TenantID     types.String `tfsdk:"tenant_id"`
}

func (r *azureServicePrincipalResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	tflog.Trace(ctx, "azureServicePrincipalResource.UpgradeState")

	return map[int64]resource.StateUpgrader{
		0: r.upgradeStateV0(),
	}
}

// upgradeStateV0 upgrades v0 state, where the tenant domain was optional, to
// v1 where the tenant domain is required. The tenant domain is only missing
// when the service principal has been given as a credentials file, in which
// case it is read from the file.
func (r *azureServicePrincipalResource) upgradeStateV0() resource.StateUpgrader {
	return resource.StateUpgrader{
		PriorSchema: &schema.Schema{
			Attributes: map[string]schema.Attribute{
				keyID: schema.StringAttribute{
					Computed: true,
				},
				keyAppID: schema.StringAttribute{
					Optional: true,
				},
				keyAppName: schema.StringAttribute{
					Optional: true,
				},
				keyAppSecret: schema.StringAttribute{
					Optional:  true,
					Sensitive: true,
				},
				keyCredentials: schema.StringAttribute{
					Optional: true,
				},
				keyTenantDomain: schema.StringAttribute{
					Optional: true,
				},
				keyTenantID: schema.StringAttribute{
					Optional: true,
				},
			},
		},
		StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, res *resource.UpgradeStateResponse) {
			tflog.Trace(ctx, "azureServicePrincipalResource.upgradeStateV0")

			var prior azureServicePrincipalResourceModelV0
			res.Diagnostics.Append(req.State.Get(ctx, &prior)...)
			if res.Diagnostics.HasError() {
				return
			}

			state, err := upgradeAzureServicePrincipalStateV0(prior)
			if err != nil {
				res.Diagnostics.AddError("State upgrade failed", err.Error())
				return
			}
			res.Diagnostics.Append(res.State.Set(ctx, &state)...)
		},
	}
}

// upgradeAzureServicePrincipalStateV0 upgrades v0 state to v1. The tenant
// domain is read from the credentials file when the service principal has
// been given as a credentials file. Used by both the state upgrader and the
// v0 state mover.
func upgradeAzureServicePrincipalStateV0(prior azureServicePrincipalResourceModelV0) (azureServicePrincipalResourceModel, error) {
	if credentials := prior.Credentials.ValueString(); credentials != "" {
		buf, err := os.ReadFile(credentials)
		if err != nil {
			return azureServicePrincipalResourceModel{}, err
		}

		var tenantDomain struct {
			V0 string `json:"tenant_domain"`
			V1 string `json:"tenantDomain"`
		}
		if err := json.Unmarshal(buf, &tenantDomain); err != nil {
			return azureServicePrincipalResourceModel{}, err
		}

		switch {
		case tenantDomain.V0 != "":
			prior.TenantDomain = types.StringValue(tenantDomain.V0)
		case tenantDomain.V1 != "":
			prior.TenantDomain = types.StringValue(tenantDomain.V1)
		default:
			return azureServicePrincipalResourceModel{},
				errors.New("credentials file does not contain tenant domain")
		}
	}

	return azureServicePrincipalResourceModel{
		ID:              prior.ID,
		AppID:           prior.AppID,
		AppName:         prior.AppName,
		AppSecret:       prior.AppSecret,
		Credentials:     prior.Credentials,
		SDKAuth:         types.StringNull(),
		Permissions:     types.StringNull(),
		PermissionsHash: types.StringNull(),
		TenantDomain:    prior.TenantDomain,
		TenantID:        prior.TenantID,
		UseCase:         types.StringValue(useCaseCloudNativeProtection),
	}, nil
}
