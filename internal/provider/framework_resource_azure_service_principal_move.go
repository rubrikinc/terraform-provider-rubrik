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
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func (r *azureServicePrincipalResource) MoveState(ctx context.Context) []resource.StateMover {
	tflog.Trace(ctx, "azureServicePrincipalResource.MoveState")

	return []resource.StateMover{
		r.moveStateV0(),
		r.moveStateV1(),
	}
}

// moveStateV0 moves v0 state from the polaris_azure_service_principal resource
// in the rubrikinc/polaris provider to the rubrik_azure_service_principal
// resource. The v0 state has an optional tenant domain, which is read from the
// credentials file when the service principal has been given as a credentials
// file.
func (r *azureServicePrincipalResource) moveStateV0() resource.StateMover {
	return resource.StateMover{
		SourceSchema: &schema.Schema{
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
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "azureServicePrincipalResource.moveStateV0")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolaris+"_"+keyAzureServicePrincipal {
				return
			}
			if req.SourceSchemaVersion != 0 {
				return
			}

			var prior azureServicePrincipalResourceModelV0
			res.Diagnostics.Append(req.SourceState.Get(ctx, &prior)...)
			if res.Diagnostics.HasError() {
				return
			}

			state, err := upgradeAzureServicePrincipalStateV0(prior)
			if err != nil {
				res.Diagnostics.AddError("State move failed", err.Error())
				return
			}
			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}

// moveStateV1 moves v1 state from the polaris_azure_service_principal resource
// in the rubrikinc/polaris provider, or from the deprecated alias in this
// provider, to the rubrik_azure_service_principal resource.
func (r *azureServicePrincipalResource) moveStateV1() resource.StateMover {
	return resource.StateMover{
		SourceSchema: &schema.Schema{
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
				keySDKAuth: schema.StringAttribute{
					Optional: true,
				},
				keyPermissions: schema.StringAttribute{
					Optional: true,
				},
				keyPermissionsHash: schema.StringAttribute{
					Optional: true,
				},
				keyTenantDomain: schema.StringAttribute{
					Required: true,
				},
				keyTenantID: schema.StringAttribute{
					Optional: true,
				},
				keyUseCase: schema.StringAttribute{
					Optional: true,
				},
			},
		},
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "azureServicePrincipalResource.moveStateV1")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolaris+"_"+keyAzureServicePrincipal {
				return
			}
			if req.SourceSchemaVersion != 1 {
				return
			}

			var state azureServicePrincipalResourceModel
			res.Diagnostics.Append(req.SourceState.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}
