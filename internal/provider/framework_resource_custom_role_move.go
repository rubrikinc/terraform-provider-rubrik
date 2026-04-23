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

func (r *customRoleResource) MoveState(ctx context.Context) []resource.StateMover {
	tflog.Trace(ctx, "customRoleResource.MoveState")

	return []resource.StateMover{
		r.moveStateV0(),
	}
}

// moveStateV0 moves v0 state from the polaris_custom_role resource in the
// rubrikinc/polaris provider to the rubrik_custom_role resource.
func (r *customRoleResource) moveStateV0() resource.StateMover {
	return resource.StateMover{
		SourceSchema: &schema.Schema{
			Attributes: map[string]schema.Attribute{
				keyID: schema.StringAttribute{
					Computed:    true,
					Description: "Role ID (UUID).",
				},
				keyName: schema.StringAttribute{
					Required:    true,
					Description: "Role name.",
				},
				keyDescription: schema.StringAttribute{
					Optional:    true,
					Description: "Role description.",
				},
			},
			Blocks: map[string]schema.Block{
				keyPermission: schema.SetNestedBlock{
					Description: "Role permission.",
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							keyOperation: schema.StringAttribute{
								Required:    true,
								Description: "Operation to allow on object IDs under the snappable hierarchy.",
							},
						},
						Blocks: map[string]schema.Block{
							keyHierarchy: schema.SetNestedBlock{
								Description: "Snappable hierarchy.",
								NestedObject: schema.NestedBlockObject{
									Attributes: map[string]schema.Attribute{
										keySnappableType: schema.StringAttribute{
											Required:    true,
											Description: "Snappable/workload type.",
										},
										keyObjectIDs: schema.SetAttribute{
											ElementType: types.StringType,
											Required:    true,
											Description: "Object/workload identifiers.",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		StateMover: func(ctx context.Context, req resource.MoveStateRequest, res *resource.MoveStateResponse) {
			tflog.Trace(ctx, "customRoleResource.moveStateV0")

			if !strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/polaris") &&
				!strings.HasSuffix(req.SourceProviderAddress, "rubrikinc/rubrik") {
				return
			}
			if req.SourceTypeName != keyPolaris+"_"+keyCustomRole {
				return
			}
			if req.SourceSchemaVersion != 0 {
				return
			}

			var state customRoleModel
			res.Diagnostics.Append(req.SourceState.Get(ctx, &state)...)
			if res.Diagnostics.HasError() {
				return
			}

			res.Diagnostics.Append(res.TargetState.Set(ctx, &state)...)
		},
	}
}
