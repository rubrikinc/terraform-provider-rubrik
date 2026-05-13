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
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/aws"
	"github.com/rubrikinc/rubrik-polaris-sdk-for-go/pkg/polaris/graphql/core"
)

func awsFeatureAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		keyName:             types.StringType,
		keyPermissionGroups: types.SetType{ElemType: types.StringType},
	}
}

func awsFromFeatures(ctx context.Context, features []aws.Feature) (types.Set, diag.Diagnostics) {
	featureValues := make([]attr.Value, 0, len(features))
	for _, feature := range features {
		groupValues := make([]attr.Value, 0, len(feature.PermissionGroups))
		for _, group := range feature.PermissionGroups {
			groupValues = append(groupValues, types.StringValue(string(group)))
		}

		groupSet, d := types.SetValue(types.StringType, groupValues)
		if d.HasError() {
			return types.SetNull(types.ObjectType{AttrTypes: awsFeatureAttrTypes()}), d
		}

		object, d := types.ObjectValue(awsFeatureAttrTypes(), map[string]attr.Value{
			keyName:             types.StringValue(feature.Name),
			keyPermissionGroups: groupSet,
		})
		if d.HasError() {
			return types.SetNull(types.ObjectType{AttrTypes: awsFeatureAttrTypes()}), d
		}
		featureValues = append(featureValues, object)
	}

	return types.SetValue(types.ObjectType{AttrTypes: awsFeatureAttrTypes()}, featureValues)
}

func awsToFeatures(ctx context.Context, featureSet types.Set) ([]core.Feature, diag.Diagnostics) {
	var featureModels []struct {
		Name             types.String `tfsdk:"name"`
		PermissionGroups types.Set    `tfsdk:"permission_groups"`
	}
	diags := featureSet.ElementsAs(ctx, &featureModels, false)
	if diags.HasError() {
		return nil, diags
	}

	features := make([]core.Feature, 0, len(featureModels))
	for _, model := range featureModels {
		var groups []string
		diags.Append(model.PermissionGroups.ElementsAs(ctx, &groups, false)...)
		if diags.HasError() {
			return nil, diags
		}

		feature := core.Feature{Name: model.Name.ValueString()}
		for _, g := range groups {
			feature = feature.WithPermissionGroups(core.PermissionGroup(g))
		}
		features = append(features, feature)
	}

	return features, diags
}
