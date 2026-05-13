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

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/access"
)

func permissionAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		keyOperation: types.StringType,
		keyHierarchy: types.SetType{ElemType: types.ObjectType{AttrTypes: hierarchyAttrTypes()}},
	}
}

func hierarchyAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		keySnappableType: types.StringType,
		keyObjectIDs:     types.SetType{ElemType: types.StringType},
	}
}

func fromPermissions(ctx context.Context, permissions []access.Permission) (types.Set, diag.Diagnostics) {
	permissionValues := make([]attr.Value, 0, len(permissions))
	for _, p := range permissions {
		hierarchyValues := make([]attr.Value, 0, len(p.ObjectsForHierarchyTypes))
		for _, h := range p.ObjectsForHierarchyTypes {
			objectValues := make([]attr.Value, 0, len(h.ObjectIDs))
			for _, id := range h.ObjectIDs {
				objectValues = append(objectValues, types.StringValue(id))
			}

			objectSet, diags := types.SetValue(types.StringType, objectValues)
			if diags.HasError() {
				return types.SetNull(types.ObjectType{AttrTypes: permissionAttrTypes()}), diags
			}

			hierarchyValue, diags := types.ObjectValue(hierarchyAttrTypes(), map[string]attr.Value{
				keySnappableType: types.StringValue(h.SnappableType),
				keyObjectIDs:     objectSet,
			})
			if diags.HasError() {
				return types.SetNull(types.ObjectType{AttrTypes: permissionAttrTypes()}), diags
			}

			hierarchyValues = append(hierarchyValues, hierarchyValue)
		}

		hierarchySet, diags := types.SetValue(types.ObjectType{AttrTypes: hierarchyAttrTypes()}, hierarchyValues)
		if diags.HasError() {
			return types.SetNull(types.ObjectType{AttrTypes: permissionAttrTypes()}), diags
		}

		permissionValue, diags := types.ObjectValue(permissionAttrTypes(), map[string]attr.Value{
			keyOperation: types.StringValue(p.Operation),
			keyHierarchy: hierarchySet,
		})
		if diags.HasError() {
			return types.SetNull(types.ObjectType{AttrTypes: permissionAttrTypes()}), diags
		}

		permissionValues = append(permissionValues, permissionValue)
	}

	return types.SetValue(types.ObjectType{AttrTypes: permissionAttrTypes()}, permissionValues)
}

func toPermissions(ctx context.Context, permissionSet types.Set) ([]access.Permission, diag.Diagnostics) {
	var permissionModels []struct {
		Operation types.String `tfsdk:"operation"`
		Hierarchy types.Set    `tfsdk:"hierarchy"`
	}
	diags := permissionSet.ElementsAs(ctx, &permissionModels, false)
	if diags.HasError() {
		return nil, diags
	}

	permissions := make([]access.Permission, 0, len(permissionModels))
	for _, pm := range permissionModels {
		var hierarchyModels []struct {
			SnappableType types.String `tfsdk:"snappable_type"`
			ObjectIDs     types.Set    `tfsdk:"object_ids"`
		}
		diags.Append(pm.Hierarchy.ElementsAs(ctx, &hierarchyModels, false)...)
		if diags.HasError() {
			return nil, diags
		}

		hierarchies := make([]access.ObjectsForHierarchyType, 0, len(hierarchyModels))
		for _, hm := range hierarchyModels {
			var objectIDs []string
			diags.Append(hm.ObjectIDs.ElementsAs(ctx, &objectIDs, false)...)
			if diags.HasError() {
				return nil, diags
			}

			hierarchies = append(hierarchies, access.ObjectsForHierarchyType{
				SnappableType: hm.SnappableType.ValueString(),
				ObjectIDs:     objectIDs,
			})
		}

		permissions = append(permissions, access.Permission{
			Operation:                pm.Operation.ValueString(),
			ObjectsForHierarchyTypes: hierarchies,
		})
	}

	return permissions, diags
}
