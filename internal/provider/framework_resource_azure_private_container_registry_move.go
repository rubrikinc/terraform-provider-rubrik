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

func (r *azurePrivateContainerRegistryResource) MoveState(ctx context.Context) []resource.StateMover {
	tflog.Trace(ctx, "azurePrivateContainerRegistryResource.MoveState")

	return []resource.StateMover{
		r.moveStateV0(),
	}
}

// moveStateV0 moves v0 state from the polaris_azure_private_container_registry
// resource in the rubrikinc/polaris provider, or from the deprecated alias in
// this provider, to the rubrik_azure_private_container_registry resource.
func (r *azurePrivateContainerRegistryResource) moveStateV0() resource.StateMover {
	return resource.StateMover{
		SourceSchema: &schema.Schema{
			Attributes: map[string]schema.Attribute{
				keyID: schema.StringAttribute{
					Computed: true,
				},
				keyCloudAccountID: schema.StringAttribute{
					Required: true,
				},
				keyAppID: schema.StringAttribute{
					Required: true,
				},
				keyURL: schema.StringAttribute{
					Required: true,
				},
			},
		},
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "azurePrivateContainerRegistryResource.moveStateV0")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolaris+"_"+keyAzurePrivateContainerRegistry {
				return
			}
			if req.SourceSchemaVersion != 0 {
				return
			}

			var state azurePrivateContainerRegistryResourceModel
			res.Diagnostics.Append(req.SourceState.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}
